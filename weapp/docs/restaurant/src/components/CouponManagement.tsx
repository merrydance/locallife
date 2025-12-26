import { useState } from 'react';
import { Plus, Search, Edit2, Trash2, Gift } from 'lucide-react';

interface Coupon {
  id: string;
  name: string;
  type: 'discount' | 'cash' | 'gift';
  value: number;
  minSpend: number;
  stock: number;
  used: number;
  validFrom: string;
  validTo: string;
  status: 'active' | 'inactive' | 'expired';
}

const initialCoupons: Coupon[] = [
  {
    id: '1',
    name: '新客专享券',
    type: 'cash',
    value: 20,
    minSpend: 100,
    stock: 1000,
    used: 245,
    validFrom: '2025-12-01',
    validTo: '2025-12-31',
    status: 'active',
  },
  {
    id: '2',
    name: '满200减50',
    type: 'cash',
    value: 50,
    minSpend: 200,
    stock: 500,
    used: 180,
    validFrom: '2025-12-15',
    validTo: '2025-12-31',
    status: 'active',
  },
  {
    id: '3',
    name: '9折优惠券',
    type: 'discount',
    value: 0.9,
    minSpend: 0,
    stock: 2000,
    used: 850,
    validFrom: '2025-12-01',
    validTo: '2025-12-25',
    status: 'active',
  },
  {
    id: '4',
    name: '消费送饮料',
    type: 'gift',
    value: 0,
    minSpend: 150,
    stock: 300,
    used: 120,
    validFrom: '2025-11-01',
    validTo: '2025-11-30',
    status: 'expired',
  },
];

export function CouponManagement() {
  const [coupons, setCoupons] = useState<Coupon[]>(initialCoupons);
  const [searchTerm, setSearchTerm] = useState('');
  const [filter, setFilter] = useState<'all' | Coupon['status']>('all');

  const filteredCoupons = coupons.filter(coupon => {
    const matchesSearch = coupon.name.toLowerCase().includes(searchTerm.toLowerCase());
    const matchesFilter = filter === 'all' || coupon.status === filter;
    return matchesSearch && matchesFilter;
  });

  const handleDelete = (id: string) => {
    setCoupons(coupons.filter(c => c.id !== id));
  };

  const toggleStatus = (id: string) => {
    setCoupons(coupons.map(coupon =>
      coupon.id === id && coupon.status !== 'expired'
        ? { ...coupon, status: coupon.status === 'active' ? 'inactive' : 'active' }
        : coupon
    ));
  };

  const getTypeText = (type: Coupon['type']) => {
    switch (type) {
      case 'discount': return '折扣券';
      case 'cash': return '代金券';
      case 'gift': return '赠品券';
    }
  };

  const getTypeColor = (type: Coupon['type']) => {
    switch (type) {
      case 'discount': return 'bg-blue-100 text-blue-700';
      case 'cash': return 'bg-red-100 text-red-700';
      case 'gift': return 'bg-purple-100 text-purple-700';
    }
  };

  const getStatusColor = (status: Coupon['status']) => {
    switch (status) {
      case 'active': return 'bg-green-100 text-green-700';
      case 'inactive': return 'bg-gray-100 text-gray-700';
      case 'expired': return 'bg-red-100 text-red-700';
    }
  };

  const getStatusText = (status: Coupon['status']) => {
    switch (status) {
      case 'active': return '使用中';
      case 'inactive': return '已停用';
      case 'expired': return '已过期';
    }
  };

  return (
    <div className="p-8">
      <h2 className="text-gray-900 mb-6">优惠券管理</h2>

      <div className="flex gap-4 mb-6">
        <div className="flex-1 relative">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-400 w-5 h-5" />
          <input
            type="text"
            placeholder="搜索优惠券名称..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full pl-10 pr-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>
        <button
          onClick={() => setFilter('all')}
          className={`px-4 py-2 rounded-lg ${filter === 'all' ? 'bg-blue-600 text-white' : 'bg-white text-gray-700 border border-gray-300'}`}
        >
          全部
        </button>
        <button
          onClick={() => setFilter('active')}
          className={`px-4 py-2 rounded-lg ${filter === 'active' ? 'bg-blue-600 text-white' : 'bg-white text-gray-700 border border-gray-300'}`}
        >
          使用中
        </button>
        <button className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700">
          <Plus className="w-5 h-5" />
          创建优惠券
        </button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {filteredCoupons.map((coupon) => (
          <div key={coupon.id} className="bg-gradient-to-br from-white to-gray-50 rounded-lg shadow-lg p-6 border-2 border-dashed border-gray-200">
            <div className="flex justify-between items-start mb-4">
              <div className="flex items-center gap-2">
                <Gift className="w-5 h-5 text-blue-600" />
                <h3 className="text-gray-900">{coupon.name}</h3>
              </div>
              <span className={`px-2 py-1 rounded-full text-xs ${getStatusColor(coupon.status)}`}>
                {getStatusText(coupon.status)}
              </span>
            </div>

            <div className="mb-4">
              <span className={`px-3 py-1 rounded-full text-xs ${getTypeColor(coupon.type)}`}>
                {getTypeText(coupon.type)}
              </span>
            </div>

            <div className="mb-4 pb-4 border-b border-gray-200">
              {coupon.type === 'discount' ? (
                <p className="text-gray-900">
                  打 <span className="text-red-600">{coupon.value * 10}</span> 折
                </p>
              ) : coupon.type === 'cash' ? (
                <p className="text-gray-900">
                  满 ¥{coupon.minSpend} 减 <span className="text-red-600">¥{coupon.value}</span>
                </p>
              ) : (
                <p className="text-gray-900">
                  满 ¥{coupon.minSpend} 送赠品
                </p>
              )}
            </div>

            <div className="space-y-2 mb-4 text-gray-600">
              <div className="flex justify-between">
                <span>发放数量</span>
                <span>{coupon.stock}</span>
              </div>
              <div className="flex justify-between">
                <span>已使用</span>
                <span className="text-green-600">{coupon.used}</span>
              </div>
              <div className="flex justify-between">
                <span>剩余</span>
                <span className="text-blue-600">{coupon.stock - coupon.used}</span>
              </div>
              <div className="pt-2 border-t border-gray-200">
                <p className="text-xs">
                  有效期: {coupon.validFrom} 至 {coupon.validTo}
                </p>
              </div>
            </div>

            <div className="flex gap-2">
              {coupon.status !== 'expired' && (
                <button
                  onClick={() => toggleStatus(coupon.id)}
                  className={`flex-1 px-3 py-2 rounded-lg border transition-colors ${
                    coupon.status === 'active'
                      ? 'border-gray-300 text-gray-700 hover:bg-gray-50'
                      : 'border-green-600 text-green-600 hover:bg-green-50'
                  }`}
                >
                  {coupon.status === 'active' ? '停用' : '启用'}
                </button>
              )}
              <button className="p-2 text-blue-600 hover:bg-blue-50 rounded-lg">
                <Edit2 className="w-5 h-5" />
              </button>
              <button
                onClick={() => handleDelete(coupon.id)}
                className="p-2 text-red-600 hover:bg-red-50 rounded-lg"
              >
                <Trash2 className="w-5 h-5" />
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
