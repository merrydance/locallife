import { RiderInfo, RiderStatus } from '../../../api/rider'
import { RecommendedOrder } from '../../../api/delivery'
import {
  DashboardDeliveryView,
  TagTheme,
  WsUnsubscribe,
  riderDashboardRuntimeMethods
} from '../../../utils/rider-dashboard-runtime'

Page({
  data: {
    navBarHeight: 88,
    activeTab: 'hall',
    loading: false,
    onlineSwitchLoading: false,
    initError: '',
    initErrorCanRetry: true,
    hallLoadError: '',
    myLoadError: '',
    isRefresherTriggered: false,
    statusText: '休息中 - 点击上线',
    showDepositReminder: false,
    depositReminderText: '',

    // 骑手基础信息
    riderInfo: null as RiderInfo | null,
    riderStatus: null as RiderStatus | null,
    isOnline: false,

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
    locationStatusTheme: 'default' as TagTheme,
    locationPendingText: '',
    locationUpdatedText: '',
    locationNeedsPermission: false,

    newOrdersCount: 0,
    _wsListeners: [] as WsUnsubscribe[]
  },

  ...riderDashboardRuntimeMethods
})
