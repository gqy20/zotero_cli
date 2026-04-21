import { Outlet, NavLink } from 'react-router-dom'
import { BookOpen, LayoutDashboard, Search, Tags, Upload, Library } from 'lucide-react'

const navItems = [
  { to: '/dashboard', label: '总览', icon: LayoutDashboard },
  { to: '/library', label: '文献库', icon: Library },
  { to: '/search', label: '搜索', icon: Search },
  { to: '/tags', label: '标签', icon: Tags },
  { to: '/export', label: '导出', icon: Upload },
]

export default function Layout() {
  return (
    <div className="flex h-screen bg-gray-50">
      {/* Sidebar */}
      <aside className="w-56 border-r border-gray-200 bg-white flex flex-col">
        <div className="flex items-center gap-2 px-5 h-14 border-b border-gray-200">
          <BookOpen className="w-5 h-5 text-red-600" />
          <span className="font-semibold text-sm">Zotero Web</span>
        </div>
        <nav className="flex-1 px-3 py-4 space-y-1">
          {navItems.map(({ to, label, icon: Icon }) => (
            <NavLink
              key={to}
              to={to}
              end={to === '/dashboard'}
              className={({ isActive }) =>
                `flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors ${
                  isActive
                    ? 'bg-red-50 text-red-700 font-medium'
                    : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900'
                }`
              }
            >
              <Icon className="w-4 h-4" />
              {label}
            </NavLink>
          ))}
        </nav>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto">
        <Outlet />
      </main>
    </div>
  )
}
