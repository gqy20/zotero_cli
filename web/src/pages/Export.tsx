import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { Item } from '@/types/item'
import { Download, FileCode, FileText, Braces, Check, ChevronDown } from 'lucide-react'

const formats = [
  { name: 'BibTeX', icon: FileCode, desc: 'LaTeX 引用格式', color: 'from-red-500 to-rose-600' },
  { name: 'RIS', icon: FileText, desc: 'EndNote / Reference Manager', color: 'from-blue-500 to-indigo-600' },
  { name: 'CSL-JSON', icon: Braces, desc: 'Citation Style Language', color: 'from-emerald-500 to-teal-600' },
]

export default function Export() {
  const [selectedFormat, setSelectedFormat] = useState('BibTeX')
  const [selectedItems, setSelectedItems] = useState<Set<string>>(new Set())
  const { data: itemsData } = useQuery({
    queryKey: ['items', 0, 100],
    queryFn: () => api.items({ start: 0, limit: 100 }),
  })

  const items = itemsData?.ok ? (itemsData.data as Item[]) : []

  const toggleAll = () => {
    if (selectedItems.size === items.length) {
      setSelectedItems(new Set())
    } else {
      setSelectedItems(new Set(items.map(i => i.key)))
    }
  }

  const toggleItem = (key: string) => {
    const next = new Set(selectedItems)
    if (next.has(key)) next.delete(key)
    else next.add(key)
    setSelectedItems(next)
  }

  return (
    <div className="p-8 space-y-8 max-w-4xl">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-gray-900 tracking-tight">导出中心</h1>
        <p className="text-sm text-gray-400 mt-1">选择文献并导出为多种引用格式</p>
      </div>

      {/* Format selection */}
      <div>
        <label className="block text-xs font-semibold text-gray-400 uppercase tracking-wider mb-3">导出格式</label>
        <div className="grid grid-cols-3 gap-3">
          {formats.map(fmt => (
            <button
              key={fmt.name}
              onClick={() => setSelectedFormat(fmt.name)}
              className={`relative p-4 rounded-2xl border-2 text-left transition-all duration-200 ${
                selectedFormat === fmt.name
                  ? 'border-transparent shadow-lg scale-[1.02]'
                  : 'border-gray-100 hover:border-gray-200 hover:shadow-sm'
              }`}
            >
              {selectedFormat === fmt.name && (
                <div className={`absolute inset-0 bg-gradient-to-br ${fmt.color} opacity-[0.04] rounded-2xl`} />
              )}
              <div className="flex items-center gap-3 relative">
                <div className={`w-10 h-10 rounded-xl flex items-center justify-center ${
                  selectedFormat === fmt.name
                    ? `bg-gradient-to-br ${fmt.color} shadow-md`
                    : 'bg-gray-50'
                }`}>
                  <fmt.icon className={`w-5 h-5 ${selectedFormat === fmt.name ? 'text-white' : 'text-gray-400'}`} />
                </div>
                <div>
                  <p className={`font-semibold text-sm ${selectedFormat === fmt.name ? 'text-gray-900' : 'text-gray-600'}`}>{fmt.name}</p>
                  <p className="text-[10px] text-gray-400 mt-0.5">{fmt.desc}</p>
                </div>
                {selectedFormat === fmt.name && (
                  <Check className="w-4 h-4 ml-auto text-green-500" />
                )}
              </div>
            </button>
          ))}
        </div>
      </div>

      {/* Item selection */}
      <div>
        <div className="flex items-center justify-between mb-3">
          <label className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
            选择文献
            <span className="ml-2 font-normal normal-case text-gray-300">({items.length} 条可用)</span>
          </label>
          <button
            onClick={toggleAll}
            className="text-xs text-red-500 hover:text-red-600 font-medium transition-colors"
          >
            {selectedItems.size === items.length ? '取消全选' : '全选'}
          </button>
        </div>

        <div className="bg-white rounded-2xl border border-gray-100 overflow-hidden shadow-sm">
          <div className="max-h-[420px] overflow-y-auto divide-y divide-gray-50">
            {items.length === 0 ? (
              <div className="p-8 text-center">
                <Download className="w-8 h-8 text-gray-200 mx-auto mb-2" />
                <p className="text-sm text-gray-400">暂无文献可导出</p>
              </div>
            ) : (
              items.map(item => (
                <label
                  key={item.key}
                  className={`flex items-center gap-3 px-5 py-3 cursor-pointer transition-colors ${
                    selectedItems.has(item.key) ? 'bg-red-50/40' : 'hover:bg-gray-50/80'
                  }`}
                >
                  <div
                    className={`w-4.5 h-4.5 rounded-md border-2 flex items-center justify-center transition-all cursor-pointer shrink-0 ${
                      selectedItems.has(item.key)
                        ? 'bg-red-500 border-red-500'
                        : 'border-gray-200 hover:border-gray-300'
                    }`}
                    onClick={() => toggleItem(item.key)}
                  >
                    {selectedItems.has(item.key) && <Check className="w-3 h-3 text-white" />}
                  </div>
                  <input
                    type="checkbox"
                    checked={selectedItems.has(item.key)}
                    onChange={() => toggleItem(item.key)}
                    className="sr-only"
                  />
                  <span className="text-sm truncate flex-1 text-gray-700">{item.title}</span>
                  <span className="text-[10px] px-1.5 py-0.5 bg-gray-100 text-gray-400 rounded shrink-0">{item.item_type}</span>
                </label>
              ))
            )}
          </div>
        </div>
      </div>

      {/* Export button */}
      <div className="flex items-center justify-between pt-2">
        <p className="text-xs text-gray-400">
          已选择 <strong className="text-gray-600">{selectedItems.size}</strong> 篇文献
        </p>
        <button
          disabled={selectedItems.size === 0}
          className="inline-flex items-center gap-2 px-6 py-2.5 bg-gradient-to-r from-red-500 to-rose-600 text-white text-sm font-medium rounded-xl hover:shadow-lg hover:shadow-red-500/25 hover:-translate-y-0.5 active:translate-y-0 transition-all duration-200 disabled:opacity-30 disabled:cursor-not-allowed disabled:hover:shadow-none disabled:hover:translate-y-0"
        >
          <Download className="w-4 h-4" />
          导出为 {selectedFormat}
        </button>
      </div>
    </div>
  )
}
