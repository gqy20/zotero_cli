import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatDate(dateStr: string): string {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return dateStr
  return d.getFullYear().toString()
}

export function truncate(str: string, len: number): string {
  if (!str || str.length <= len) return str
  return str.slice(0, len) + '...'
}

export function formatAuthors(creators: { name: string }[]): string {
  if (!creators.length) return ''
  if (creators.length === 1) return creators[0].name
  return `${creators[0].name} et al.`
}
