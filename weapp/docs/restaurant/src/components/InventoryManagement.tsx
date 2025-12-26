import { useState } from 'react';
import { Search, AlertTriangle, Plus, Edit2 } from 'lucide-react';

interface InventoryItem {
  id: string;
  dishName: string;
  category: string;
  dailyStock: number;
  soldToday: number;
  remaining: number;
  date: string;
}

const initialInventory: InventoryItem[] = [
  { id: '1', dishName: '宫保鸡丁', category: '热菜', dailyStock: 50, soldToday: 25, remaining: 25, date: '2025-12-20' },
  { id: '2', dishName: '红烧肉', category: '热菜', dailyStock: 40, soldToday: 18, remaining: 22, date: '2025-12-20' },
  { id: '3', dishName: '清蒸鲈鱼', category: '海鲜', dailyStock: 30, soldToday: 28, remaining: 2, date: '2025-12-20' },
  { id: '4', dishName: '麻婆豆腐', category: '热菜', dailyStock: 60, soldToday: 35, remaining: 25, date: '2025-12-20' },
  { id: '5', dishName: '凉拌黄瓜', category: '凉菜', dailyStock: 80, soldToday: 42, remaining: 38, date: '2025-12-20' },
  { id: '6', dishName: '糖醋里脊', category: '热菜', dailyStock: 35, soldToday: 32, remaining: 3, date: '2025-12-20' },
];

export function InventoryManagement() {
  const [inventory, setInventory] = useState<InventoryItem[]>(initialInventory);
  const [searchTerm, setSearchTerm] = useState('');
  const [filter, setFilter] = useState<'all' | 'low'>('all');
  const [showEditModal, setShowEditModal] = useState(false);
  const [selectedItem, setSelectedItem] = useState<InventoryItem | null>(null);

  const filteredInventory = inventory.filter(item => {
    const matchesSearch = item.dishName.toLowerCase().includes(searchTerm.toLowerCase()) ||
                         item.category.toLowerCase().includes(searchTerm.toLowerCase());
    const matchesFilter = filter === 'all' || (filter === 'low' && item.remaining <= 5);
    return matchesSearch && matchesFilter;
  });

  const lowStockCount = inventory.filter(item => item.remaining <= 5).length;

  const handleEdit = (item: InventoryItem) => {
    setSelectedItem(item);
    setShowEditModal(true);
  };

  return (
    <div className="p-8">
      <h2 className="text-gray-900 mb-6">成品库存管理</h2>

      {lowStockCount > 0 && (
        <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-4 mb-6 flex items-center gap-3">
          <AlertTriangle className="w-5 h-5 text-yellow-600" />
          <span className="text-yellow-800">
            有 {lowStockCount} 道菜品库存不足（剩余≤5份），请注意补货或暂停售卖
          </span>
        </div>
      )}

      <div className="flex gap-4 mb-6">
        <div className="flex-1 relative">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-400 w-5 h-5" />
          <input
            type="text"
            placeholder="搜索菜品名称或分类..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full pl-10 pr-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>
        <button
          onClick={() => setFilter(filter === 'all' ? 'low' : 'all')}
          className={`px-4 py-2 rounded-lg ${
            filter === 'low'
              ? 'bg-yellow-600 text-white'
              : 'bg-white text-gray-700 border border-gray-300'
          }`}
        >
          库存不足
        </button>
        <button className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700">
          <Plus className="w-5 h-5" />
          批量录入
        </button>
      </div>

      <div className="bg-white rounded-lg shadow">
        <table className="w-full">
          <thead className="bg-gray-50 border-b border-gray-200">
            <tr>
              <th className="px-6 py-3 text-left text-gray-600">菜品名称</th>
              <th className="px-6 py-3 text-left text-gray-600">分类</th>
              <th className="px-6 py-3 text-left text-gray-600">今日备货</th>
              <th className="px-6 py-3 text-left text-gray-600">已售出</th>
              <th className="px-6 py-3 text-left text-gray-600">剩余</th>
              <th className="px-6 py-3 text-left text-gray-600">售罄率</th>
              <th className="px-6 py-3 text-left text-gray-600">日期</th>
              <th className="px-6 py-3 text-left text-gray-600">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-200">
            {filteredInventory.map((item) => {
              const sellRate = (item.soldToday / item.dailyStock * 100).toFixed(0);
              return (
                <tr key={item.id} className="hover:bg-gray-50">
                  <td className="px-6 py-4 text-gray-900">{item.dishName}</td>
                  <td className="px-6 py-4 text-gray-600">{item.category}</td>
                  <td className="px-6 py-4 text-gray-900">{item.dailyStock}份</td>
                  <td className="px-6 py-4 text-blue-600">{item.soldToday}份</td>
                  <td className="px-6 py-4">
                    <span className={item.remaining <= 5 ? 'text-red-600' : 'text-green-600'}>
                      {item.remaining}份
                    </span>
                    {item.remaining <= 5 && (
                      <AlertTriangle className="inline w-4 h-4 ml-2 text-yellow-600" />
                    )}
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-2">
                      <div className="w-20 h-2 bg-gray-200 rounded-full overflow-hidden">
                        <div 
                          className={`h-full ${
                            parseInt(sellRate) >= 80 ? 'bg-green-500' :
                            parseInt(sellRate) >= 50 ? 'bg-blue-500' : 'bg-gray-400'
                          }`}
                          style={{ width: `${sellRate}%` }}
                        />
                      </div>
                      <span className="text-gray-600 text-xs">{sellRate}%</span>
                    </div>
                  </td>
                  <td className="px-6 py-4 text-gray-600">{item.date}</td>
                  <td className="px-6 py-4">
                    <button 
                      onClick={() => handleEdit(item)}
                      className="px-3 py-1 bg-blue-600 text-white rounded hover:bg-blue-700 text-xs flex items-center gap-1"
                    >
                      <Edit2 className="w-3 h-3" />
                      调整
                    </button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {showEditModal && selectedItem && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-8 max-w-md w-full mx-4">
            <h3 className="text-gray-900 mb-4">调整库存 - {selectedItem.dishName}</h3>
            <div className="mb-4">
              <p className="text-gray-600 mb-2">当前库存: {selectedItem.remaining}份</p>
              <p className="text-gray-600 mb-2">已售出: {selectedItem.soldToday}份</p>
              <p className="text-gray-600 mb-4">今日备货: {selectedItem.dailyStock}份</p>
            </div>
            <div className="mb-6">
              <label className="block text-gray-700 mb-2">调整今日备货数量</label>
              <input
                type="number"
                defaultValue={selectedItem.dailyStock}
                className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
              <p className="text-gray-500 text-xs mt-2">提示：调整后剩余库存 = 新备货数 - 已售出数</p>
            </div>
            <div className="flex gap-3">
              <button
                onClick={() => {
                  setShowEditModal(false);
                  setSelectedItem(null);
                }}
                className="flex-1 px-4 py-2 bg-gray-200 text-gray-700 rounded-lg hover:bg-gray-300"
              >
                取消
              </button>
              <button
                onClick={() => {
                  setShowEditModal(false);
                  setSelectedItem(null);
                }}
                className="flex-1 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
              >
                确认调整
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}