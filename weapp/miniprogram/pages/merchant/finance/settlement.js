"use strict";
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const responsive_1 = require("../../../utils/responsive");
const logger_1 = require("../../../utils/logger");
const error_handler_1 = require("../../../utils/error-handler");
const finance_analytics_1 = require("../../../api/finance-analytics");
const dayjs_1 = __importDefault(require("dayjs"));
const app = getApp();
Page({
    behaviors: [responsive_1.responsiveBehavior],
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
        settlementList: [],
        selectedSettlement: null
    },
    onLoad() {
        this.initData();
    },
    initData() {
        return __awaiter(this, void 0, void 0, function* () {
            const merchantId = app.globalData.merchantId;
            if (merchantId) {
                this.setData({ merchantId: Number(merchantId) });
                this.loadFinanceData();
            }
            else {
                app.userInfoReadyCallback = () => {
                    if (app.globalData.merchantId) {
                        this.setData({ merchantId: Number(app.globalData.merchantId) });
                        this.loadFinanceData();
                    }
                };
            }
        });
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.height || 88 });
    },
    loadFinanceData() {
        return __awaiter(this, void 0, void 0, function* () {
            if (!this.data.merchantId)
                return;
            this.setData({ loading: true });
            const endDate = (0, dayjs_1.default)().format('YYYY-MM-DD');
            const startDate = (0, dayjs_1.default)().subtract(30, 'day').format('YYYY-MM-DD');
            try {
                const [overview, settlements] = yield Promise.all([
                    finance_analytics_1.financeManagementService.getFinanceOverview({ start_date: startDate, end_date: endDate }),
                    finance_analytics_1.financeManagementService.getSettlements({ start_date: startDate, end_date: endDate, page: 1, limit: 50 })
                ]);
                // 适配统计输出
                const adaptedStats = {
                    total_balance: (overview.net_income / 100).toFixed(2),
                    pending_settle: (overview.pending_income / 100).toFixed(2),
                    today_gmv: (overview.total_gmv / 100).toFixed(2)
                };
                // 适配结算列表 (后端返回通常是 object { items: [], total: 0 })
                const list = Array.isArray(settlements) ? settlements : (settlements.items || []);
                const processedList = list.map((item) => (Object.assign(Object.assign({}, item), { amount: (item.amount / 100).toFixed(2), date: (0, dayjs_1.default)(item.created_at || item.date).format('YYYY-MM-DD') })));
                this.setData({
                    stats: adaptedStats,
                    settlementList: processedList,
                    loading: false
                });
                // PC端选中首项
                // @ts-ignore
                if (this.data.deviceType !== 'mobile' && processedList.length > 0 && !this.data.selectedSettlement) {
                    this.setData({ selectedSettlement: processedList[0] });
                }
            }
            catch (error) {
                logger_1.logger.error('加载财务数据失败', error, 'Finance');
                error_handler_1.ErrorHandler.handle(error, 'LoadFinanceData');
                this.setData({ loading: false });
            }
        });
    },
    onTabChange(e) {
        this.setData({ activeTab: e.detail.value });
        // 如果有不同Tab的数据加载逻辑可以在此扩展
    },
    onSelectSettlement(e) {
        this.setData({ selectedSettlement: e.currentTarget.dataset.item });
    }
});
