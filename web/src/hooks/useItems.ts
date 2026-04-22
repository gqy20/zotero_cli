import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { Item } from '@/types/item'

interface UseItemsParams {
  start?: number
  limit?: number
  q?: string
  enabled?: boolean | undefined
}

interface UseItemsReturn {
  items: Item[]
  isLoading: boolean
  total?: number
}

export default function useItems(params?: UseItemsParams, enabled?: boolean): UseItemsReturn {
  const { data, isLoading } = useQuery({
    queryKey: ['items', params],
    queryFn: () => api.items(params as Record<string, string | number | boolean>),
    enabled: enabled ?? params?.enabled ?? true,
  })

  const items = data?.ok ? (data.data as Item[]) : []

  return {
    items,
    isLoading,
    total: data?.meta?.total,
  }
}
