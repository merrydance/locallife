import { useState } from 'react';
import { Save, Upload, Clock, Phone, MapPin, Mail } from 'lucide-react';

interface RestaurantInfo {
  name: string;
  description: string;
  phone: string;
  address: string;
  email: string;
  logo?: string;
  coverImage?: string;
  openTime: string;
  closeTime: string;
  serviceCharge: number;
  taxRate: number;
}

const initialInfo: RestaurantInfo = {
  name: '欢乐餐厅',
  description: '正宗川菜，用心服务每一位顾客',
  phone: '0571-12345678',
  address: '浙江省杭州市西湖区文一西路XXX号',
  email: 'contact@restaurant.com',
  coverImage: 'https://images.unsplash.com/photo-1649553413086-50f78d330a3b?crop=entropy&cs=tinysrgb&fit=max&fm=jpg&ixid=M3w3Nzg4Nzd8MHwxfHNlYXJjaHwxfHxyZXN0YXVyYW50JTIwZGluaW5nJTIwcm9vbXxlbnwxfHx8fDE3NjYxOTE5MTB8MA&ixlib=rb-4.1.0&q=80&w=1080',
  openTime: '10:00',
  closeTime: '22:00',
  serviceCharge: 0,
  taxRate: 6,
};

export function RestaurantSettings() {
  const [info, setInfo] = useState<RestaurantInfo>(initialInfo);
  const [isSaved, setIsSaved] = useState(false);

  const handleSave = () => {
    // 保存设置
    setIsSaved(true);
    setTimeout(() => setIsSaved(false), 2000);
  };

  const handleImageUpload = (type: 'logo' | 'cover') => {
    alert(`上传${type === 'logo' ? 'Logo' : '封面图'}`);
  };

  return (
    <div className="p-8 max-w-4xl">
      <div className="flex justify-between items-center mb-6">
        <h2 className="text-gray-900">餐厅设置</h2>
        <button
          onClick={handleSave}
          className={`flex items-center gap-2 px-4 py-2 rounded-lg transition-colors ${
            isSaved
              ? 'bg-green-600 text-white'
              : 'bg-blue-600 text-white hover:bg-blue-700'
          }`}
        >
          <Save className="w-5 h-5" />
          {isSaved ? '已保存' : '保存设置'}
        </button>
      </div>

      <div className="space-y-6">
        {/* 基本信息 */}
        <div className="bg-white rounded-lg shadow p-6">
          <h3 className="text-gray-900 mb-4">基本信息</h3>
          
          <div className="space-y-4">
            <div>
              <label className="block text-gray-700 mb-2">餐厅名称</label>
              <input
                type="text"
                value={info.name}
                onChange={(e) => setInfo({ ...info, name: e.target.value })}
                className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>

            <div>
              <label className="block text-gray-700 mb-2">餐厅简介</label>
              <textarea
                value={info.description}
                onChange={(e) => setInfo({ ...info, description: e.target.value })}
                rows={3}
                className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-gray-700 mb-2 flex items-center gap-2">
                  <Phone className="w-4 h-4" />
                  联系电话
                </label>
                <input
                  type="tel"
                  value={info.phone}
                  onChange={(e) => setInfo({ ...info, phone: e.target.value })}
                  className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>

              <div>
                <label className="block text-gray-700 mb-2 flex items-center gap-2">
                  <Mail className="w-4 h-4" />
                  电子邮箱
                </label>
                <input
                  type="email"
                  value={info.email}
                  onChange={(e) => setInfo({ ...info, email: e.target.value })}
                  className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>
            </div>

            <div>
              <label className="block text-gray-700 mb-2 flex items-center gap-2">
                <MapPin className="w-4 h-4" />
                餐厅地址
              </label>
              <input
                type="text"
                value={info.address}
                onChange={(e) => setInfo({ ...info, address: e.target.value })}
                className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
          </div>
        </div>

        {/* 营业时间 */}
        <div className="bg-white rounded-lg shadow p-6">
          <h3 className="text-gray-900 mb-4 flex items-center gap-2">
            <Clock className="w-5 h-5" />
            营业时间
          </h3>
          
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-gray-700 mb-2">营业开始</label>
              <input
                type="time"
                value={info.openTime}
                onChange={(e) => setInfo({ ...info, openTime: e.target.value })}
                className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>

            <div>
              <label className="block text-gray-700 mb-2">营业结束</label>
              <input
                type="time"
                value={info.closeTime}
                onChange={(e) => setInfo({ ...info, closeTime: e.target.value })}
                className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
          </div>
        </div>

        {/* 费用设置 */}
        <div className="bg-white rounded-lg shadow p-6">
          <h3 className="text-gray-900 mb-4">费用设置</h3>
          
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-gray-700 mb-2">服务费 (%)</label>
              <input
                type="number"
                value={info.serviceCharge}
                onChange={(e) => setInfo({ ...info, serviceCharge: parseFloat(e.target.value) })}
                min="0"
                max="100"
                step="0.1"
                className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
              <p className="text-gray-500 text-xs mt-1">设置为0表示不收取服务费</p>
            </div>

            <div>
              <label className="block text-gray-700 mb-2">税率 (%)</label>
              <input
                type="number"
                value={info.taxRate}
                onChange={(e) => setInfo({ ...info, taxRate: parseFloat(e.target.value) })}
                min="0"
                max="100"
                step="0.1"
                className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
          </div>
        </div>

        {/* 图片设置 */}
        <div className="bg-white rounded-lg shadow p-6">
          <h3 className="text-gray-900 mb-4">图片设置</h3>
          
          <div className="space-y-6">
            <div>
              <label className="block text-gray-700 mb-2">Logo</label>
              <div className="flex items-center gap-4">
                {info.logo ? (
                  <img src={info.logo} alt="Logo" className="w-32 h-32 object-cover rounded-lg border border-gray-200" />
                ) : (
                  <div className="w-32 h-32 bg-gray-100 rounded-lg flex items-center justify-center border border-gray-200">
                    <Upload className="w-8 h-8 text-gray-400" />
                  </div>
                )}
                <button
                  onClick={() => handleImageUpload('logo')}
                  className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                >
                  上传Logo
                </button>
              </div>
              <p className="text-gray-500 text-xs mt-2">建议尺寸: 200x200px, 格式: PNG/JPG</p>
            </div>

            <div>
              <label className="block text-gray-700 mb-2">封面图片</label>
              <div className="space-y-3">
                {info.coverImage ? (
                  <img src={info.coverImage} alt="Cover" className="w-full h-48 object-cover rounded-lg border border-gray-200" />
                ) : (
                  <div className="w-full h-48 bg-gray-100 rounded-lg flex items-center justify-center border border-gray-200">
                    <Upload className="w-8 h-8 text-gray-400" />
                  </div>
                )}
                <button
                  onClick={() => handleImageUpload('cover')}
                  className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                >
                  上传封面图
                </button>
              </div>
              <p className="text-gray-500 text-xs mt-2">建议尺寸: 1200x400px, 格式: PNG/JPG, 用于小程序首页展示</p>
            </div>
          </div>
        </div>

        {/* 小程序设置 */}
        <div className="bg-white rounded-lg shadow p-6">
          <h3 className="text-gray-900 mb-4">小程序设置</h3>
          
          <div className="space-y-4">
            <div className="flex items-center justify-between p-4 bg-blue-50 rounded-lg">
              <div>
                <p className="text-gray-900 mb-1">微信小程序</p>
                <p className="text-gray-600 text-xs">已配置并绑定</p>
              </div>
              <span className="px-3 py-1 bg-green-100 text-green-700 rounded-full text-xs">
                已启用
              </span>
            </div>

            <div className="p-4 bg-gray-50 rounded-lg">
              <p className="text-gray-700 mb-2">小程序AppID</p>
              <code className="text-gray-900 bg-white px-3 py-2 rounded border border-gray-200 block">
                wxabcdef1234567890
              </code>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
