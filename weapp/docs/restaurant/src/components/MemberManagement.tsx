import { useState } from 'react';
import { Search, Plus, CreditCard, TrendingUp } from 'lucide-react';

interface Member {
  id: string;
  name: string;
  phone: string;
  balance: number;
  points: number;
  level: 'bronze' | 'silver' | 'gold' | 'platinum';
  joinDate: string;
  totalSpent: number;
}

const initialMembers: Member[] = [
  { id: '1', name: '张三', phone: '138****1234', balance: 500, points: 1200, level: 'gold', joinDate: '2024-01-15', totalSpent: 5600 },
  { id: '2', name: '李四', phone: '139****5678', balance: 200, points: 450, level: 'silver', joinDate: '2024-03-20', totalSpent: 2100 },
  { id: '3', name: '王五', phone: '136****9012', balance: 1000, points: 2500, level: 'platinum', joinDate: '2023-11-05', totalSpent: 12000 },
  { id: '4', name: '赵六', phone: '137****3456', balance: 150, points: 200, level: 'bronze', joinDate: '2024-08-10', totalSpent: 800 },
];

export function MemberManagement() {
  const [members, setMembers] = useState<Member[]>(initialMembers);
  const [searchTerm, setSearchTerm] = useState('');
  const [showRechargeModal, setShowRechargeModal] = useState(false);
  const [selectedMember, setSelectedMember] = useState<Member | null>(null);
  const [rechargeAmount, setRechargeAmount] = useState('');

  const filteredMembers = members.filter(member =>
    member.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
    member.phone.includes(searchTerm)
  );

  const getLevelColor = (level: Member['level']) => {
    switch (level) {
      case 'bronze': return 'bg-orange-100 text-orange-700';
      case 'silver': return 'bg-gray-100 text-gray-700';
      case 'gold': return 'bg-yellow-100 text-yellow-700';
      case 'platinum': return 'bg-purple-100 text-purple-700';
    }
  };

  const getLevelText = (level: Member['level']) => {
    switch (level) {
      case 'bronze': return '铜卡会员';
      case 'silver': return '银卡会员';
      case 'gold': return '金卡会员';
      case 'platinum': return '白金会员';
    }
  };

  const handleRecharge = (member: Member) => {
    setSelectedMember(member);
    setShowRechargeModal(true);
  };

  const confirmRecharge = () => {
    if (selectedMember && rechargeAmount) {
      const amount = parseFloat(rechargeAmount);
      setMembers(members.map(m =>
        m.id === selectedMember.id
          ? { ...m, balance: m.balance + amount, points: m.points + Math.floor(amount) }
          : m
      ));
      setShowRechargeModal(false);
      setRechargeAmount('');
      setSelectedMember(null);
    }
  };

  const stats = {
    totalMembers: members.length,
    totalBalance: members.reduce((sum, m) => sum + m.balance, 0),
    totalPoints: members.reduce((sum, m) => sum + m.points, 0),
  };

  return (
    <div className="p-8">
      <h2 className="text-gray-900 mb-6">会员储值管理</h2>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
        <div className="bg-white rounded-lg shadow p-6">
          <p className="text-gray-500 mb-2">会员总数</p>
          <p className="text-gray-900">{stats.totalMembers}</p>
        </div>
        <div className="bg-white rounded-lg shadow p-6">
          <p className="text-gray-500 mb-2">储值总额</p>
          <p className="text-green-600">¥{stats.totalBalance.toLocaleString()}</p>
        </div>
        <div className="bg-white rounded-lg shadow p-6">
          <p className="text-gray-500 mb-2">积分总数</p>
          <p className="text-blue-600">{stats.totalPoints.toLocaleString()}</p>
        </div>
      </div>

      <div className="flex gap-4 mb-6">
        <div className="flex-1 relative">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-400 w-5 h-5" />
          <input
            type="text"
            placeholder="搜索会员姓名或手机号..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full pl-10 pr-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>
        <button className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700">
          <Plus className="w-5 h-5" />
          添加会员
        </button>
      </div>

      <div className="bg-white rounded-lg shadow">
        <table className="w-full">
          <thead className="bg-gray-50 border-b border-gray-200">
            <tr>
              <th className="px-6 py-3 text-left text-gray-600">会员信息</th>
              <th className="px-6 py-3 text-left text-gray-600">等级</th>
              <th className="px-6 py-3 text-left text-gray-600">余额</th>
              <th className="px-6 py-3 text-left text-gray-600">积分</th>
              <th className="px-6 py-3 text-left text-gray-600">累计消费</th>
              <th className="px-6 py-3 text-left text-gray-600">注册时间</th>
              <th className="px-6 py-3 text-left text-gray-600">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-200">
            {filteredMembers.map((member) => (
              <tr key={member.id} className="hover:bg-gray-50">
                <td className="px-6 py-4">
                  <div>
                    <p className="text-gray-900">{member.name}</p>
                    <p className="text-gray-500">{member.phone}</p>
                  </div>
                </td>
                <td className="px-6 py-4">
                  <span className={`px-2 py-1 rounded-full text-xs ${getLevelColor(member.level)}`}>
                    {getLevelText(member.level)}
                  </span>
                </td>
                <td className="px-6 py-4 text-green-600">¥{member.balance}</td>
                <td className="px-6 py-4 text-blue-600">{member.points}</td>
                <td className="px-6 py-4 text-gray-900">¥{member.totalSpent.toLocaleString()}</td>
                <td className="px-6 py-4 text-gray-600">{member.joinDate}</td>
                <td className="px-6 py-4">
                  <button
                    onClick={() => handleRecharge(member)}
                    className="flex items-center gap-1 px-3 py-1 bg-green-600 text-white rounded hover:bg-green-700 text-xs"
                  >
                    <CreditCard className="w-4 h-4" />
                    充值
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {showRechargeModal && selectedMember && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-8 max-w-md w-full mx-4">
            <h3 className="text-gray-900 mb-4">会员充值</h3>
            <div className="mb-4">
              <p className="text-gray-600 mb-2">会员: {selectedMember.name}</p>
              <p className="text-gray-600 mb-2">当前余额: ¥{selectedMember.balance}</p>
            </div>
            <div className="mb-6">
              <label className="block text-gray-700 mb-2">充值金额</label>
              <input
                type="number"
                value={rechargeAmount}
                onChange={(e) => setRechargeAmount(e.target.value)}
                placeholder="请输入充值金额"
                className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
              <div className="flex gap-2 mt-3">
                {[100, 200, 500, 1000].map(amount => (
                  <button
                    key={amount}
                    onClick={() => setRechargeAmount(amount.toString())}
                    className="px-3 py-1 bg-gray-100 text-gray-700 rounded hover:bg-gray-200 text-xs"
                  >
                    ¥{amount}
                  </button>
                ))}
              </div>
            </div>
            <div className="flex gap-3">
              <button
                onClick={() => {
                  setShowRechargeModal(false);
                  setRechargeAmount('');
                  setSelectedMember(null);
                }}
                className="flex-1 px-4 py-2 bg-gray-200 text-gray-700 rounded-lg hover:bg-gray-300"
              >
                取消
              </button>
              <button
                onClick={confirmRecharge}
                className="flex-1 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
              >
                确认充值
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
