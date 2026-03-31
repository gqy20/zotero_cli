# Rate Limit Optimization

## Problem Statement

Zotero API has opaque rate limits. When limits are exceeded, requests fail with `429 Too Many Requests` and a `Retry-After` header indicating how long to wait.

Current implementation issues:

1. **Retry delay is insufficient**: default base delay is 250ms, resulting in total ~1.75s across 3 retries. This is far short of Zotero's actual reset window.
2. **`Retry-After` parsing is incomplete**: `parseRetryAfter` only handles integer seconds format (`"60"`), but Zotero may return HTTP-date format (`"Wed, 21 Oct 2015 07:28:00 GMT"`). On parse failure, delay falls back to 0 and the short exponential backoff is used.
3. **No client-side rate limiting**: the client fires requests as fast as possible, making it easy to trigger 429s especially in AI agent loops or batch operations.
4. **No response caching**: repeated queries for the same data (e.g., `collections`, static metadata) each hit the API independently.
5. **No ETag/conditional request support**: reads always transfer full response bodies even when data hasn't changed.

This document defines a layered optimization strategy to address these issues.

---

## Layer 0: Fix Retry-After Parsing

### Problem

```go
// current implementation — fails on HTTP-date format
func parseRetryAfter(value string) time.Duration {
    seconds, err := strconv.Atoi(value)  // "60" works, "Wed, 21 Oct 2015 07:28:00 GMT" does not
    if err != nil || seconds <= 0 {
        return 0  // falls back to short exponential backoff
    }
    return time.Duration(seconds) * time.Second
}
```

### Solution

Support both integer-seconds and HTTP-date formats:

```go
func parseRetryAfter(value string) time.Duration {
    if value == "" {
        return 0
    }

    // Try integer seconds first
    if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
        return time.Duration(seconds) * time.Second
    }

    // Fall back to HTTP-date parsing
    if t, err := time.Parse(time.RFC1123, value); err == nil {
        dur := t.Sub(time.Now())
        if dur > 0 {
            return dur
        }
    }

    return 0
}
```

### Implementation

- File: `internal/zoteroapi/retry.go` (new file)
- Move `parseRetryAfter` and `shouldRetryStatus` there
- Add `shouldRetryError` for network timeout detection
- Add tests for both `parseRetryAfter` formats

---

## Layer 1: Add Jitter to Retries

### Problem

When multiple clients retry simultaneously after a 429, they all wait the same delay and then fire at the same instant — creating a "thundering herd" that immediately triggers another 429.

### Solution

Add ±30% random jitter to each retry delay:

```go
func (c *Client) pauseBeforeRetry(attempt int, retryAfter time.Duration) {
    delay := retryAfter
    if delay <= 0 {
        baseDelay := time.Duration(c.cfg.RetryBaseDelayMilliseconds) * time.Millisecond
        if baseDelay <= 0 {
            baseDelay = 250 * time.Millisecond
        }
        delay = baseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
    }

    // Add jitter: ±30%
    jitter := delay / 10 * 3  // 30% of delay
    randDelay := delay - jitter + time.Duration(rand.Int63n(int64(2*jitter)))

    if c.sleep != nil {
        c.sleep(randDelay)
    }
}
```

### Config

Expose jitter as a fraction in config:

```json
{
  "retry_jitter_fraction": 0.3
}
```

Or as an environment variable: `ZOT_RETRY_JITTER_FRACTION=0.3`

---

## Layer 2: Client-Side Token Bucket Rate Limiter

### Problem

Without client-side rate limiting, a script or AI agent that fires many rapid requests (e.g., batch processing, monitor loops) can easily trigger 429s.

### Solution

Implement a Token Bucket rate limiter at the HTTP client level:

```go
type RateLimiter struct {
    mu           sync.Mutex
    capacity     int           // max tokens in bucket
    refillRate   float64       // tokens added per second
    tokens      float64
    lastRefill  time.Time
}

func (rl *RateLimiter) Allow() bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    rl.refill()
    if rl.tokens >= 1 {
        rl.tokens--
        return true
    }
    return false
}

func (rl *RateLimiter) Wait(ctx context.Context) error {
    for {
        if rl.Allow() {
            return nil
        }
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(50 * time.Millisecond):
        }
    }
}
```

