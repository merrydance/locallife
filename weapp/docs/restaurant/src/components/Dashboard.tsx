import { useState } from 'react';
import { Settings, Users, Clock, MapPin, ShoppingBag, Package, Calendar, Utensils, TrendingUp, Ticket } from 'lucide-react';

interface Order {
  id: string;
  type: 'dine-in' | 'takeout' | 'delivery' | 'reservation';
  tableNumber?: string;
  customerName?: string;
  phone?: string;
  items: { name: string; quantity: number }[];
  total: number;
  status: 'pending' | 'confirmed' | 'preparing' | 'ready' | 'completed';
  time: string;
  reservationTime?: string;
}

interface Table {
  id: string;
  number: string;
  type: 'table' | 'room';
  seats: number;
  status: 'available' | 'occupied' | 'reserved';
  currentOrder?: string;
  startTime?: string;
  image?: string;
}

const mockOrders: Order[] = [
  {
    id: 'ORD001',
    type: 'dine-in',
    tableNumber: '3号桌',
    items: [{ name: '宫保鸡丁', quantity: 2 }, { name: '清蒸鲈鱼', quantity: 1 }],
    total: 144,
    status: 'preparing',
    time: '12:30',
  },
  {
    id: 'ORD002',
    type: 'delivery',
    customerName: '张先生',
    phone: '138****1234',
    items: [{ name: '红烧肉', quantity: 1 }, { name: '麻婆豆腐', quantity: 2 }],
    total: 104,
    status: 'confirmed',
    time: '12:45',
  },
  {
    id: 'ORD003',
    type: 'takeout',
    customerName: '李女士',
    phone: '139****5678',
    items: [{ name: '宫保鸡丁', quantity: 1 }],
    total: 38,
    status: 'ready',
    time: '12:50',
  },
  {
    id: 'ORD004',
    type: 'reservation',
    customerName: '王先生',
    phone: '136****9012',
    tableNumber: '包间A',
    items: [],
    total: 0,
    status: 'pending',
    time: '11:00',
    reservationTime: '18:00',
  },
];

const mockTables: Table[] = [
  { id: '1', number: '1号桌', type: 'table', seats: 2, status: 'available' },
  { id: '2', number: '2号桌', type: 'table', seats: 2, status: 'available' },
  { id: '3', number: '3号桌', type: 'table', seats: 4, status: 'occupied', currentOrder: 'ORD001', startTime: '12:30' },
  { id: '4', number: '4号桌', type: 'table', seats: 4, status: 'available' },
  { id: '5', number: '5号桌', type: 'table', seats: 4, status: 'available' },
  { id: '6', number: '6号桌', type: 'table', seats: 6, status: 'available' },
  { id: '7', number: '包间A', type: 'room', seats: 8, status: 'reserved' },
  { id: '8', number: '包间B', type: 'room', seats: 10, status: 'available' },
];

interface DashboardProps {
  onEnterSettings: () => void;
}

