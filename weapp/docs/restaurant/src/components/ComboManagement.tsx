import { useState } from 'react';
import { Plus, Search, Edit2, Trash2, Upload } from 'lucide-react';

interface Combo {
  id: string;
  name: string;
  description: string;
  dishes: string[];
  originalPrice: number;
  comboPrice: number;
  status: 'active' | 'inactive';
  image?: string;
}

const initialCombos: Combo[] = [
  {
    id: '1',
    name: '经典双人套餐',
    description: '适合2人享用',
    dishes: ['宫保鸡丁', '红烧肉', '清蒸鲈鱼', '凉拌黄瓜'],
    originalPrice: 172,
    comboPrice: 138,
    status: 'active',
    image: 'https://images.unsplash.com/photo-1630564510802-0cac202af38d?crop=entropy&cs=tinysrgb&fit=max&fm=jpg&ixid=M3w3Nzg4Nzd8MHwxfHNlYXJjaHwxfHxjaGluZXNlJTIwZm9vZCUyMGRpc2h8ZW58MXx8fHwxNzY2MTkxOTA5fDA&ixlib=rb-4.1.0&q=80&w=400',
  },
  {
    id: '2',
    name: '午餐特惠套餐',
    description: '工作日午餐优惠',
    dishes: ['麻婆豆腐', '凉拌黄瓜', '米饭'],
    originalPrice: 56,
    comboPrice: 39,
    status: 'active',
  },
  {
    id: '3',
    name: '家庭聚餐套餐',
    description: '适合4-6人',
    dishes: ['宫保鸡丁', '红烧肉', '清蒸鲈鱼', '麻婆豆腐', '凉拌黄瓜'],
    originalPrice: 220,
    comboPrice: 188,
    status: 'active',
    image: 'https://images.unsplash.com/photo-1630564510802-0cac202af38d?crop=entropy&cs=tinysrgb&fit=max&fm=jpg&ixid=M3w3Nzg4Nzd8MHwxfHNlYXJjaHwxfHxjaGluZXNlJTIwZm9vZCUyMGRpc2h8ZW58MXx8fHwxNzY2MTkxOTA5fDA&ixlib=rb-4.1.0&q=80&w=400',
  },
];

export function ComboManagement() {
  const [combos, setCombos] = useState<Combo[]>(initialCombos);
  const [searchTerm, setSearchTerm] = useState('');

  const filteredCombos = combos.filter(combo =>
    combo.name.toLowerCase().includes(searchTerm.toLowerCase())
  );

  const handleDelete = (id: string) => {
    setCombos(combos.filter(c => c.id !== id));
  };

  const toggleStatus = (id: string) => {
    setCombos(combos.map(combo =>
      combo.id === id
        ? { ...combo, status: combo.status === 'active' ? 'inactive' : 'active' }
        : combo
    ));
  };

  return (
    <div className="p-8">
      <div className="mb-6">
        <h2 className="text-gray-900 mb-6">套餐管理</h2>
        <div className="flex gap-4">
          <div className="flex-1 relative">
            <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-400 w-5 h-5" />
            <input
              type="text"
              placeholder="搜索套餐名称..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="w-full pl-10 pr-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
          <button className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors">
            <Plus className="w-5 h-5" />
            添加套餐
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {filteredCombos.map((combo) => (
          <div key={combo.id} className="bg-white rounded-lg shadow overflow-hidden">
            {combo.image ? (
              <img src={combo.image} alt={combo.name} className="w-full h-48 object-cover" />
            ) : (
              <div className="w-full h-48 bg-gradient-to-br from-blue-100 to-purple-100 flex items-center justify-center">
                <Upload className="w-12 h-12 text-gray-400" />
              </div>
            )}

            <div className="p-6">
              <div className="flex justify-between items-start mb-3">
                <h3 className="text-gray-900">{combo.name}</h3>
                <span className={`px-2 py-1 rounded-full text-xs ${
                  combo.status === 'active'
                    ? 'bg-green-100 text-green-700'
                    : 'bg-gray-100 text-gray-700'
                }`}>
                  {combo.status === 'active' ? '启用' : '停用'}
                </span>
              </div>
              
              <p className="text-gray-500 mb-4">{combo.description}</p>
              
              <div className="mb-4">
                <p className="text-gray-600 mb-2">包含菜品：</p>
                <div className="flex flex-wrap gap-2">
                  {combo.dishes.map((dish, index) => (
                    <span key={index} className="px-2 py-1 bg-blue-50 text-blue-700 rounded text-xs">
                      {dish}
                    </span>
                  ))}
                </div>
              </div>

              <div className="mb-4 pb-4 border-b border-gray-200">
                <div className="flex items-baseline gap-2">
                  <span className="text-gray-900">¥{combo.comboPrice}</span>
                  <span className="text-gray-400 line-through">¥{combo.originalPrice}</span>
                  <span className="text-green-600 ml-auto">
                    省¥{combo.originalPrice - combo.comboPrice}
                  </span>
                </div>
              </div>

              <div className="flex gap-2">
                <button
                  onClick={() => toggleStatus(combo.id)}
                  className={`flex-1 px-3 py-2 rounded-lg border transition-colors ${
                    combo.status === 'active'
                      ? 'border-gray-300 text-gray-700 hover:bg-gray-50'
                      : 'border-green-600 text-green-600 hover:bg-green-50'
                  }`}
                >
                  {combo.status === 'active' ? '停用' : '启用'}
                </button>
                <button className="p-2 text-blue-600 hover:bg-blue-50 rounded-lg">
                  <Edit2 className="w-5 h-5" />
                </button>
                <button
                  onClick={() => handleDelete(combo.id)}
                  className="p-2 text-red-600 hover:bg-red-50 rounded-lg"
                >
                  <Trash2 className="w-5 h-5" />
                </button>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}