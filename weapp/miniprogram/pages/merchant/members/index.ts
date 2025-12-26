/**
 * 会员管理页面
 * 基于 MerchantStatsService 实现会员数据展示与分析
 * 遵循 LDS 工作站布局规范
 */

import {
    MerchantStatsService,
    AnalyticsAdapter,
    type CustomerStatsResponse,
    type RepurchaseStatsResponse
} from '@/api/merchant-analytics';
import { responsiveBehavior } from '@/utils/responsive';

// 会员详情响应类型 (根据 swagger.json customerDetailResponse)
interface MemberDetailResponse {
    user_id: number;
    username: string;
    total_orders: number;
    total_spent: number;
    avg_order_value: number;
    last_order_date: string;
    favorite_dishes?: Array<{
        dish_id: number;
        dish_name: string;
        order_count: number;
    }>;
}

Page({
    behaviors: [responsiveBehavior],
    data: {
        // 会员数据
        members: [] as CustomerStatsResponse[],
        selectedMember: null as CustomerStatsResponse | null,
        memberDetail: null as MemberDetailResponse | null,

        // 统计数据
        stats: {
            total_customers: 0,
            repurchase_customers: 0,
            repurchase_rate: 0,
            avg_repurchase_interval: 0
        } as RepurchaseStatsResponse,

        // 分页
        page: 1,
        pageSize: 20,
        hasMore: true,

        // 搜索
        searchKeyword: '',

        // 日期范围 (默认最近90天)
        dateRange: {
            start_date: '',
            end_date: ''
        },

        // 界面状态
        loading: true
    },

    onLoad() {
        this.initPage();
    },

    onShow() {
        // 如果已经加载过，刷新数据
        if (this.data.members.length > 0) {
            this.loadMembers(true);
        }
    },

    /**
     * 初始化页面
     */
    async initPage() {
        // 设置默认日期范围（最近90天）
        const endDate = new Date();
        const startDate = new Date();
        startDate.setDate(startDate.getDate() - 90);

        this.setData({
            dateRange: {
                start_date: this.formatDate(startDate),
                end_date: this.formatDate(endDate)
            }
        });

        try {
            this.setData({ loading: true });
            await Promise.all([
                this.loadStats(),
                this.loadMembers(true)
            ]);
        } catch (error: any) {
            console.error('初始化页面失败:', error);
            wx.showToast({
                title: error.message || '加载失败',
                icon: 'error'
            });
        } finally {
            this.setData({ loading: false });
        }
    },

    /**
     * 加载统计数据
     */
    async loadStats() {
        try {
            const { dateRange } = this.data;
            const stats = await MerchantStatsService.getRepurchaseStats(dateRange);
            this.setData({ stats });
        } catch (error: any) {
            console.error('加载统计数据失败:', error);
        }
    },

    /**
     * 加载会员列表
     */
    async loadMembers(reset: boolean = false) {
        try {
            const { dateRange, page, pageSize } = this.data;

            if (reset) {
                this.setData({ page: 1, members: [], hasMore: true });
            }

            const result = await MerchantStatsService.getCustomerStats({
                ...dateRange,
                page_id: reset ? 1 : page,
                page_size: pageSize
            });

            const newMembers = reset ? result : [...this.data.members, ...result];

            this.setData({
                members: newMembers,
                hasMore: result.length === pageSize,
                page: reset ? 2 : page + 1
            });

        } catch (error: any) {
            console.error('加载会员列表失败:', error);
            wx.showToast({
                title: '加载会员失败',
                icon: 'error'
            });
        }
    },

    /**
     * 选择会员
     */
    onSelectMember(e: any) {
        const member = e.currentTarget.dataset.item as CustomerStatsResponse;
        this.setData({
            selectedMember: member,
            memberDetail: null // 清空详情，触发重新加载
        });

        // 加载会员详情
        this.loadMemberDetail(member.user_id);
    },

    /**
     * 加载会员详情
     */
    async loadMemberDetail(userId: number) {
        try {
            // 注意：这个 API 可能需要在 merchant-analytics.ts 中添加
            // 目前使用已有的客户统计数据作为详情
            // 真实场景应调用 GET /v1/merchant/stats/customers/{user_id}

            // 暂时使用选中的会员数据作为详情
            const { selectedMember } = this.data;
            if (selectedMember) {
                this.setData({
                    memberDetail: {
                        user_id: selectedMember.user_id,
                        username: selectedMember.username,
                        total_orders: selectedMember.total_orders,
                        total_spent: selectedMember.total_spent,
                        avg_order_value: selectedMember.avg_order_value,
                        last_order_date: selectedMember.last_order_date,
                        favorite_dishes: [] // TODO: 从详情 API 获取
                    }
                });
            }
        } catch (error: any) {
            console.error('加载会员详情失败:', error);
        }
    },

    /**
     * 搜索会员
     */
    onSearch(e: any) {
        const keyword = e.detail.value;
        this.setData({ searchKeyword: keyword });

        // TODO: 后端搜索支持，目前前端过滤
        // 实际应用中应调用带 keyword 参数的 API
    },

    /**
     * 上一页
     */
    onPrevPage() {
        const { page } = this.data;
        if (page > 2) {
            this.setData({ page: page - 2 });
            this.loadMembers(false);
        }
    },

    /**
     * 下一页
     */
    onNextPage() {
        const { hasMore } = this.data;
        if (hasMore) {
            this.loadMembers(false);
        }
    },

    /**
     * 返回工作台
     */
    onBack() {
        wx.navigateBack({
            fail: () => {
                wx.redirectTo({ url: '/pages/merchant/dashboard/index' });
            }
        });
    },

    // ==================== 工具方法 ====================

    /**
     * 格式化日期
     */
    formatDate(date: Date): string {
        const year = date.getFullYear();
        const month = ('0' + (date.getMonth() + 1)).slice(-2);
        const day = ('0' + date.getDate()).slice(-2);
        return `${year}-${month}-${day}`;
    },

    /**
     * 格式化金额
     */
    formatAmount(amount: number): string {
        return AnalyticsAdapter.formatAmount(amount);
    },

    /**
     * 格式化百分比
     */
    formatPercentage(value: number): string {
        return AnalyticsAdapter.formatPercentage(value);
    }
});