export function Dashboard({ onEnterSettings }: DashboardProps) {
  const [orders] = useState<Order[]>(mockOrders);
  const [tables] = useState<Table[]>(mockTables);
  const [orderFilter, setOrderFilter] = useState<'all' | Order['type']>('all');

  const filteredOrders = orderFilter === 'all' 
    ? orders 
    : orders.filter(o => o.type === orderFilter);

  const getOrderTypeIcon = (type: Order['type']) => {
    switch (type) {
      case 'dine-in': return <Utensils className="w-4 h-4" />;
      case 'delivery': return <ShoppingBag className="w-4 h-4" />;
      case 'takeout': return <Package className="w-4 h-4" />;
      case 'reservation': return <Calendar className="w-4 h-4" />;
    }
  };

  const getOrderTypeText = (type: Order['type']) => {
    switch (type) {
      case 'dine-in': return '堂食';
      case 'delivery': return '外卖';
      case 'takeout': return '外带';
      case 'reservation': return '预定';
    }
  };

  const getOrderTypeColor = (type: Order['type']) => {
    switch (type) {
      case 'dine-in': return 'bg-blue-100 text-blue-700';
      case 'delivery': return 'bg-green-100 text-green-700';
      case 'takeout': return 'bg-purple-100 text-purple-700';
      case 'reservation': return 'bg-orange-100 text-orange-700';
    }
  };

  const getStatusColor = (status: Order['status']) => {
    switch (status) {
      case 'pending': return 'bg-gray-100 text-gray-700';
      case 'confirmed': return 'bg-yellow-100 text-yellow-700';
      case 'preparing': return 'bg-blue-100 text-blue-700';
      case 'ready': return 'bg-green-100 text-green-700';
      case 'completed': return 'bg-gray-100 text-gray-500';
    }
  };

  const getStatusText = (status: Order['status']) => {
    switch (status) {
      case 'pending': return '待确认';
      case 'confirmed': return '已确认';
      case 'preparing': return '制作中';
      case 'ready': return '待取餐';
      case 'completed': return '已完成';
    }
  };

  const stats = {
    available: tables.filter(t => t.status === 'available').length,
    occupied: tables.filter(t => t.status === 'occupied').length,
    pendingOrders: orders.filter(o => o.status === 'pending' || o.status === 'confirmed').length,
    todayRevenue: 5680,
  };

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <header className="bg-white border-b border-gray-200 px-6 py-4">
        <div className="flex justify-between items-center">
          <div>
            <h1 className="text-gray-900">欢乐餐厅管理系统</h1>
            <p className="text-gray-500 mt-1">2025年12月20日 星期六</p>
          </div>
          
          <div className="flex gap-4">
            <button 
              onClick={onEnterSettings}
              className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
            >
              <Utensils className="w-5 h-5" />
              菜品管理
            </button>
            <button 
              onClick={onEnterSettings}
              className="flex items-center gap-2 px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700"
            >
              <Users className="w-5 h-5" />
              会员中心
            </button>
            <button 
              onClick={onEnterSettings}
              className="flex items-center gap-2 px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700"
            >
              <Ticket className="w-5 h-5" />
              优惠券
            </button>
            <button 
              onClick={onEnterSettings}
              className="flex items-center gap-2 px-4 py-2 bg-orange-600 text-white rounded-lg hover:bg-orange-700"
            >
              <TrendingUp className="w-5 h-5" />
              经营分析
            </button>
            <button 
              onClick={onEnterSettings}
              className="flex items-center gap-2 px-4 py-2 bg-gray-700 text-white rounded-lg hover:bg-gray-800"
            >
              <Settings className="w-5 h-5" />
              系统设置
            </button>
          </div>
        </div>
      </header>

      {/* Stats Bar */}
      <div className="bg-white border-b border-gray-200 px-6 py-4">
        <div className="grid grid-cols-4 gap-4">
          <div className="flex items-center gap-3">
            <div className="w-12 h-12 bg-green-100 rounded-lg flex items-center justify-center">
              <MapPin className="w-6 h-6 text-green-600" />
            </div>
            <div>
              <p className="text-gray-500">空闲桌台</p>
              <p className="text-gray-900">{stats.available} 个</p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <div className="w-12 h-12 bg-red-100 rounded-lg flex items-center justify-center">
              <Users className="w-6 h-6 text-red-600" />
            </div>
            <div>
              <p className="text-gray-500">使用中</p>
              <p className="text-gray-900">{stats.occupied} 个</p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <div className="w-12 h-12 bg-yellow-100 rounded-lg flex items-center justify-center">
              <Clock className="w-6 h-6 text-yellow-600" />
            </div>
            <div>
              <p className="text-gray-500">待处理订单</p>
              <p className="text-gray-900">{stats.pendingOrders} 个</p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <div className="w-12 h-12 bg-blue-100 rounded-lg flex items-center justify-center">
              <TrendingUp className="w-6 h-6 text-blue-600" />
            </div>
            <div>
              <p className="text-gray-500">今日营业额</p>
              <p className="text-gray-900">¥{stats.todayRevenue}</p>
            </div>
          </div>
        </div>
      </div>

      <div className="flex-1 flex overflow-hidden">
        {/* Left: Orders List */}
        <div className="w-96 bg-white border-r border-gray-200 flex flex-col">
          <div className="p-4 border-b border-gray-200">
            <h2 className="text-gray-900 mb-4">实时订单</h2>
            <div className="flex gap-2 overflow-x-auto">
              <button
                onClick={() => setOrderFilter('all')}
                className={`px-3 py-1 rounded-lg whitespace-nowrap text-xs ${
                  orderFilter === 'all' ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-700'
                }`}
              >
                全部
              </button>
              <button
                onClick={() => setOrderFilter('dine-in')}
                className={`px-3 py-1 rounded-lg whitespace-nowrap text-xs ${
                  orderFilter === 'dine-in' ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-700'
                }`}
              >
                堂食
              </button>
              <button
                onClick={() => setOrderFilter('delivery')}
                className={`px-3 py-1 rounded-lg whitespace-nowrap text-xs ${
                  orderFilter === 'delivery' ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-700'
                }`}
              >
                外卖
              </button>
              <button
                onClick={() => setOrderFilter('takeout')}
                className={`px-3 py-1 rounded-lg whitespace-nowrap text-xs ${
                  orderFilter === 'takeout' ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-700'
                }`}
              >
                外带
              </button>
              <button
                onClick={() => setOrderFilter('reservation')}
                className={`px-3 py-1 rounded-lg whitespace-nowrap text-xs ${
                  orderFilter === 'reservation' ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-700'
                }`}
              >
                预定
              </button>
            </div>
          </div>

          <div className="flex-1 overflow-auto p-4 space-y-3">
            {filteredOrders.map((order) => (
              <div key={order.id} className="bg-gray-50 rounded-lg p-4 border border-gray-200 hover:border-blue-300 cursor-pointer transition-colors">
                <div className="flex justify-between items-start mb-3">
                  <div>
                    <div className="flex items-center gap-2 mb-1">
                      <span className={`flex items-center gap-1 px-2 py-1 rounded text-xs ${getOrderTypeColor(order.type)}`}>
                        {getOrderTypeIcon(order.type)}
                        {getOrderTypeText(order.type)}
                      </span>
                      <span className={`px-2 py-1 rounded text-xs ${getStatusColor(order.status)}`}>
                        {getStatusText(order.status)}
                      </span>
                    </div>
                    <p className="text-gray-900">{order.id}</p>
                  </div>
                  <p className="text-gray-500">{order.time}</p>
                </div>

                {order.tableNumber && (
                  <p className="text-gray-700 mb-2">{order.tableNumber}</p>
                )}
                {order.customerName && (
                  <p className="text-gray-700 mb-2">{order.customerName} {order.phone}</p>
                )}
                {order.reservationTime && (
                  <p className="text-orange-600 mb-2">预定时间: {order.reservationTime}</p>
                )}

                {order.items.length > 0 && (
                  <div className="space-y-1 mb-3">
                    {order.items.map((item, idx) => (
                      <p key={idx} className="text-gray-600">
                        {item.name} x{item.quantity}
                      </p>
                    ))}
                  </div>
                )}

                <div className="flex justify-between items-center pt-3 border-t border-gray-200">
                  <span className="text-gray-900">¥{order.total}</span>
                  {order.status === 'pending' && (
                    <button className="px-3 py-1 bg-blue-600 text-white rounded text-xs hover:bg-blue-700">
                      确认订单
                    </button>
                  )}
                  {order.status === 'ready' && (
                    <button className="px-3 py-1 bg-green-600 text-white rounded text-xs hover:bg-green-700">
                      完成
                    </button>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Right: Table Status */}
        <div className="flex-1 p-6 overflow-auto">
          <h2 className="text-gray-900 mb-6">桌台状态</h2>
          
          <div className="mb-6">
            <h3 className="text-gray-700 mb-4">普通桌台</h3>
            <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
              {tables.filter(t => t.type === 'table').map((table) => (
                <div
                  key={table.id}
                  className={`bg-white rounded-lg shadow p-6 relative border-2 cursor-pointer transition-all ${
                    table.status === 'available' 
                      ? 'border-green-200 hover:border-green-400' 
                      : table.status === 'occupied'
                      ? 'border-red-200'
                      : 'border-yellow-200'
                  }`}
                >
                  <div className={`absolute top-3 right-3 w-3 h-3 rounded-full ${
                    table.status === 'available' ? 'bg-green-500' :
                    table.status === 'occupied' ? 'bg-red-500' : 'bg-yellow-500'
                  }`} />
                  
                  <h3 className="text-gray-900 mb-2">{table.number}</h3>
                  <div className="flex items-center gap-2 text-gray-600 mb-3">
                    <Users className="w-4 h-4" />
                    <span>{table.seats}人座</span>
                  </div>

                  {table.status === 'occupied' && table.currentOrder && (
                    <div className="pt-3 border-t border-gray-200">
                      <p className="text-gray-500 mb-1">订单: {table.currentOrder}</p>
                      <div className="flex items-center gap-1 text-gray-600">
                        <Clock className="w-3 h-3" />
                        <span className="text-xs">{table.startTime}</span>
                      </div>
                    </div>
                  )}

                  {table.status === 'available' && (
                    <p className="text-green-600 text-xs">可用</p>
                  )}
                  {table.status === 'reserved' && (
                    <p className="text-yellow-600 text-xs">已预订</p>
                  )}
                </div>
              ))}
            </div>
          </div>

          <div>
            <h3 className="text-gray-700 mb-4">包间</h3>
            <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
              {tables.filter(t => t.type === 'room').map((table) => (
                <div
                  key={table.id}
                  className={`bg-white rounded-lg shadow p-6 relative border-2 cursor-pointer transition-all ${
                    table.status === 'available' 
                      ? 'border-green-200 hover:border-green-400' 
                      : table.status === 'occupied'
                      ? 'border-red-200'
                      : 'border-yellow-200'
                  }`}
                >
                  <div className={`absolute top-3 right-3 w-3 h-3 rounded-full ${
                    table.status === 'available' ? 'bg-green-500' :
                    table.status === 'occupied' ? 'bg-red-500' : 'bg-yellow-500'
                  }`} />
                  
                  <h3 className="text-gray-900 mb-2">{table.number}</h3>
                  <div className="flex items-center gap-2 text-gray-600 mb-3">
                    <Users className="w-4 h-4" />
                    <span>{table.seats}人座</span>
                  </div>

                  {table.status === 'available' && (
                    <p className="text-green-600 text-xs">可用</p>
                  )}
                  {table.status === 'reserved' && (
                    <p className="text-yellow-600 text-xs">已预订</p>
                  )}
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
