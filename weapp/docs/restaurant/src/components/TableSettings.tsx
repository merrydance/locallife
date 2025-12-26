import { useState } from 'react';
import { Plus, QrCode, Edit2, Trash2, Download } from 'lucide-react';

interface TableConfig {
  id: string;
  number: string;
  type: 'table' | 'room';
  seats: number;
  qrCode: string;
  image?: string;
  minCharge?: number; // 包间最低消费
}

const initialTables: TableConfig[] = [
  { id: '1', number: '1号桌', type: 'table', seats: 2, qrCode: 'QR_TABLE_001' },
  { id: '2', number: '2号桌', type: 'table', seats: 2, qrCode: 'QR_TABLE_002' },
  { id: '3', number: '3号桌', type: 'table', seats: 4, qrCode: 'QR_TABLE_003' },
  { id: '4', number: '4号桌', type: 'table', seats: 4, qrCode: 'QR_TABLE_004' },
  { id: '5', number: '5号桌', type: 'table', seats: 4, qrCode: 'QR_TABLE_005' },
  { id: '6', number: '6号桌', type: 'table', seats: 6, qrCode: 'QR_TABLE_006' },
  { id: '7', number: '包间A', type: 'room', seats: 8, qrCode: 'QR_ROOM_A', minCharge: 500 },
  { id: '8', number: '包间B', type: 'room', seats: 10, qrCode: 'QR_ROOM_B', minCharge: 800 },
  { id: '9', number: '包间C', type: 'room', seats: 12, qrCode: 'QR_ROOM_C', minCharge: 1000 },
];

