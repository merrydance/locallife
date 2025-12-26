import { useState } from 'react';
import { Settings } from 'lucide-react';
import { Dashboard } from './components/Dashboard';
import { Sidebar } from './components/Sidebar';
import { DishManagement } from './components/DishManagement';
import { ComboManagement } from './components/ComboManagement';
import { TableSettings } from './components/TableSettings';
import { InventoryManagement } from './components/InventoryManagement';
import { KitchenDisplay } from './components/KitchenDisplay';
import { MemberManagement } from './components/MemberManagement';
import { CouponManagement } from './components/CouponManagement';
import { Analytics } from './components/Analytics';
import { RestaurantSettings } from './components/RestaurantSettings';

export type PageType = 'dishes' | 'combos' | 'tables' | 'inventory' | 'kitchen' | 'members' | 'coupons' | 'analytics' | 'restaurant';

export default function App() {
  const [isSettingsMode, setIsSettingsMode] = useState(false);
  const [currentPage, setCurrentPage] = useState<PageType>('dishes');

  if (!isSettingsMode) {
    return (
      <div className="h-screen bg-gray-50">
        <Dashboard onEnterSettings={() => setIsSettingsMode(true)} />
      </div>
    );
  }

  const renderPage = () => {
    switch (currentPage) {
      case 'dishes':
        return <DishManagement />;
      case 'combos':
        return <ComboManagement />;
      case 'tables':
        return <TableSettings />;
      case 'inventory':
        return <InventoryManagement />;
      case 'kitchen':
        return <KitchenDisplay />;
      case 'members':
        return <MemberManagement />;
      case 'coupons':
        return <CouponManagement />;
      case 'analytics':
        return <Analytics />;
      case 'restaurant':
        return <RestaurantSettings />;
      default:
        return <DishManagement />;
    }
  };

  return (
    <div className="flex h-screen bg-gray-50">
      <Sidebar 
        currentPage={currentPage} 
        onPageChange={setCurrentPage}
        onExitSettings={() => setIsSettingsMode(false)}
      />
      <main className="flex-1 overflow-auto">
        {renderPage()}
      </main>
    </div>
  );
}
