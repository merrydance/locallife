import { useState, useEffect } from 'react';
import { Volume2, Printer, CheckCircle, Clock, ChefHat } from 'lucide-react';

interface KitchenOrderItem {
  id: string;
  name: string;
  quantity: number;
  notes?: string;
  status: 'pending' | 'preparing' | 'ready';
  startTime?: string;
}

interface KitchenOrder {
  id: string;
  tableNumber: string;
  type: 'dine-in' | 'takeout' | 'delivery' | 'reservation';
  items: KitchenOrderItem[];
  time: string;
  waitTime: number;
}

const initialKitchenOrders: KitchenOrder[] = [
  {
    id: 'ORD001',
    tableNumber: '3号桌',
    type: 'dine-in',
    items: [
      { id: 'item1', name: '宫保鸡丁', quantity: 2, notes: '少辣', status: 'preparing', startTime: '12:30' },
      { id: 'item2', name: '清蒸鲈鱼', quantity: 1, status: 'pending' },
    ],
    time: '12:30',
    waitTime: 15,
  },
  {
    id: 'ORD002',
    tableNumber: '外卖-张先生',
    type: 'delivery',
    items: [
      { id: 'item3', name: '红烧肉', quantity: 1, status: 'pending' },
      { id: 'item4', name: '麻婆豆腐', quantity: 2, notes: '多辣', status: 'pending' },
      { id: 'item5', name: '凉拌黄瓜', quantity: 1, status: 'ready' },
    ],
    time: '12:45',
    waitTime: 0,
  },
];

