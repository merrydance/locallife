import { useState } from 'react';
import { Plus, Clock, CheckCircle, XCircle, DollarSign } from 'lucide-react';

interface OrderItem {
  dishName: string;
  quantity: number;
  price: number;
}

interface Order {
  id: string;
  tableNumber: string;
  items: OrderItem[];
  total: number;
  status: 'pending' | 'cooking' | 'serving' | 'completed' | 'cancelled';
  time: string;
  paymentMethod?: string;
}

const initialOrders: Order[] = [
  {
    id: 'ORD001',
    tableNumber: '3号桌',
    items: [
      { dishName: '宫保鸡丁', quantity: 2, price: 38 },
      { dishName: '清蒸鲈鱼', quantity: 1, price: 68 },
    ],
    total: 144,
    status: 'cooking',
    time: '12:30',
  },
  {
    id: 'ORD002',
    tableNumber: '5号桌',
    items: [
      { dishName: '红烧肉', quantity: 1, price: 48 },
      { dishName: '麻婆豆腐', quantity: 2, price: 28 },
      { dishName: '凉拌黄瓜', quantity: 1, price: 18 },
    ],
    total: 122,
    status: 'pending',
    time: '12:45',
  },
  {
    id: 'ORD003',
    tableNumber: '8号桌',
    items: [
      { dishName: '宫保鸡丁', quantity: 1, price: 38 },
      { dishName: '凉拌黄瓜', quantity: 2, price: 18 },
    ],
    total: 74,
    status: 'serving',
    time: '12:15',
  },
];

export function OrderManagement() {
  const [orders, setOrders] = useState<Order[]>(initialOrders);
  const [filter, setFilter] = useState<'all' | Order['status']>('all');

  const filteredOrders = filter === 'all' 
    ? orders 
    : orders.filter(order => order.status === filter);

  const updateOrderStatus = (id: string, status: Order['status']) => {
    setOrders(orders.map(order => 
      order.id === id ? { ...order, status } : order
    ));
  };

  const getStatusColor = (status: Order['status']) => {
    switch (status) {
      case 'pending': return 'bg-yellow-100 text-yellow-700';
      case 'cooking': return 'bg-blue-100 text-blue-700';
      case 'serving': return 'bg-purple-100 text-purple-700';
      case 'completed': return 'bg-green-100 text-green-700';
      case 'cancelled': return 'bg-red-100 text-red-700';
    }
  };

  const getStatusText = (status: Order['status']) => {
    switch (status) {
      case 'pending': return '待处理';
      case 'cooking': return '制作中';
      case 'serving': return '已上菜';
      case 'completed': return '已完成';
      case 'cancelled': return '已取消';
    }
  };

  return (
    <div className="p-8">
      <div className="mb-6">
        <h2 className="text-gray-900 mb-6">订单管理</h2>
        <div className="flex gap-2 mb-4">
          <button
            onClick={() => setFilter('all')}
            className={`px-4 py-2 rounded-lg ${filter === 'all' ? 'bg-blue-600 text-white' : 'bg-white text-gray-700 border border-gray-300'}`}
          >
            全部
          </button>
          <button
            onClick={() => setFilter('pending')}
            className={`px-4 py-2 rounded-lg ${filter === 'pending' ? 'bg-blue-600 text-white' : 'bg-white text-gray-700 border border-gray-300'}`}
          >
            待处理
          </button>
          <button
            onClick={() => setFilter('cooking')}
            className={`px-4 py-2 rounded-lg ${filter === 'cooking' ? 'bg-blue-600 text-white' : 'bg-white text-gray-700 border border-gray-300'}`}
          >
            制作中
          </button>
          <button
            onClick={() => setFilter('serving')}
            className={`px-4 py-2 rounded-lg ${filter === 'serving' ? 'bg-blue-600 text-white' : 'bg-white text-gray-700 border border-gray-300'}`}
          >
            已上菜
          </button>
          <button
            onClick={() => setFilter('completed')}
            className={`px-4 py-2 rounded-lg ${filter === 'completed' ? 'bg-blue-600 text-white' : 'bg-white text-gray-700 border border-gray-300'}`}
          >
            已完成
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {filteredOrders.map((order) => (
          <div key={order.id} className="bg-white rounded-lg shadow p-6">
            <div className="flex justify-between items-start mb-4">
              <div>
                <h3 className="text-gray-900">{order.tableNumber}</h3>
                <p className="text-gray-500">订单号: {order.id}</p>
              </div>
              <span className={`px-3 py-1 rounded-full text-xs ${getStatusColor(order.status)}`}>
                {getStatusText(order.status)}
              </span>
            </div>

            <div className="mb-4 space-y-2">
              {order.items.map((item, index) => (
                <div key={index} className="flex justify-between text-gray-600">
                  <span>{item.dishName} x{item.quantity}</span>
                  <span>¥{item.price * item.quantity}</span>
                </div>
              ))}
            </div>

            <div className="border-t border-gray-200 pt-4 mb-4">
              <div className="flex justify-between items-center">
                <span className="text-gray-700">总计</span>
                <span className="text-gray-900">¥{order.total}</span>
              </div>
              <div className="flex items-center gap-2 mt-2 text-gray-500">
                <Clock className="w-4 h-4" />
                <span>{order.time}</span>
              </div>
            </div>

            <div className="flex gap-2">
              {order.status === 'pending' && (
                <button
                  onClick={() => updateOrderStatus(order.id, 'cooking')}
                  className="flex-1 px-3 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                >
                  开始制作
                </button>
              )}
              {order.status === 'cooking' && (
                <button
                  onClick={() => updateOrderStatus(order.id, 'serving')}
                  className="flex-1 px-3 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700"
                >
                  上菜
                </button>
              )}
              {order.status === 'serving' && (
                <button
                  onClick={() => updateOrderStatus(order.id, 'completed')}
                  className="flex-1 px-3 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700"
                >
                  完成订单
                </button>
              )}
              {(order.status === 'pending' || order.status === 'cooking') && (
                <button
                  onClick={() => updateOrderStatus(order.id, 'cancelled')}
                  className="px-3 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700"
                >
                  <XCircle className="w-5 h-5" />
                </button>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
