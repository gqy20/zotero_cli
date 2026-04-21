import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createLogger, LogLevel } from './logger'

describe('Logger', () => {
  let logSpy: ReturnType<typeof vi.spyOn>
  let warnSpy: ReturnType<typeof vi.spyOn>
  let errorSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    logSpy = vi.spyOn(console, 'log').mockImplementation(() => {})
    warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
    errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    logSpy.mockRestore()
    warnSpy.mockRestore()
    errorSpy.mockRestore()
  })

  function allCalls() {
    return [...logSpy.mock.calls, ...warnSpy.mock.calls, ...errorSpy.mock.calls]
  }

  it('creates logger with default info level', () => {
    const log = createLogger('Test')
    expect(log).toBeDefined()
    expect(log.level).toBe(LogLevel.INFO)
  })

  it('respects level filtering (debug suppressed at info)', () => {
    const log = createLogger('Test', LogLevel.INFO)
    log.debug('hidden')
    log.info('visible')

    const calls = allCalls()
    const debugCalls = calls.filter(c => c[0]?.toString().includes('[DEBUG]'))
    expect(debugCalls).toHaveLength(0)

    const infoCalls = calls.filter(c => c[0]?.toString().includes('[INFO]'))
    expect(infoCalls).toHaveLength(1)
  })

  it('includes prefix in output', () => {
    const log = createLogger('MyModule')
    log.info('hello')

    const output = allCalls()[0]?.[0]?.toString() ?? ''
    expect(output).toContain('[MyModule]')
  })

  it('formats key-value pairs in output', () => {
    const log = createLogger('API')
    log.info('request', { url: '/items', method: 'GET' })

    const output = allCalls()[0]?.[0]?.toString() ?? ''
    expect(output).toContain('/items')
    expect(output).toContain('GET')
  })

  it('outputs all levels when set to DEBUG', () => {
    const log = createLogger('X', LogLevel.DEBUG)
    log.debug('d')
    log.info('i')
    log.warn('w')
    log.error('e')

    const calls = allCalls()
    expect(calls.length).toBe(4)

    const levels = calls.map(c => c[0]?.toString())
    expect(levels.some(l => l?.includes('[DEBUG]'))).toBe(true)
    expect(levels.some(l => l?.includes('[INFO]'))).toBe(true)
    expect(levels.some(l => l?.includes('[WARN]'))).toBe(true)
    expect(levels.some(l => l?.includes('[ERROR]'))).toBe(true)
  })
})
