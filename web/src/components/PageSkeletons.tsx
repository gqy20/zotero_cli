import { Skeleton } from './ui/skeleton'

export function DashboardSkeleton() {
  return (
    <div className="p-8 space-y-8">
      <div className="flex items-end justify-between">
        <div className="space-y-2">
          <Skeleton className="h-7 w-32" />
          <Skeleton className="h-4 w-48" />
        </div>
        <Skeleton className="h-5 w-24" />
      </div>
      <div className="grid grid-cols-4 gap-5">
        {[0, 1, 2, 3].map(i => (
          <div key={i} className="bg-white rounded-2xl border border-gray-100 p-5">
            <div className="flex items-center justify-between">
              <div className="space-y-2 flex-1">
                <Skeleton className="h-3 w-16" />
                <Skeleton className="h-8 w-20" />
              </div>
              <Skeleton className="w-12 h-12 rounded-xl" />
            </div>
          </div>
        ))}
      </div>
      <div className="bg-white rounded-2xl border border-gray-100 overflow-hidden shadow-sm">
        <div className="px-6 py-4 border-b border-gray-100">
          <Skeleton className="h-4 w-24" />
        </div>
        <div className="p-6 space-y-4">
          {[0, 1, 2].map(i => (
            <div key={i} className="flex items-center justify-between py-2">
              <div className="flex-1 space-y-2 mr-4">
                <Skeleton className="h-4 w-[60%]" />
                <Skeleton className="h-3 w-[40%]" />
              </div>
              <Skeleton className="h-4 w-12" />
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

export function LibrarySkeleton() {
  return (
    <div className="flex h-full">
      <aside className="w-56 border-r border-gray-200/80 bg-white">
        <div className="px-4 py-4 border-b border-gray-100">
          <Skeleton className="h-3.5 w-20" />
        </div>
        <div className="py-2 px-2 space-y-1">
          <Skeleton className="h-9 rounded-xl" />
          <Skeleton className="h-9 rounded-xl" />
          <Skeleton className="h-9 rounded-xl" />
          <Skeleton className="h-9 rounded-xl" />
        </div>
      </aside>
      <div className="flex-1 flex flex-col">
        <div className="px-8 py-5 border-b border-gray-200/80 bg-white/60">
          <div className="flex items-center justify-between">
            <Skeleton className="h-10 flex-1 max-w-md" />
            <Skeleton className="h-4 w-16" />
          </div>
        </div>
        <div className="flex-1 overflow-auto p-8 space-y-3">
          <Skeleton className="h-4 w-full max-w-[600px]" />
          {[0, 1, 2, 3, 4].map(i => (
            <div key={i} className="flex items-center gap-4 py-3">
              <Skeleton className="h-4 w-[45%]" />
              <Skeleton className="h-3 w-[25%]" />
              <Skeleton className="h-3 w-[20%]" />
              <Skeleton className="h-3 w-12" />
            </div>
          ))}
        </div>
        <div className="px-8 py-4 border-t border-gray-100 flex items-center justify-between">
          <Skeleton className="h-8 w-20" />
          <Skeleton className="h-8 w-16" />
          <Skeleton className="h-8 w-20" />
        </div>
      </div>
    </div>
  )
}

export function ItemDetailSkeleton() {
  return (
    <div className="p-8 space-y-8">
      <Skeleton className="h-5 w-40" />
      <div className="space-y-3">
        <Skeleton className="h-8 w-[70%]" />
        <Skeleton className="h-6 w-[40%] inline-block" />
      </div>
      <div className="bg-white rounded-2xl border border-gray-100 p-6 shadow-sm">
        <div className="grid grid-cols-2 gap-x-10 gap-y-4">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="flex items-start gap-3">
              <Skeleton className="w-4 h-4 rounded" />
              <div className="flex-1 space-y-1.5">
                <Skeleton className="h-3 w-16" />
                <Skeleton className="h-4 w-[80%]" />
              </div>
            </div>
          ))}
        </div>
      </div>
      <div className="bg-white rounded-2xl border border-gray-100 p-6 shadow-sm">
        <Skeleton className="h-4 w-20 mb-4" />
        <div className="space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="flex items-center gap-3">
              <Skeleton className="w-8 h-8 rounded-lg" />
              <Skeleton className="h-4 flex-1" />
              <Skeleton className="h-7 w-20" />
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

export function SearchSkeleton() {
  return (
    <div className="p-8 space-y-8 max-w-4xl">
      <div className="space-y-2">
        <Skeleton className="h-7 w-48" />
        <Skeleton className="h-4 w-64" />
      </div>
      <Skeleton className="h-14 w-full rounded-2xl" />
      <div className="space-y-4 pt-4">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="p-5 bg-white rounded-2xl border border-gray-100">
            <div className="flex items-start justify-between gap-4">
              <div className="flex-1 space-y-2">
                <Skeleton className="h-5 w-[65%]" />
                <div className="flex gap-2">
                  <Skeleton className="h-3 w-24" />
                  <Skeleton className="h-3 w-16" />
                  <Skeleton className="h-4 w-20" />
                </div>
                <Skeleton className="h-3.5 w-[70%] mt-2" />
              </div>
              <Skeleton className="w-4 h-4" />
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

export function TagsSkeleton() {
  return (
    <div className="p-8 space-y-8">
      <div className="flex items-end justify-between">
        <div className="space-y-2">
          <Skeleton className="h-7 w-36" />
          <Skeleton className="h-4 w-48" />
        </div>
        <Skeleton className="h-6 w-24" />
      </div>
      <div className="grid grid-cols-3 gap-4">
        {[0, 1, 2].map(i => (
          <div key={i} className="bg-white rounded-2xl border border-gray-100 p-5">
            <div className="flex items-center gap-3 mb-3">
              <Skeleton className="w-10 h-10 rounded-xl" />
              <Skeleton className="h-3 w-8" />
            </div>
            <Skeleton className="h-5 w-28" />
            <Skeleton className="h-8 w-16" />
            <Skeleton className="h-3 w-14" />
          </div>
        ))}
      </div>
      <div className="bg-white rounded-2xl border border-gray-100 overflow-hidden shadow-sm">
        <div className="px-6 py-4 border-b border-gray-100">
          <Skeleton className="h-4 w-20" />
        </div>
        <div className="divide-y divide-gray-50">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="px-6 py-3.5 flex items-center gap-4">
              <Skeleton className="h-6 w-20 rounded-lg" />
              <Skeleton className="flex-1 h-1.5 rounded-full" />
              <Skeleton className="h-4 w-10" />
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

export function ExportSkeleton() {
  return (
    <div className="p-8 space-y-8 max-w-4xl">
      <div className="space-y-2">
        <Skeleton className="h-7 w-40" />
        <Skeleton className="h-4 w-56" />
      </div>
      <div>
        <Skeleton className="h-4 w-28 mb-3" />
        <div className="grid grid-cols-3 gap-3">
          {[0, 1, 2].map(i => (
            <div key={i} className="p-4 rounded-2xl border-2 border-gray-100">
              <div className="flex items-center gap-3 mb-2">
                <Skeleton className="w-10 h-10 rounded-xl" />
                <div className="flex-1 space-y-1">
                  <Skeleton className="h-4 w-16" />
                  <Skeleton className="h-3 w-24" />
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
      <div>
        <div className="flex items-center justify-between mb-3">
          <Skeleton className="h-4 w-36" />
          <Skeleton className="h-4 w-12" />
        </div>
        <div className="bg-white rounded-2xl border border-gray-100 overflow-hidden">
          <div className="max-h-[420px] space-y-px p-3">
            {Array.from({ length: 6 }).map((_, i) => (
              <div key={i} className="flex items-center gap-3 px-3 py-2.5">
                <Skeleton className="w-4 h-4 rounded" />
                <Skeleton className="h-4 flex-1" />
                <Skeleton className="h-5 w-20" />
              </div>
            ))}
          </div>
        </div>
      </div>
      <div className="flex items-center justify-between pt-2">
        <Skeleton className="h-4 w-32" />
        <Skeleton className="h-10 w-36" />
      </div>
    </div>
  )
}
