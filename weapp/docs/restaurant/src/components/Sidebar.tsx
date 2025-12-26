import { UtensilsCrossed, Package2, Grid3x3, Archive, Printer, Users, Ticket, TrendingUp, Home, Store } from 'lucide-react';
import type { PageType } from '../App';

interface SidebarProps {
  currentPage: PageType;
  onPageChange: (page: PageType) => void;
  onExitSettings: () => void;
}

const menuItems = [
  { id: 'dishes' as PageType, icon: UtensilsCrossed, label: '菜品管理' },
  { id: 'combos' as PageType, icon: Package2, label: '套餐管理' },
  { id: 'tables' as PageType, icon: Grid3x3, label: '桌台设置' },
  { id: 'inventory' as PageType, icon: Archive, label: '库存管理' },
  { id: 'kitchen' as PageType, icon: Printer, label: '厨房显示' },
  { id: 'members' as PageType, icon: Users, label: '会员管理' },
  { id: 'coupons' as PageType, icon: Ticket, label: '优惠券' },
  { id: 'analytics' as PageType, icon: TrendingUp, label: '经营分析' },
  { id: 'restaurant' as PageType, icon: Store, label: '餐厅设置' },
];

export function Sidebar({ currentPage, onPageChange, onExitSettings }: SidebarProps) {
  return (
    <aside className="w-64 bg-white border-r border-gray-200 flex flex-col">
      <div className="p-6 border-b border-gray-200">
        <h1 className="text-gray-900">系统设置</h1>
      </div>
      
      <button
        onClick={onExitSettings}
        className="m-4 flex items-center gap-3 px-4 py-3 rounded-lg bg-blue-50 text-blue-600 hover:bg-blue-100"
      >
        <Home className="w-5 h-5" />
        <span>返回首页</span>
      </button>

      <nav className="flex-1 p-4">
        {menuItems.map((item) => {
          const Icon = item.icon;
          const isActive = currentPage === item.id;
          return (
            <button
              key={item.id}
              onClick={() => onPageChange(item.id)}
              className={`w-full flex items-center gap-3 px-4 py-3 rounded-lg mb-1 transition-colors ${
                isActive
                  ? 'bg-blue-50 text-blue-600'
                  : 'text-gray-700 hover:bg-gray-50'
              }`}
            >
              <Icon className="w-5 h-5" />
              <span>{item.label}</span>
            </button>
          );
        })}
      </nav>
    </aside>
  );
}