### Integration

- The rate limiter wraps `httpClient.Do()` inside `doHTTPRequest`
- Config via `ZOT_REQUESTS_PER_SECOND` (default: 10)
- Applies to all requests, but write operations (`POST`, `PUT`, `PATCH`, `DELETE`) can be configured to bypass rate limiting since they are infrequent

### Config

```json
{
  "requests_per_second": 10
}
```

Or env: `ZOT_REQUESTS_PER_SECOND=10`

### Interaction with Retry

Rate limiting and retry are complementary:
- Rate limiter prevents triggering 429s under normal usage
- Retry with proper backoff handles the cases when 429 still occurs

---

## Layer 3: In-Memory Response Cache with ETag

### Problem

Repeated API calls for the same data (collections, item types, tags) each transfer full response bodies and count against rate limits.

### Solution

Implement an HTTP cache using `ETag` and `Last-Modified` headers:

```go
type ResponseCache struct {
    mu       sync.RWMutex
    entries  map[string]CacheEntry
    ttl      time.Duration
}

type CacheEntry struct {
    ETag         string
    LastModified string
    Body         []byte
    CachedAt     time.Time
}
```

Cache rules:

| Data type | TTL | Reasoning |
|-----------|-----|-----------|
| `item-types`, `creator-fields` | 24h | Rarely changes |
| `item-fields`, `item-type-fields`, `item-type-creator-types` | 24h | Rarely changes |
| `collections`, `tags`, `searches` | 5 min | Moderate change frequency |
| `items`, `trash`, `publications` | 0 (no cache) | Real-time accuracy required |
| `deleted` | 0 (no cache) | Security-sensitive |

### Conditional Request Flow

```
1. Check cache for entry
2. If present with ETag:
   - Add If-None-Match: <ETag> header to request
   - If 304 returned → return cached body (fast, no body transfer)
   - If 200 returned → update cache
3. If no cache or not cacheable → make direct request
```

### Cache Key

Based on request method + URL (excluding pagination start):

```
GET /users/13651982/items?limit=100&itemType=journalArticle
GET /users/13651982/items?limit=100&itemType=journalArticle&start=100  ← different cache key
```

Note: paginated responses should NOT be cached as a whole because the client fetches pages on demand.

### Implementation

- New file: `internal/zoteroapi/cache.go`
- Integrate into `doHTTPRequest` as a wrapper
- Per-request cache control via a `cacheable` flag

---

## Layer 4: Batch Request Optimization

### Problem

`GetItem` fetches one item at a time. When an AI workflow needs details on 20 items, it makes 20 sequential API calls.

### Solution

Zotero API supports `itemKey=KEY1,KEY2,...` to fetch multiple items in one request (max 50).

```go
// Before: N sequential requests
for _, key := range keys {
    item, _ := client.GetItem(ctx, key)
}

// After: 1 batched request (up to 50 keys per call)
func (c *Client) GetItemsBatch(ctx context.Context, keys []string) ([]Item, error) {
    const batchSize = 50
    var allItems []Item
    for i := 0; i < len(keys); i += batchSize {
        batch := keys[i:min(i+batchSize, len(keys))]
        resp, err := c.doRequest(ctx, "items", FindOptions{},
            map[string]string{"itemKey": strings.Join(batch, ",")})
        // process response
    }
    return allItems, nil
}
```

### Scope

- Apply batch fetching to `GetItem` internally when called in a loop
- `ExportItems` already fetches by key list — verify it batches correctly
- `GetCitation` for `bib` format calls `GetCitation` in a loop — batch this

---

## Layer 5:分级请求策略与熔断器

### 分级请求

不同操作有不同可靠性需求，应该有不同的缓存/限流策略：

| 级别 | 操作 | 缓存策略 | 限流 |
|------|------|---------|------|
| 静态元数据 | `item-types`, `item-fields` | 长缓存 24h | 极低频率 |
| 配置数据 | `collections`, `tags`, `searches` | 中缓存 5min | 低频率 |
| 实时数据 | `items`, `show`, `find` | 不缓存 | 正常频率 |
| 写操作 | `create-*`, `update-*`, `delete-*` | 不缓存 | 旁路限流 |

