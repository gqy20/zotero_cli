import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { Collection } from '@/types/item'

interface UseCollectionsReturn {
  collections: Collection[]
  isLoading: boolean
}

export default function useCollections(): UseCollectionsReturn {
  const { data, isLoading } = useQuery({
    queryKey: ['collections'],
    queryFn: () => api.collections(),
  })

  const collections = data?.ok ? (data.data as Collection[]) : []

  return { collections, isLoading }
}
