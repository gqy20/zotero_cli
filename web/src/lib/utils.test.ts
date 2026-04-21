import { describe, it, expect } from 'vitest'
import { cn, formatDate, truncate, formatAuthors } from './utils'

describe('utils', () => {
  describe('cn', () => {
    it('merges class names', () => {
      expect(cn('foo', 'bar')).toBe('foo bar')
    })

    it('handles conditional classes', () => {
      expect(cn('base', true && 'active', false && 'hidden')).toBe('base active')
    })
  })

  describe('formatDate', () => {
    it('returns year for valid date string', () => {
      expect(formatDate('2024-03-15')).toBe('2024')
    })

    it('returns empty for empty input', () => {
      expect(formatDate('')).toBe('')
    })

    it('returns raw string for unparseable dates', () => {
      expect(formatDate('not-a-date')).toBe('not-a-date')
    })
  })

  describe('truncate', () => {
    it('truncates long strings', () => {
      expect(truncate('Hello World', 5)).toBe('Hello...')
    })

    it('returns short strings as-is', () => {
      expect(truncate('Hi', 10)).toBe('Hi')
    })

    it('handles empty strings', () => {
      expect(truncate('', 5)).toBe('')
    })
  })

  describe('formatAuthors', () => {
    it('returns single author name', () => {
      expect(formatAuthors([{ name: 'Zhang' }])).toBe('Zhang')
    })

    it('returns first author + et al. for multiple', () => {
      expect(formatAuthors([
        { name: 'Zhang' },
        { name: 'Li' },
        { name: 'Wang' },
      ])).toBe('Zhang et al.')
    })

    it('returns empty string for empty list', () => {
      expect(formatAuthors([])).toBe('')
    })
  })
})
