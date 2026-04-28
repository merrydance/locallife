import { RiderInfo, RiderStatus } from '../../../api/rider'
import { RecommendedOrder } from '../../../api/delivery'
import {
  DashboardDeliveryView,
  TagTheme,
  WsUnsubscribe,
  riderDashboardRuntimeMethods
} from '../../../utils/rider-dashboard-runtime'
import { RiderWorkbenchDashboardView } from '../../../services/rider-workbench'
import { resolveStatusTagTheme } from '../../../utils/status-tag'

const emptyWorkbenchView: RiderWorkbenchDashboardView = {
  riderStatus: {
    status: '',
    is_online: false,
    online_status: 'offline',
    active_deliveries: 0,
    can_go_online: false,
    can_go_offline: false
  },
  metrics: [],
  risks: [],
  currentDelivery: null,
  availableOrderCount: 0,
  activeDeliveryCount: 0,
  todayCompletedDeliveries: 0,
  todayIncomeDisplay: '0.00',
  unavailableText: ''
}

Page({
  data: {
    navBarHeight: 88,
    activeTab: 'hall',
    loading: false,
    onlineSwitchLoading: false,
    grabActionLoading: false,
    deliveryActionLoadingIds: [] as number[],
    locationActionLoading: false,
    initError: '',
    initErrorCanRetry: true,
    workbenchRefreshError: '',
    dashboardInlineError: '',
    hallLoadError: '',
    myLoadError: '',
    isRefresherTriggered: false,
    statusText: '休息中 - 点击上线',
    showDepositReminder: false,
    depositReminderText: '',
    onlineStatusLabel: '休息中',
    onlineStatusTheme: 'default',

    // 骑手基础信息
    riderInfo: null as RiderInfo | null,
    riderStatus: null as RiderStatus | null,
    isOnline: false,
    workbench: emptyWorkbenchView,

    recommendOrders: [] as RecommendedOrder[],
    activeDeliveries: [] as DashboardDeliveryView[],
    currentDelivery: null as DashboardDeliveryView | null,
    hallTabLabel: '抢单大厅 0',
    myTabLabel: '我的任务 0',
    stats: {
      todayCount: 0,
      todayEarnings: 0,
      creditScore: 0
    },

    locationDeliveryId: 0,
    locationStatusText: '',
    locationStatusTheme: resolveStatusTagTheme('neutral') as TagTheme,
    locationPendingText: '',
    locationUpdatedText: '',
    locationNeedsPermission: false,

    newOrdersCount: 0,
    _wsListeners: [] as WsUnsubscribe[]
  },

  ...riderDashboardRuntimeMethods
})
