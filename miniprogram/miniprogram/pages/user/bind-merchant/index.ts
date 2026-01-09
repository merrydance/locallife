/**
 * 员工绑定商户页面
 * 用户扫描邀请码二维码或手动输入邀请码加入商户
 */

import { request } from '@/utils/request'

interface BindResult {
    message: string
    merchant_id: number
    merchant_name: string
    role: string
}

Page({
    data: {
        inviteCode: '',
        loading: false,
        success: false,
        result: null as BindResult | null
    },

    onLoad(options: { code?: string }) {
        // 如果扫码进入，自动填入邀请码
        if (options.code) {
            this.setData({ inviteCode: options.code })
            // 自动绑定
            this.bindMerchant()
        }
    },

    // 输入邀请码
    onInput(e: WechatMiniprogram.Input) {
        this.setData({ inviteCode: e.detail.value })
    },

    // 扫描二维码
    onScan() {
        wx.scanCode({
            onlyFromCamera: false,
            success: (res) => {
                // 检查是否是本小程序的页面路径
                const result = res.result
                if (result.includes('bind-merchant') && result.includes('code=')) {
                    // 从路径中提取 code 参数
                    const match = result.match(/code=([^&]+)/)
                    if (match) {
                        this.setData({ inviteCode: match[1] })
                        this.bindMerchant()
                        return
                    }
                }
                // 直接是邀请码
                this.setData({ inviteCode: result })
            }
        })
    },

    // 绑定商户
    async bindMerchant() {
        const { inviteCode } = this.data
        if (!inviteCode.trim()) {
            wx.showToast({ title: '请输入邀请码', icon: 'none' })
            return
        }

        this.setData({ loading: true })
        try {
            const result = await request<BindResult>({
                url: '/v1/bind-merchant',
                method: 'POST',
                data: { invite_code: inviteCode.trim() }
            })

            this.setData({
                success: true,
                result,
                loading: false
            })

            wx.showToast({ title: '加入成功', icon: 'success' })
        } catch (error: any) {
            console.error('绑定失败:', error)
            this.setData({ loading: false })

            // 友好提示已入职情况
            if (error.statusCode === 409 || error.message?.includes('already')) {
                wx.showModal({
                    title: '已入职',
                    content: '您已经是该商户的员工，无需重复绑定',
                    showCancel: false,
                    confirmText: '我知道了'
                })
            } else {
                wx.showToast({ title: error.message || '绑定失败', icon: 'none' })
            }
        }
    },

    // 前往商户工作台
    goToMerchant() {
        wx.reLaunch({
            url: '/pages/merchant/dashboard/index'
        })
    },

    // 返回首页
    goHome() {
        wx.reLaunch({
            url: '/pages/index/index'
        })
    }
})
