import { useState } from 'react';
import { Plus, Search, Edit2, Trash2, Upload } from 'lucide-react';

interface Dish {
  id: string;
  name: string;
  category: string;
  price: number;
  cost: number;
  stock: number;
  status: 'available' | 'unavailable';
  image?: string;
  description?: string;
}

const initialDishes: Dish[] = [
  { 
    id: '1', 
    name: '宫保鸡丁', 
    category: '热菜', 
    price: 38, 
    cost: 15, 
    stock: 100, 
    status: 'available',
    image: 'https://images.unsplash.com/photo-1630564510802-0cac202af38d?crop=entropy&cs=tinysrgb&fit=max&fm=jpg&ixid=M3w3Nzg4Nzd8MHwxfHNlYXJjaHwxfHxjaGluZXNlJTIwZm9vZCUyMGRpc2h8ZW58MXx8fHwxNzY2MTkxOTA5fDA&ixlib=rb-4.1.0&q=80&w=400',
    description: '经典川菜，香辣可口'
  },
  { id: '2', name: '红烧肉', category: '热菜', price: 48, cost: 20, stock: 80, status: 'available', image: 'https://images.unsplash.com/photo-1630564510802-0cac202af38d?crop=entropy&cs=tinysrgb&fit=max&fm=jpg&ixid=M3w3Nzg4Nzd8MHwxfHNlYXJjaHwxfHxjaGluZXNlJTIwZm9vZCUyMGRpc2h8ZW58MXx8fHwxNzY2MTkxOTA5fDA&ixlib=rb-4.1.0&q=80&w=400', description: '肥而不腻，入口即化' },
  { id: '3', name: '清蒸鲈鱼', category: '海鲜', price: 68, cost: 35, stock: 50, status: 'available', image: 'https://images.unsplash.com/photo-1630564510802-0cac202af38d?crop=entropy&cs=tinysrgb&fit=max&fm=jpg&ixid=M3w3Nzg4Nzd8MHwxfHNlYXJjaHwxfHxjaGluZXNlJTIwZm9vZCUyMGRpc2h8ZW58MXx8fHwxNzY2MTkxOTA5fDA&ixlib=rb-4.1.0&q=80&w=400', description: '鲜嫩可口，营养丰富' },
  { id: '4', name: '麻婆豆腐', category: '热菜', price: 28, cost: 10, stock: 120, status: 'available', description: '麻辣鲜香，下饭首选' },
  { id: '5', name: '凉拌黄瓜', category: '凉菜', price: 18, cost: 5, stock: 150, status: 'available', description: '清爽开胃' },
];

export function DishManagement() {
  const [dishes, setDishes] = useState<Dish[]>(initialDishes);
  const [searchTerm, setSearchTerm] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [editingDish, setEditingDish] = useState<Dish | null>(null);

  const filteredDishes = dishes.filter(dish =>
    dish.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
    dish.category.toLowerCase().includes(searchTerm.toLowerCase())
  );

  const handleDelete = (id: string) => {
    setDishes(dishes.filter(d => d.id !== id));
  };

  const handleEdit = (dish: Dish) => {
    setEditingDish(dish);
    setShowModal(true);
  };

  const handleAdd = () => {
    setEditingDish(null);
    setShowModal(true);
  };

  return (
    <div className="p-8">
      <div className="mb-6">
        <h2 className="text-gray-900 mb-6">菜品管理</h2>
        <div className="flex gap-4">
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
            onClick={handleAdd}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
          >
            <Plus className="w-5 h-5" />
            添加菜品
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
        {filteredDishes.map((dish) => (
          <div key={dish.id} className="bg-white rounded-lg shadow overflow-hidden">
            {dish.image ? (
              <img src={dish.image} alt={dish.name} className="w-full h-48 object-cover" />
            ) : (
              <div className="w-full h-48 bg-gray-200 flex items-center justify-center">
                <Upload className="w-12 h-12 text-gray-400" />
              </div>
            )}
            
            <div className="p-4">
              <div className="flex justify-between items-start mb-2">
                <h3 className="text-gray-900">{dish.name}</h3>
                <span className={`px-2 py-1 rounded-full text-xs ${
                  dish.status === 'available'
                    ? 'bg-green-100 text-green-700'
                    : 'bg-red-100 text-red-700'
                }`}>
                  {dish.status === 'available' ? '可售' : '售罄'}
                </span>
              </div>

              <p className="text-gray-500 mb-2">{dish.category}</p>
              {dish.description && (
                <p className="text-gray-600 mb-3 text-xs line-clamp-2">{dish.description}</p>
              )}

              <div className="flex justify-between items-center mb-3 pb-3 border-b border-gray-200">
                <div>
                  <p className="text-gray-900">¥{dish.price}</p>
                  <p className="text-gray-500 text-xs">成本: ¥{dish.cost}</p>
                </div>
                <div className="text-right">
                  <p className="text-gray-600">库存</p>
                  <p className="text-gray-900">{dish.stock}</p>
                </div>
              </div>

              <div className="flex gap-2">
                <button
                  onClick={() => handleEdit(dish)}
                  className="flex-1 px-3 py-2 bg-blue-50 text-blue-600 rounded-lg hover:bg-blue-100 flex items-center justify-center gap-1 text-xs"
                >
                  <Edit2 className="w-3 h-3" />
                  编辑
                </button>
                <button
                  onClick={() => handleDelete(dish.id)}
                  className="flex-1 px-3 py-2 bg-red-50 text-red-600 rounded-lg hover:bg-red-100 flex items-center justify-center gap-1 text-xs"
                >
                  <Trash2 className="w-3 h-3" />
                  删除
                </button>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}