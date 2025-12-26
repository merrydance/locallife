/**
 * 扫码点餐入口页面
 * 处理微信扫一扫跳转到小程序的场景
 */

import { scanTableQRCode } from '../../../api/customer-basic';
import { getTableInfo } from '../../../api/customer-reservation';

interface TableInfo {
    id: number;
    table_number: string;
    merchant_id: number;
    merchant_name: string;
    merchant_logo: string;
    capacity: number;
    status: 'available' | 'occupied' | 'reserved';
    qr_code_data: string;
}

Page({
    data: {
        loading: true,
        tableInfo: null as TableInfo | null,
        error: null as string | null,
        merchantInfo: null as any
    },

    onLoad(options: any) {
        console.log('扫码点餐页面加载，参数:', options);

        // 从扫码参数中获取桌台信息
        const { scene, q } = options;

        if (scene) {
            // 通过scene参数获取桌台ID（小程序码场景）
            this.handleSceneParam(scene);
        } else if (q) {
            // 通过q参数获取完整URL（二维码场景）
            this.handleQRCodeParam(decodeURIComponent(q));
        } else {
            // 直接传入table_id参数（测试场景）
            const tableId = options.table_id;
            if (tableId) {
                this.loadTableInfo(parseInt(tableId));
            } else {
                this.setData({
                    loading: false,
                    error: '无效的扫码参数'
                });
            }
        }
    },

    /**
     * 处理小程序码scene参数
     */
    handleSceneParam(scene: string) {
        try {
            // scene格式: table_123 或 t123
            const tableId = scene.replace(/^(table_|t)/, '');
            if (tableId && !isNaN(parseInt(tableId))) {
                this.loadTableInfo(parseInt(tableId));
            } else {
                throw new Error('无效的桌台ID');
            }
        } catch (error) {
            console.error('解析scene参数失败:', error);
            this.setData({
                loading: false,
                error: '无效的扫码信息'
            });
        }
    },

    /**
     * 处理二维码URL参数
     */
    handleQRCodeParam(url: string) {
        try {
            // 解析URL获取桌台ID
            const urlObj = new URL(url);
            const tableId = urlObj.searchParams.get('table_id') ||
                urlObj.pathname.match(/\/table\/(\d+)/)?.[1];

            if (tableId && !isNaN(parseInt(tableId))) {
                this.loadTableInfo(parseInt(tableId));
            } else {
                throw new Error('URL中未找到桌台ID');
            }
        } catch (error) {
            console.error('解析二维码URL失败:', error);
            this.setData({
                loading: false,
                error: '无效的二维码信息'
            });
        }
    },

    /**
     * 加载桌台信息
     */
    async loadTableInfo(tableId: number) {
        try {
            this.setData({ loading: true, error: null });

            // 调用扫码接口验证桌台
            const scanResult = await scanTableQRCode(tableId);

            // 获取桌台详细信息
            const tableInfo = await getTableInfo(tableId);

            this.setData({
                loading: false,
                tableInfo: {
                    ...tableInfo,
                    ...scanResult
                },
                merchantInfo: scanResult.merchant
            });

            // 记录扫码行为
            this.trackScanBehavior(tableId, scanResult.merchant_id);

        } catch (error: any) {
            console.error('加载桌台信息失败:', error);
            this.setData({
                loading: false,
                error: error.message || '获取桌台信息失败，请重试'
            });
        }
    },

    /**
     * 记录扫码行为
     */
    async trackScanBehavior(tableId: number, merchantId: number) {
        try {
            // 记录用户行为用于数据分析
            wx.reportAnalytics('scan_table_qr', {
                table_id: tableId,
                merchant_id: merchantId,
                scan_time: new Date().toISOString()
            });
        } catch (error) {
            console.warn('记录扫码行为失败:', error);
        }
    },

    /**
     * 开始点餐
     */
    startDining() {
        const { tableInfo } = this.data;
        if (!tableInfo) return;

        // 跳转到堂食点餐页面
        wx.navigateTo({
            url: `/pages/dine-in/menu/menu?table_id=${tableInfo.id}&merchant_id=${tableInfo.merchant_id}`
        });
    },

    /**
     * 重新扫码
     */
    rescan() {
        wx.navigateBack();
    },

    /**
     * 查看商户信息
     */
    viewMerchantInfo() {
        const { merchantInfo } = this.data;
        if (!merchantInfo) return;

        wx.navigateTo({
            url: `/pages/merchant/detail/detail?id=${merchantInfo.id}`
        });
    },

    /**
     * 分享给好友
     */
    onShareAppMessage() {
        const { tableInfo, merchantInfo } = this.data;

        return {
            title: `${merchantInfo?.name || '餐厅'}的${tableInfo?.table_number || ''}号桌`,
            path: `/pages/dine-in/scan-entry/scan-entry?table_id=${tableInfo?.id}`,
            imageUrl: merchantInfo?.cover_image || merchantInfo?.logo_url
        };
    },

    /**
     * 分享到朋友圈
     */
    onShareTimeline() {
        const { tableInfo, merchantInfo } = this.data;

        return {
            title: `在${merchantInfo?.name || '餐厅'}用餐`,
            query: `table_id=${tableInfo?.id}`,
            imageUrl: merchantInfo?.cover_image || merchantInfo?.logo_url
        };
    },

    /**
     * 跳转到外卖页面
     */
    goToTakeout() {
        const { merchantInfo } = this.data;

        wx.switchTab({
            url: '/pages/takeout/index',
            success: () => {
                // 可以通过全局变量或事件传递商户信息
                getApp().globalData.recommendMerchant = merchantInfo;
            }
        });
    },

    /**
     * 跳转到包间预定页面
     */
    goToReservation() {
        const { merchantInfo } = this.data;

        wx.switchTab({
            url: '/pages/reservation/index',
            success: () => {
                // 可以通过全局变量或事件传递商户信息
                getApp().globalData.recommendMerchant = merchantInfo;
            }
        });
    }
});