export function TableSettings() {
  const [tables, setTables] = useState<TableConfig[]>(initialTables);
  const [showQRModal, setShowQRModal] = useState(false);
  const [selectedTable, setSelectedTable] = useState<TableConfig | null>(null);
  const [filter, setFilter] = useState<'all' | 'table' | 'room'>('all');

  const filteredTables = filter === 'all' 
    ? tables 
    : tables.filter(t => t.type === filter);

  const handleDelete = (id: string) => {
    if (confirm('确定要删除这个桌台吗？')) {
      setTables(tables.filter(t => t.id !== id));
    }
  };

  const showQRCode = (table: TableConfig) => {
    setSelectedTable(table);
    setShowQRModal(true);
  };

  const downloadQRCode = (table: TableConfig) => {
    // 实际应用中这里会生成并下载真实的二维码图片
    alert(`下载二维码: ${table.number} (${table.qrCode})`);
  };

  const batchDownloadQR = () => {
    alert(`批量下载所有二维码 (${tables.length}个)`);
  };

  return (
    <div className="p-8">
      <div className="mb-6">
        <div className="flex justify-between items-center mb-6">
          <h2 className="text-gray-900">桌台设置</h2>
          <div className="flex gap-3">
            <button
              onClick={batchDownloadQR}
              className="flex items-center gap-2 px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700"
            >
              <Download className="w-5 h-5" />
              批量下载二维码
            </button>
            <button className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700">
              <Plus className="w-5 h-5" />
              添加桌台
            </button>
          </div>
        </div>

        <div className="flex gap-2">
          <button
            onClick={() => setFilter('all')}
            className={`px-4 py-2 rounded-lg ${filter === 'all' ? 'bg-blue-600 text-white' : 'bg-white text-gray-700 border border-gray-300'}`}
          >
            全部 ({tables.length})
          </button>
          <button
            onClick={() => setFilter('table')}
            className={`px-4 py-2 rounded-lg ${filter === 'table' ? 'bg-blue-600 text-white' : 'bg-white text-gray-700 border border-gray-300'}`}
          >
            普通桌台 ({tables.filter(t => t.type === 'table').length})
          </button>
          <button
            onClick={() => setFilter('room')}
            className={`px-4 py-2 rounded-lg ${filter === 'room' ? 'bg-blue-600 text-white' : 'bg-white text-gray-700 border border-gray-300'}`}
          >
            包间 ({tables.filter(t => t.type === 'room').length})
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
        {filteredTables.map((table) => (
          <div key={table.id} className="bg-white rounded-lg shadow p-6">
            <div className="flex justify-between items-start mb-4">
              <div>
                <h3 className="text-gray-900 mb-1">{table.number}</h3>
                <span className={`px-2 py-1 rounded text-xs ${
                  table.type === 'table' 
                    ? 'bg-blue-100 text-blue-700' 
                    : 'bg-purple-100 text-purple-700'
                }`}>
                  {table.type === 'table' ? '普通桌台' : '包间'}
                </span>
              </div>
            </div>

            {table.image ? (
              <div className="w-full h-32 bg-gray-200 rounded-lg mb-4 flex items-center justify-center">
                <img src={table.image} alt={table.number} className="w-full h-full object-cover rounded-lg" />
              </div>
            ) : (
              <div className="w-full h-32 bg-gray-100 rounded-lg mb-4 flex items-center justify-center text-gray-400">
                暂无图片
              </div>
            )}

            <div className="space-y-2 mb-4 text-gray-600">
              <div className="flex justify-between">
                <span>座位数</span>
                <span className="text-gray-900">{table.seats}人</span>
              </div>
              {table.minCharge && (
                <div className="flex justify-between">
                  <span>最低消费</span>
                  <span className="text-red-600">¥{table.minCharge}</span>
                </div>
              )}
              <div className="pt-2 border-t border-gray-200">
                <p className="text-xs text-gray-500">二维码: {table.qrCode}</p>
              </div>
            </div>

            <div className="grid grid-cols-2 gap-2">
              <button
                onClick={() => showQRCode(table)}
                className="px-3 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 flex items-center justify-center gap-1 text-xs"
              >
                <QrCode className="w-4 h-4" />
                查看二维码
              </button>
              <button
                onClick={() => downloadQRCode(table)}
                className="px-3 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 flex items-center justify-center gap-1 text-xs"
              >
                <Download className="w-4 h-4" />
                下载
              </button>
              <button className="px-3 py-2 bg-gray-100 text-gray-700 rounded-lg hover:bg-gray-200 flex items-center justify-center gap-1 text-xs">
                <Edit2 className="w-4 h-4" />
                编辑
              </button>
              <button
                onClick={() => handleDelete(table.id)}
                className="px-3 py-2 bg-red-50 text-red-600 rounded-lg hover:bg-red-100 flex items-center justify-center gap-1 text-xs"
              >
                <Trash2 className="w-4 h-4" />
                删除
              </button>
            </div>
          </div>
        ))}
      </div>

      {/* QR Code Modal */}
      {showQRModal && selectedTable && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-8 max-w-md w-full mx-4">
            <h3 className="text-gray-900 mb-4">{selectedTable.number} 二维码</h3>
            
            <div className="bg-white p-6 rounded-lg border-2 border-gray-200 mb-4">
              <div className="w-full aspect-square bg-gray-100 rounded-lg flex items-center justify-center mb-3">
                <div className="text-center">
                  <QrCode className="w-32 h-32 mx-auto text-gray-400 mb-2" />
                  <p className="text-gray-500">二维码占位图</p>
                  <p className="text-gray-400 text-xs mt-1">{selectedTable.qrCode}</p>
                </div>
              </div>
              <div className="text-center">
                <p className="text-gray-900">{selectedTable.number}</p>
                <p className="text-gray-500 text-xs">扫码点餐</p>
              </div>
            </div>

            <p className="text-gray-600 mb-4 text-xs">
              提示：将此二维码打印后贴在{selectedTable.number}上，顾客扫码即可进入点餐小程序
            </p>

            <div className="flex gap-3">
              <button
                onClick={() => {
                  setShowQRModal(false);
                  setSelectedTable(null);
                }}
                className="flex-1 px-4 py-2 bg-gray-200 text-gray-700 rounded-lg hover:bg-gray-300"
              >
                关闭
              </button>
              <button
                onClick={() => downloadQRCode(selectedTable)}
                className="flex-1 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
              >
                下载二维码
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
