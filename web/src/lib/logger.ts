export enum LogLevel {
  DEBUG = 0,
  INFO = 1,
  WARN = 2,
  ERROR = 3,
  SILENT = 4,
}

interface LogEntry {
  level: string
  prefix: string
  message: string
  data?: Record<string, unknown>
  timestamp: string
}

export interface Logger {
  debug: (msg: string, data?: Record<string, unknown>) => void
  info: (msg: string, data?: Record<string, unknown>) => void
  warn: (msg: string, data?: Record<string, unknown>) => void
  error: (msg: string, data?: Record<string, unknown> | Error) => void
  level: LogLevel
  setLevel: (level: LogLevel) => void
}

const LEVEL_NAMES: Record<LogLevel, string> = {
  [LogLevel.DEBUG]: 'DEBUG',
  [LogLevel.INFO]: 'INFO',
  [LogLevel.WARN]: 'WARN',
  [LogLevel.ERROR]: 'ERROR',
  [LogLevel.SILENT]: 'SILENT',
}

function formatEntry(entry: LogEntry): string {
  const base = `[${entry.timestamp}] [${entry.prefix}] [${entry.level}] ${entry.message}`
  if (entry.data) {
    return `${base} ${JSON.stringify(entry.data)}`
  }
  return base
}

export function createLogger(prefix: string, level = LogLevel.INFO): Logger {
  let currentLevel = level

  function log(lvl: LogLevel, msg: string, data?: Record<string, unknown> | Error): void {
    if (lvl < currentLevel) return

    const entry: LogEntry = {
      level: LEVEL_NAMES[lvl],
      prefix,
      message: msg,
      timestamp: new Date().toISOString(),
    }

    if (data instanceof Error) {
      entry.data = { name: data.name, message: data.message, stack: data.stack }
    } else if (data) {
      entry.data = data
    }

    const formatted = formatEntry(entry)
    switch (lvl) {
      case LogLevel.DEBUG:
      case LogLevel.INFO:
        console.log(formatted)
        break
      case LogLevel.WARN:
        console.warn(formatted)
        break
      case LogLevel.ERROR:
        console.error(formatted)
        break
    }
  }

  return {
    debug: (msg, data) => log(LogLevel.DEBUG, msg, data),
    info: (msg, data) => log(LogLevel.INFO, msg, data),
    warn: (msg, data) => log(LogLevel.WARN, msg, data),
    error: (msg, data) => log(LogLevel.ERROR, msg, data),
    get level() { return currentLevel },
    setLevel: (l) => { currentLevel = l },
  }
}

// Default app logger
const isDev = typeof importMeta !== 'undefined' && (importMeta as any).env?.DEV
export const logger = createLogger('ZoteroWeb', isDev ? LogLevel.DEBUG : LogLevel.INFO)

declare const importMeta: { env?: { DEV?: boolean } } | undefined

// API request/response logging helper
export function logApiRequest(method: string, url: string, options?: RequestInit): void {
  logger.debug('API request', { method, url, bodySize: options?.body ? JSON.stringify(options.body).length : 0 })
}

export function logApiResponse(method: string, url: string, status: number, durationMs: number): void {
  const lvl = status >= 400 ? LogLevel.WARN : LogLevel.DEBUG
  const prevLevel = logger.level
  logger.setLevel(lvl)
  logger.info('API response', { method, url, status, durationMs: `${durationMs}ms` })
  logger.setLevel(prevLevel)
}