### 熔断器

当连续 N 次请求都收到 429 时，说明当前请求速率超出 API 承受能力：

```
状态机：
  closed（正常）→ open（熔断）→ half-open（试探）
  closed: 正常请求，失败计数器=0
  open:   直接拒绝请求，等待一段冷静期
  half-open: 允许少量请求试探，如果成功则回到 closed
```

```go
type CircuitBreaker struct {
    mu            sync.Mutex
    state         State  // closed, open, halfOpen
    failureCount  int
    successCount  int
    threshold     int    // 连续失败次数阈值（建议: 5）
    resetTimeout  time.Duration  // 冷静期（建议: 60s）
    halfOpenLimit int    // half-open 时允许的试探请求数（建议: 3）
}

func (cb *CircuitBreaker) Allow() (bool, State) {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    switch cb.state {
    case closed:
        return true, closed
    case open:
        if time.Since(cb.openAt) > cb.resetTimeout {
            cb.state = halfOpen
            cb.successCount = 0
            return true, halfOpen
        }
        return false, open
    case halfOpen:
        if cb.successCount < cb.halfOpenLimit {
            cb.successCount++
            return true, halfOpen
        }
        return false, halfOpen
    }
}
```

熔断器触发后的行为：
- 读请求返回友好的错误信息，不立即重试
- Agent/自动化工具可以感知错误并实现自己的重试逻辑

---

## Config Schema Additions

```json
{
  "rate_limit": {
    "requests_per_second": 10,
    "retry_max_attempts": 5,
    "retry_base_delay_ms": 1000,
    "retry_jitter_fraction": 0.3,
    "cache_ttl_minutes": 5
  }
}
```

对应环境变量：

```bash
ZOT_REQUESTS_PER_SECOND=10
ZOT_RETRY_MAX_ATTEMPTS=5
ZOT_RETRY_BASE_DELAY_MS=1000
ZOT_RETRY_JITTER_FRACTION=0.3
ZOT_CACHE_TTL_MINUTES=5
```

---

## Implementation Order

| 优先级 | 改动 | 难度 | 收益 |
|:------:|------|:----:|:----:|
| P0 | `parseRetryAfter` 支持 HTTP-date + jitter | 低 | 中 |
| P0 | Token bucket rate limiter | 中 | 高 |
| P1 | 响应缓存 + ETag 条件请求 | 中 | 高 |
| P1 | `GetItem` 批量请求优化 | 低 | 中 |
| P2 | 分级请求策略 | 低 | 中 |
| P2 | 熔断器 | 中 | 中 |

建议实施顺序：
1. **Layer 0 + 1**（立即修复重试问题，防止连续 429）
2. **Layer 2**（防止正常使用时触发限制）
3. **Layer 3**（减少重复请求，节省 API 调用）
4. **Layer 4**（批量操作性能优化）
5. **Layer 5**（AI agent 长时间运行时的健壮性）

---

## Testing Strategy

### Unit tests

- `parseRetryAfter` — integer seconds, HTTP-date, invalid input
- `RateLimiter` — token consumption, refill behavior
- `CircuitBreaker` — state transitions
- `ResponseCache` — TTL eviction, ETag matching

### Integration tests

- Mock 429 responses with various `Retry-After` formats
- Verify retry delay exceeds `Retry-After` value
- Verify jitter does not cause retry to happen before `Retry-After`
- Verify rate limiter actually spaces requests apart
- Verify 304 responses return cached body without new transfer

---

## Open Questions

1. **Cache persistence**: should the cache survive between CLI invocations? (建议: 否，每次运行独立缓存，避免陈旧数据问题)
2. **Rate limiter global vs per-command**: should rate limiting be global across all commands in one session, or reset per invocation? (建议: 全局，跨命令协调更合理)
3. **Cache size limit**: how many entries should the cache hold before eviction? (建议: 100 entries，LRU 淘汰)
4. **User feedback on rate limiting**: should CLI warn when approaching limits, or only on 429? (建议: 仅在 429 时告知，避免噪音)
