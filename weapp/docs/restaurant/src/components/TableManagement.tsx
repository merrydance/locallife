import { useState } from 'react';
import { Users, Clock } from 'lucide-react';

interface Table {
  id: string;
  number: string;
  seats: number;
  status: 'available' | 'occupied' | 'reserved';
  currentOrder?: string;
  startTime?: string;
  duration?: number;
}

const initialTables: Table[] = [
  { id: '1', number: '1号桌', seats: 2, status: 'available' },
  { id: '2', number: '2号桌', seats: 2, status: 'available' },
  { id: '3', number: '3号桌', seats: 4, status: 'occupied', currentOrder: 'ORD001', startTime: '12:30', duration: 45 },
  { id: '4', number: '4号桌', seats: 4, status: 'reserved' },
  { id: '5', number: '5号桌', seats: 4, status: 'occupied', currentOrder: 'ORD002', startTime: '12:45', duration: 30 },
  { id: '6', number: '6号桌', seats: 6, status: 'available' },
  { id: '7', number: '7号桌', seats: 6, status: 'available' },
  { id: '8', number: '8号桌', seats: 8, status: 'occupied', currentOrder: 'ORD003', startTime: '12:15', duration: 60 },
  { id: '9', number: '9号桌', seats: 8, status: 'available' },
  { id: '10', number: '10号桌', seats: 10, status: 'reserved' },
];

export function TableManagement() {
  const [tables, setTables] = useState<Table[]>(initialTables);
  const [filter, setFilter] = useState<'all' | Table['status']>('all');

  const filteredTables = filter === 'all'
    ? tables
    : tables.filter(table => table.status === filter);

  const getStatusColor = (status: Table['status']) => {
    switch (status) {
      case 'available': return 'bg-green-500';
      case 'occupied': return 'bg-red-500';
      case 'reserved': return 'bg-yellow-500';
    }
  };

  const getStatusText = (status: Table['status']) => {
    switch (status) {
      case 'available': return '空闲';
      case 'occupied': return '使用中';
      case 'reserved': return '已预订';
    }
  };

  const changeTableStatus = (id: string, status: Table['status']) => {
    setTables(tables.map(table =>
      table.id === id ? { ...table, status } : table
    ));
  };

  const stats = {
    total: tables.length,
    available: tables.filter(t => t.status === 'available').length,
    occupied: tables.filter(t => t.status === 'occupied').length,
    reserved: tables.filter(t => t.status === 'reserved').length,
  };

  return (
    <div className="p-8">
      <h2 className="text-gray-900 mb-6">桌台管理</h2>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
        <div className="bg-white rounded-lg shadow p-6">
          <p className="text-gray-500 mb-2">总桌台数</p>
          <p className="text-gray-900">{stats.total}</p>
        </div>
        <div className="bg-white rounded-lg shadow p-6">
          <p className="text-gray-500 mb-2">空闲桌台</p>
          <p className="text-green-600">{stats.available}</p>
        </div>
        <div className="bg-white rounded-lg shadow p-6">
          <p className="text-gray-500 mb-2">使用中</p>
          <p className="text-red-600">{stats.occupied}</p>
        </div>
        <div className="bg-white rounded-lg shadow p-6">
          <p className="text-gray-500 mb-2">已预订</p>
          <p className="text-yellow-600">{stats.reserved}</p>
        </div>
      </div>

      <div className="flex gap-2 mb-6">
        <button
          onClick={() => setFilter('all')}
          className={`px-4 py-2 rounded-lg ${filter === 'all' ? 'bg-blue-600 text-white' : 'bg-white text-gray-700 border border-gray-300'}`}
        >
          全部
        </button>
        <button
          onClick={() => setFilter('available')}
          className={`px-4 py-2 rounded-lg ${filter === 'available' ? 'bg-blue-600 text-white' : 'bg-white text-gray-700 border border-gray-300'}`}
        >
          空闲
        </button>
        <button
          onClick={() => setFilter('occupied')}
          className={`px-4 py-2 rounded-lg ${filter === 'occupied' ? 'bg-blue-600 text-white' : 'bg-white text-gray-700 border border-gray-300'}`}
        >
          使用中
        </button>
        <button
          onClick={() => setFilter('reserved')}
          className={`px-4 py-2 rounded-lg ${filter === 'reserved' ? 'bg-blue-600 text-white' : 'bg-white text-gray-700 border border-gray-300'}`}
        >
          已预订
        </button>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-4">
        {filteredTables.map((table) => (
          <div
            key={table.id}
            className="bg-white rounded-lg shadow p-6 relative"
          >
            <div className={`absolute top-3 right-3 w-3 h-3 rounded-full ${getStatusColor(table.status)}`} />
            
            <h3 className="text-gray-900 mb-2">{table.number}</h3>
            
            <div className="flex items-center gap-2 text-gray-600 mb-3">
              <Users className="w-4 h-4" />
              <span>{table.seats}人座</span>
            </div>

            <p className={`mb-4 text-xs ${
              table.status === 'available' ? 'text-green-600' :
              table.status === 'occupied' ? 'text-red-600' :
              'text-yellow-600'
            }`}>
              {getStatusText(table.status)}
            </p>

            {table.status === 'occupied' && (
              <div className="mb-4 pb-4 border-b border-gray-200">
                <p className="text-gray-500 mb-1">订单: {table.currentOrder}</p>
                <div className="flex items-center gap-1 text-gray-600">
                  <Clock className="w-3 h-3" />
                  <span className="text-xs">{table.startTime} ({table.duration}分钟)</span>
                </div>
              </div>
            )}

            <div className="space-y-2">
              {table.status === 'available' && (
                <button
                  onClick={() => changeTableStatus(table.id, 'occupied')}
                  className="w-full px-3 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 text-xs"
                >
                  开台
                </button>
              )}
              {table.status === 'occupied' && (
                <button
                  onClick={() => changeTableStatus(table.id, 'available')}
                  className="w-full px-3 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 text-xs"
                >
                  结账
                </button>
              )}
              {table.status === 'reserved' && (
                <button
                  onClick={() => changeTableStatus(table.id, 'available')}
                  className="w-full px-3 py-2 bg-yellow-600 text-white rounded-lg hover:bg-yellow-700 text-xs"
                >
                  取消预订
                </button>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
