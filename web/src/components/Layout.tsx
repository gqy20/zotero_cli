import { useState } from 'react'
import { Outlet, NavLink } from 'react-router-dom'
import { BookOpen, LayoutDashboard, Search, Tags, Upload, Library, Sparkles, Menu, X } from 'lucide-react'
import { Toaster } from '@/components/Toaster'

const navItems = [
  { to: '/dashboard', label: '总览', icon: LayoutDashboard },
  { to: '/library', label: '文献库', icon: Library },
  { to: '/search', label: '搜索', icon: Search },
  { to: '/tags', label: '标签', icon: Tags },
  { to: '/export', label: '导出', icon: Upload },
]

export default function Layout() {
  const [sidebarOpen, setSidebarOpen] = useState(false)

  return (
    <div className="flex h-screen bg-[#f8f9fc]">
      {/* Mobile menu button */}
      <button
        onClick={() => setSidebarOpen(!sidebarOpen)}
        className="md:hidden fixed top-4 left-4 z-50 w-10 h-10 flex items-center justify-center rounded-xl bg-white border border-gray-200 shadow-sm"
        aria-label={sidebarOpen ? '关闭菜单' : '打开菜单'}
        aria-expanded={sidebarOpen}
      >
        {sidebarOpen ? <X className="w-5 h-5 text-gray-600" /> : <Menu className="w-5 h-5 text-gray-600" />}
      </button>

      {/* Sidebar - overlay on mobile */}
      <aside
        className={`${
          sidebarOpen ? 'flex' : 'hidden'
        } md:flex flex-col w-60 border-r border-gray-200/80 bg-white/80 backdrop-blur-sm fixed md:relative inset-y-0 left-0 z-40 transition-transform duration-200`}
        aria-label="侧边栏"
      >
        {/* Mobile backdrop */}
        {sidebarOpen && (
          <div
            className="md:hidden fixed inset-0 bg-black/20 z-[-1]"
            onClick={() => setSidebarOpen(false)}
            aria-hidden="true"
          />
        )}

        {/* Brand */}
        <div className="flex items-center gap-3 px-5 h-16 border-b border-gray-100">
          <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-red-500 to-rose-600 flex items-center justify-center shadow-sm shadow-red-500/20">
            <BookOpen className="w-4 h-4 text-white" aria-hidden="true" />
          </div>
          <div>
            <span className="font-semibold text-sm text-gray-900 tracking-tight">Zotero Web</span>
            <p className="text-[10px] text-gray-400 -mt-0.5">Literature Manager</p>
          </div>
        </div>

        {/* Navigation */}
        <nav className="flex-1 px-3 py-4 space-y-0.5 overflow-y-auto" aria-label="主导航">
          {navItems.map(({ to, label, icon: Icon }) => (
            <NavLink
              key={to}
              to={to}
              end={to === '/dashboard'}
              onClick={() => setSidebarOpen(false)}
              className={({ isActive }) =>
                `group flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm transition-all duration-200 ${
                  isActive
                    ? 'bg-gradient-to-r from-red-50 to-rose-50 text-red-700 shadow-sm shadow-red-500/5 font-medium'
                    : 'text-gray-500 hover:text-gray-900 hover:bg-gray-50'
                }`
              }
            >
              <Icon className={`w-[18px] h-[18px] transition-colors duration-200 group-data-[active=true]:text-red-600 text-gray-400 group-hover:text-gray-600`} aria-hidden="true" />
              <span>{label}</span>
              {to === '/dashboard' && (
                <Sparkles className="w-3.5 h-3.5 ml-auto text-amber-500" aria-hidden="true" />
              )}
            </NavLink>
          ))}
        </nav>

        {/* Footer */}
        <div className="px-5 py-4 border-t border-gray-100">
          <div className="flex items-center gap-2.5 px-2 py-2 rounded-lg bg-gradient-to-r from-slate-50 to-gray-50">
            <div className="w-7 h-7 rounded-full bg-gradient-to-br from-indigo-400 to-purple-500 flex items-center justify-center text-[10px] font-semibold text-white">
              Z
            </div>
            <div className="min-w-0">
              <p className="text-xs font-medium text-gray-700 truncate">Local Library</p>
              <p className="text-[10px] text-gray-400">6,716 items</p>
            </div>
          </div>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto relative md:ml-0">
        <Outlet />
        <Toaster />
      </main>
    </div>
  )
}
