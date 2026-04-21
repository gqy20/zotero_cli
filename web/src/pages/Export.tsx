import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { Item } from '@/types/item'

const formats = ['BibTeX', 'RIS', 'CSL-JSON']

export default function Export() {
  const [selectedFormat, setSelectedFormat] = useState('BibTeX')
  const { data: itemsData } = useQuery({
    queryKey: ['items', 0, 100],
    queryFn: () => api.items({ start: 0, limit: 100 }),
  })

  const items = itemsData?.ok ? (itemsData.data as Item[]) : []

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-xl font-semibold">导出中心</h1>

      <div className="bg-white rounded-lg border border-gray-200 p-5 space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-2">导出格式</label>
          <div className="flex gap-2">
            {formats.map(fmt => (
              <button
                key={fmt}
                onClick={() => setSelectedFormat(fmt)}
                className={`px-4 py-2 text-sm rounded-md border transition-colors ${
                  selectedFormat === fmt
                    ? 'bg-red-600 text-white border-red-600'
                    : 'border-gray-300 text-gray-600 hover:bg-gray-50'
                }`}
              >
                {fmt}
              </button>
            ))}
          </div>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-2">选择文献 ({items.length} 条可用)</label>
          <div className="max-h-80 overflow-y-auto border border-gray-200 rounded-md divide-y divide-gray-100">
            {items.length === 0 ? (
              <div className="p-4 text-center text-sm text-gray-400">暂无文献可导出</div>
            ) : (
              items.map(item => (
                <label key={item.key} className="flex items-center gap-3 px-4 py-2.5 hover:bg-gray-50 cursor-pointer transition-colors">
                  <input type="checkbox" className="rounded" />
                  <span className="text-sm truncate flex-1">{item.title}</span>
                  <span className="text-xs text-gray-400">{item.item_type}</span>
                </label>
              ))
            )}
          </div>
        </div>

        <button className="px-4 py-2 bg-red-600 text-white text-sm rounded-md hover:bg-red-700 transition-colors disabled:opacity-40">
          导出为 {selectedFormat}
        </button>
      </div>
    </div>
  )
}
