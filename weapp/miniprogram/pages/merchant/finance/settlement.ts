import { responsiveBehavior } from '../../../utils/responsive'
import { logger } from '../../../utils/logger'
import { ErrorHandler } from '../../../utils/error-handler'
import { financeManagementService, FinanceAnalyticsAdapter } from '../../../api/finance-analytics'
import dayjs from 'dayjs'

const app = getApp<IAppOption>()

Page({
    behaviors: [responsiveBehavior],
    data: {
        navBarHeight: 88,
        activeTab: 'daily',
        loading: false,
        merchantId: 0,
        stats: {
            total_balance: '0.00',
            pending_settle: '0.00',
            today_gmv: '0.00'
        },
        settlementList: [] as any[],
        selectedSettlement: null as any
    },

    onLoad() {
        this.initData()
    },

    async initData() {
        const merchantId = app.globalData.merchantId;
        if (merchantId) {
            this.setData({ merchantId: Number(merchantId) })
            this.loadFinanceData()
        } else {
            app.userInfoReadyCallback = () => {
                if (app.globalData.merchantId) {
                    this.setData({ merchantId: Number(app.globalData.merchantId) })
                    this.loadFinanceData()
                }
            }
        }
    },

    onNavHeight(e: any) {
        this.setData({ navBarHeight: e.detail.height || 88 })
    },

    async loadFinanceData() {
        if (!this.data.merchantId) return
        this.setData({ loading: true })

        const endDate = dayjs().format('YYYY-MM-DD')
        const startDate = dayjs().subtract(30, 'day').format('YYYY-MM-DD')

        try {
            const [overview, settlements] = await Promise.all([
                financeManagementService.getFinanceOverview({ start_date: startDate, end_date: endDate }),
                financeManagementService.getSettlements({ start_date: startDate, end_date: endDate, page: 1, limit: 50 })
            ])

            // 适配统计输出
            const adaptedStats = {
                total_balance: (overview.net_income / 100).toFixed(2),
                pending_settle: (overview.pending_income / 100).toFixed(2),
                today_gmv: (overview.total_gmv / 100).toFixed(2)
            }

            // 适配结算列表 (后端返回通常是 object { items: [], total: 0 })
            const list = Array.isArray(settlements) ? settlements : (settlements.items || [])
            const processedList = list.map((item: any) => ({
                ...item,
                amount: (item.amount / 100).toFixed(2),
                date: dayjs(item.created_at || item.date).format('YYYY-MM-DD')
            }))

            this.setData({
                stats: adaptedStats,
                settlementList: processedList,
                loading: false
            })

            // PC端选中首项
            // @ts-ignore
            if (this.data.deviceType !== 'mobile' && processedList.length > 0 && !this.data.selectedSettlement) {
                this.setData({ selectedSettlement: processedList[0] })
            }

        } catch (error) {
            logger.error('加载财务数据失败', error, 'Finance')
            ErrorHandler.handle(error, 'LoadFinanceData')
            this.setData({ loading: false })
        }
    },

    onTabChange(e: any) {
        this.setData({ activeTab: e.detail.value })
        // 如果有不同Tab的数据加载逻辑可以在此扩展
    },

    onSelectSettlement(e: any) {
        this.setData({ selectedSettlement: e.currentTarget.dataset.item })
    }
})
