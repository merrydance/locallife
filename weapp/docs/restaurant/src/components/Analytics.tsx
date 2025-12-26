import { useState } from 'react';
import { TrendingUp, TrendingDown, DollarSign, ShoppingCart, Users, Package } from 'lucide-react';
import { LineChart, Line, BarChart, Bar, PieChart, Pie, Cell, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts';

const salesData = [
  { date: '12-14', revenue: 4500, orders: 32 },
  { date: '12-15', revenue: 5200, orders: 38 },
  { date: '12-16', revenue: 4800, orders: 35 },
  { date: '12-17', revenue: 6100, orders: 45 },
  { date: '12-18', revenue: 5800, orders: 42 },
  { date: '12-19', revenue: 7200, orders: 52 },
  { date: '12-20', revenue: 6800, orders: 48 },
];

const dishSalesData = [
  { name: '宫保鸡丁', sales: 156 },
  { name: '红烧肉', sales: 132 },
  { name: '清蒸鲈鱼', sales: 98 },
  { name: '麻婆豆腐', sales: 165 },
  { name: '凉拌黄瓜', sales: 201 },
];

const categoryData = [
  { name: '热菜', value: 35, color: '#ef4444' },
  { name: '凉菜', value: 20, color: '#3b82f6' },
  { name: '海鲜', value: 25, color: '#10b981' },
  { name: '主食', value: 15, color: '#f59e0b' },
  { name: '饮料', value: 5, color: '#8b5cf6' },
];

const timeData = [
  { hour: '11:00', orders: 5 },
  { hour: '12:00', orders: 18 },
  { hour: '13:00', orders: 15 },
  { hour: '17:00', orders: 8 },
  { hour: '18:00', orders: 22 },
  { hour: '19:00', orders: 25 },
  { hour: '20:00', orders: 18 },
];

export function Analytics() {
  const [dateRange, setDateRange] = useState('week');

  return (
    <div className="p-8">
      <div className="flex justify-between items-center mb-6">
        <h2 className="text-gray-900">经营分析</h2>
        <div className="flex gap-2">
          <button
            onClick={() => setDateRange('day')}
            className={`px-4 py-2 rounded-lg ${dateRange === 'day' ? 'bg-blue-600 text-white' : 'bg-white text-gray-700 border border-gray-300'}`}
          >
            今日
          </button>
          <button
            onClick={() => setDateRange('week')}
            className={`px-4 py-2 rounded-lg ${dateRange === 'week' ? 'bg-blue-600 text-white' : 'bg-white text-gray-700 border border-gray-300'}`}
          >
            本周
          </button>
          <button
            onClick={() => setDateRange('month')}
            className={`px-4 py-2 rounded-lg ${dateRange === 'month' ? 'bg-blue-600 text-white' : 'bg-white text-gray-700 border border-gray-300'}`}
          >
            本月
          </button>
        </div>
      </div>

      {/* KPI Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <div className="bg-white rounded-lg shadow p-6">
          <div className="flex items-center justify-between mb-2">
            <p className="text-gray-500">总营业额</p>
            <DollarSign className="w-5 h-5 text-green-600" />
          </div>
          <p className="text-gray-900 mb-1">¥40,400</p>
          <div className="flex items-center gap-1 text-green-600">
            <TrendingUp className="w-4 h-4" />
            <span className="text-xs">+12.5%</span>
          </div>
        </div>

        <div className="bg-white rounded-lg shadow p-6">
          <div className="flex items-center justify-between mb-2">
            <p className="text-gray-500">订单数量</p>
            <ShoppingCart className="w-5 h-5 text-blue-600" />
          </div>
          <p className="text-gray-900 mb-1">292</p>
          <div className="flex items-center gap-1 text-green-600">
            <TrendingUp className="w-4 h-4" />
            <span className="text-xs">+8.2%</span>
          </div>
        </div>

        <div className="bg-white rounded-lg shadow p-6">
          <div className="flex items-center justify-between mb-2">
            <p className="text-gray-500">客单价</p>
            <Package className="w-5 h-5 text-purple-600" />
          </div>
          <p className="text-gray-900 mb-1">¥138</p>
          <div className="flex items-center gap-1 text-green-600">
            <TrendingUp className="w-4 h-4" />
            <span className="text-xs">+3.8%</span>
          </div>
        </div>

        <div className="bg-white rounded-lg shadow p-6">
          <div className="flex items-center justify-between mb-2">
            <p className="text-gray-500">新增会员</p>
            <Users className="w-5 h-5 text-orange-600" />
          </div>
          <p className="text-gray-900 mb-1">28</p>
          <div className="flex items-center gap-1 text-red-600">
            <TrendingDown className="w-4 h-4" />
            <span className="text-xs">-5.2%</span>
          </div>
        </div>
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-6">
        {/* Revenue Trend */}
        <div className="bg-white rounded-lg shadow p-6">
          <h3 className="text-gray-900 mb-4">营业额趋势</h3>
          <ResponsiveContainer width="100%" height={250}>
            <LineChart data={salesData}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="date" />
              <YAxis />
              <Tooltip />
              <Legend />
              <Line type="monotone" dataKey="revenue" stroke="#3b82f6" strokeWidth={2} name="营业额 (¥)" />
            </LineChart>
          </ResponsiveContainer>
        </div>

        {/* Order Volume */}
        <div className="bg-white rounded-lg shadow p-6">
          <h3 className="text-gray-900 mb-4">订单量趋势</h3>
          <ResponsiveContainer width="100%" height={250}>
            <LineChart data={salesData}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="date" />
              <YAxis />
              <Tooltip />
              <Legend />
              <Line type="monotone" dataKey="orders" stroke="#10b981" strokeWidth={2} name="订单数" />
            </LineChart>
          </ResponsiveContainer>
        </div>

        {/* Top Dishes */}
        <div className="bg-white rounded-lg shadow p-6">
          <h3 className="text-gray-900 mb-4">热销菜品排行</h3>
          <ResponsiveContainer width="100%" height={250}>
            <BarChart data={dishSalesData}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="name" />
              <YAxis />
              <Tooltip />
              <Bar dataKey="sales" fill="#3b82f6" name="销量" />
            </BarChart>
          </ResponsiveContainer>
        </div>

        {/* Category Distribution */}
        <div className="bg-white rounded-lg shadow p-6">
          <h3 className="text-gray-900 mb-4">品类占比</h3>
          <ResponsiveContainer width="100%" height={250}>
            <PieChart>
              <Pie
                data={categoryData}
                cx="50%"
                cy="50%"
                labelLine={false}
                label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}
                outerRadius={80}
                fill="#8884d8"
                dataKey="value"
              >
                {categoryData.map((entry, index) => (
                  <Cell key={`cell-${index}`} fill={entry.color} />
                ))}
              </Pie>
              <Tooltip />
            </PieChart>
          </ResponsiveContainer>
        </div>

        {/* Peak Hours */}
        <div className="bg-white rounded-lg shadow p-6 lg:col-span-2">
          <h3 className="text-gray-900 mb-4">高峰时段分析</h3>
          <ResponsiveContainer width="100%" height={250}>
            <BarChart data={timeData}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="hour" />
              <YAxis />
              <Tooltip />
              <Bar dataKey="orders" fill="#f59e0b" name="订单数" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Summary Table */}
      <div className="bg-white rounded-lg shadow p-6">
        <h3 className="text-gray-900 mb-4">每日汇总</h3>
        <table className="w-full">
          <thead className="bg-gray-50 border-b border-gray-200">
            <tr>
              <th className="px-6 py-3 text-left text-gray-600">日期</th>
              <th className="px-6 py-3 text-left text-gray-600">营业额</th>
              <th className="px-6 py-3 text-left text-gray-600">订单数</th>
              <th className="px-6 py-3 text-left text-gray-600">客单价</th>
              <th className="px-6 py-3 text-left text-gray-600">增长率</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-200">
            {salesData.map((day, index) => (
              <tr key={index} className="hover:bg-gray-50">
                <td className="px-6 py-4 text-gray-900">{day.date}</td>
                <td className="px-6 py-4 text-gray-900">¥{day.revenue.toLocaleString()}</td>
                <td className="px-6 py-4 text-gray-600">{day.orders}</td>
                <td className="px-6 py-4 text-gray-900">¥{Math.round(day.revenue / day.orders)}</td>
                <td className="px-6 py-4">
                  <span className={index > 0 && day.revenue > salesData[index - 1].revenue ? 'text-green-600' : 'text-red-600'}>
                    {index > 0 ? `${((day.revenue - salesData[index - 1].revenue) / salesData[index - 1].revenue * 100).toFixed(1)}%` : '-'}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
