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
    current_region_id: 0,
    required_deposit: 0,
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
    showSettlementReminder: false,
    settlementReminderText: '',
    showDepositReminder: false,
    depositReminderText: '',
    onlineStatusLabel: '休息中',
    onlineStatusTheme: 'default',

    riderInfo: null as RiderInfo | null,
    riderStatus: null as RiderStatus | null,
    isOnline: false,
    workbench: emptyWorkbenchView,

    recommendOrders: [] as RecommendedOrder[],
    activeDeliveries: [] as DashboardDeliveryView[],
    currentDelivery: null as DashboardDeliveryView | null,
    hallTabLabel: '接单大厅 0',
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