export function KitchenDisplay() {
  const [orders, setOrders] = useState<KitchenOrder[]>(initialKitchenOrders);
  const [soundEnabled, setSoundEnabled] = useState(true);

  useEffect(() => {
    const timer = setInterval(() => {
      setOrders(prevOrders =>
        prevOrders.map(order => ({
          ...order,
          waitTime: order.waitTime + 1,
        }))
      );
    }, 60000);

    return () => clearInterval(timer);
  }, []);

  const updateItemStatus = (orderId: string, itemId: string, status: KitchenOrderItem['status']) => {
    setOrders(orders.map(order => {
      if (order.id === orderId) {
        return {
          ...order,
          items: order.items.map(item =>
            item.id === itemId 
              ? { ...item, status, startTime: status === 'preparing' ? new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) : item.startTime }
              : item
          ),
        };
      }
      return order;
    }));
  };

  const updateAllItemsInOrder = (orderId: string, status: KitchenOrderItem['status']) => {
    setOrders(orders.map(order => {
      if (order.id === orderId) {
        return {
          ...order,
          items: order.items.map(item => ({
            ...item,
            status,
            startTime: status === 'preparing' ? new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) : item.startTime
          })),
        };
      }
      return order;
    }));
  };

  const printOrder = (id: string) => {
    alert(`打印订单: ${id}`);
  };

  const getOrderTypeColor = (type: KitchenOrder['type']) => {
    switch (type) {
      case 'dine-in': return 'bg-blue-100 text-blue-700';
      case 'delivery': return 'bg-green-100 text-green-700';
      case 'takeout': return 'bg-purple-100 text-purple-700';
      case 'reservation': return 'bg-orange-100 text-orange-700';
    }
  };

  const getOrderTypeText = (type: KitchenOrder['type']) => {
    switch (type) {
      case 'dine-in': return '堂食';
      case 'delivery': return '外卖';
      case 'takeout': return '外带';
      case 'reservation': return '预定';
    }
  };

  const getItemStatusColor = (status: KitchenOrderItem['status']) => {
    switch (status) {
      case 'pending': return 'bg-red-100 border-red-300';
      case 'preparing': return 'bg-blue-100 border-blue-300';
      case 'ready': return 'bg-green-100 border-green-300';
    }
  };

  const allItemsReady = (items: KitchenOrderItem[]) => {
    return items.every(item => item.status === 'ready');
  };

  return (
    <div className="p-8 bg-gray-100 min-h-screen">
      <div className="flex justify-between items-center mb-6">
        <h2 className="text-gray-900">厨房显示系统</h2>
        <div className="flex gap-4">
          <button
            onClick={() => setSoundEnabled(!soundEnabled)}
            className={`flex items-center gap-2 px-4 py-2 rounded-lg ${
              soundEnabled
                ? 'bg-blue-600 text-white'
                : 'bg-gray-200 text-gray-700'
            }`}
          >
            <Volume2 className="w-5 h-5" />
            语音播报 {soundEnabled ? '开' : '关'}
          </button>
          <div className="bg-white px-4 py-2 rounded-lg shadow">
            <span className="text-gray-600">待处理订单: </span>
            <span className="text-gray-900">{orders.length}</span>
          </div>
        </div>
      </div>

      <div className="space-y-6">
        {orders.map((order) => (
          <div key={order.id} className="bg-white rounded-lg shadow-lg p-6">
            <div className="flex justify-between items-start mb-4 pb-4 border-b border-gray-200">
              <div>
                <div className="flex items-center gap-3 mb-2">
                  <h3 className="text-gray-900">{order.tableNumber}</h3>
                  <span className={`px-3 py-1 rounded-full text-xs ${getOrderTypeColor(order.type)}`}>
                    {getOrderTypeText(order.type)}
                  </span>
                  <span className={`px-3 py-1 rounded-full text-xs ${
                    order.waitTime > 20 ? 'bg-red-100 text-red-700' : 'bg-yellow-100 text-yellow-700'
                  }`}>
                    等待 {order.waitTime} 分钟
                  </span>
                </div>
                <div className="flex items-center gap-4 text-gray-600">
                  <span>订单号: {order.id}</span>
                  <div className="flex items-center gap-1">
                    <Clock className="w-4 h-4" />
                    <span>{order.time}</span>
                  </div>
                </div>
              </div>
              <div className="flex gap-2">
                <button
                  onClick={() => updateAllItemsInOrder(order.id, 'preparing')}
                  className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 text-xs"
                >
                  全部开始制作
                </button>
                <button
                  onClick={() => printOrder(order.id)}
                  className="p-2 bg-gray-100 text-gray-700 rounded-lg hover:bg-gray-200"
                >
                  <Printer className="w-5 h-5" />
                </button>
              </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3 mb-4">
              {order.items.map((item) => (
                <div key={item.id} className={`p-4 rounded-lg border-2 ${getItemStatusColor(item.status)}`}>
                  <div className="flex justify-between items-start mb-3">
                    <div>
                      <h4 className="text-gray-900 mb-1">{item.name}</h4>
                      <span className="bg-gray-900 text-white px-3 py-1 rounded-full">x{item.quantity}</span>
                    </div>
                    <ChefHat className={`w-5 h-5 ${
                      item.status === 'ready' ? 'text-green-600' :
                      item.status === 'preparing' ? 'text-blue-600' : 'text-gray-400'
                    }`} />
                  </div>
                  
                  {item.notes && (
                    <p className="text-red-600 mb-3 text-xs">⚠️ {item.notes}</p>
                  )}
                  
                  {item.status === 'preparing' && item.startTime && (
                    <p className="text-blue-600 text-xs mb-3">开始时间: {item.startTime}</p>
                  )}

                  <div className="flex gap-2">
                    {item.status === 'pending' && (
                      <button
                        onClick={() => updateItemStatus(order.id, item.id, 'preparing')}
                        className="flex-1 px-3 py-1 bg-blue-600 text-white rounded hover:bg-blue-700 text-xs"
                      >
                        开始制作
                      </button>
                    )}
                    {item.status === 'preparing' && (
                      <button
                        onClick={() => updateItemStatus(order.id, item.id, 'ready')}
                        className="flex-1 px-3 py-1 bg-green-600 text-white rounded hover:bg-green-700 text-xs"
                      >
                        制作完成
                      </button>
                    )}
                    {item.status === 'ready' && (
                      <div className="flex-1 px-3 py-1 bg-green-600 text-white rounded text-center text-xs flex items-center justify-center gap-1">
                        <CheckCircle className="w-3 h-3" />
                        已完成
                      </div>
                    )}
                  </div>
                </div>
              ))}
            </div>

            {allItemsReady(order.items) && (
              <div className="bg-green-50 border border-green-200 rounded-lg p-4 flex items-center justify-between">
                <div className="flex items-center gap-2 text-green-700">
                  <CheckCircle className="w-5 h-5" />
                  <span>所有菜品已完成，可以出餐</span>
                </div>
                <button className="px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700">
                  确认出餐
                </button>
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}