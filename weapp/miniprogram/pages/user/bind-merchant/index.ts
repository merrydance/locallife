/**
 * 员工绑定商户页面
 * 用户扫描邀请码二维码或手动输入邀请码加入商户
 */

import { bindMerchant, BindMerchantResponse } from '../../../api/personal'
import { getWebLoginSessionStatus, confirmWebLoginSession } from '../../../api/auth'

const isBindMerchantError = (error: unknown): error is { statusCode?: number; message?: string } => {
    return !!error && typeof error === 'object'
}

Page({
    data: {
        inviteCode: '',
        loading: false,
        success: false,
        result: null as BindMerchantResponse | null
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
            scanType: ['qrCode', 'wxCode'],
            success: (res) => {
                void this.handleScanResult(res)
            }
        })
    },

    async handleScanResult(res: WechatMiniprogram.ScanCodeSuccessCallbackResult) {
        const payload = this.extractRawPayload(res)
        const raw = payload.raw
        const code = this.extractCode(payload.codeCandidate)

        if (!code) {
            const system = wx.getSystemInfoSync()
            if (system.platform === 'devtools') {
                wx.showModal({
                    title: '扫码结果为空',
                    content: '开发者工具中扫码可能不会返回内容，请使用真机扫码或手动输入。',
                    confirmText: '手动输入',
                    showCancel: false,
                    success: () => this.promptManualCode()
                })
                return
            }
            this.promptManualCode()
            return
        }

        await this.handleCodeCandidate(raw, code)
    },

    async handleCodeCandidate(raw: string, code: string) {
        if (raw.includes('bind-merchant') || raw.includes('code=')) {
            this.setData({ inviteCode: code })
            this.bindMerchant()
            return
        }

        try {
            const session = await getWebLoginSessionStatus(code)
            if (session?.code) {
                this.confirmWebLogin(code)
                return
            }
        } catch (error) {
            void error
        }

        this.setData({ inviteCode: code })
    },

    extractCode(raw: string) {
        if (!raw) return ''
        const decoded = decodeURIComponent(raw)
        const match = decoded.match(/code=([^&]+)/)
        if (match) return match[1]
        const webLoginMatch = decoded.match(/web-login:([0-9a-fA-F]{32})/)
        if (webLoginMatch) return webLoginMatch[1]
        const inviteMatch = decoded.match(/invite-merchant:([A-Za-z0-9_-]+)/)
        if (inviteMatch) return inviteMatch[1]
        const hexMatch = decoded.match(/[0-9a-fA-F]{32}/)
        if (hexMatch) return hexMatch[0]
        return decoded
    },

    extractRawPayload(res: WechatMiniprogram.ScanCodeSuccessCallbackResult) {
        const anyRes = res as any
        const path = anyRes.path || ''
        const result = anyRes.result || ''
        const rawData = anyRes.rawData || ''
        const scene = anyRes.scene || ''
        const query = anyRes.query || {}
        const codeFromQuery = query.code || ''
        const candidate = [path, result, rawData, scene, codeFromQuery].find((val) => !!val) || ''
        return {
            raw: String(candidate),
            codeCandidate: String(codeFromQuery || candidate || ''),
        }
    },

    promptManualCode() {
        wx.showModal({
            title: '输入扫码内容',
            content: '未识别到二维码内容，请粘贴登录码或入职码',
            editable: true,
            placeholderText: 'web-login:xxxx 或 邀请码',
            confirmText: '继续',
            success: async (res) => {
                if (!res.confirm || !res.content) return
                const raw = String(res.content)
                const code = this.extractCode(raw)
                if (!code) {
                    wx.showToast({ title: '内容无效', icon: 'none' })
                    return
                }
                await this.handleCodeCandidate(raw, code)
            }
        })
    },

    confirmWebLogin(code: string) {
        wx.showModal({
            title: 'Web 登录确认',
            content: '检测到 Web 登录码，是否确认登录网页端？',
            confirmText: '确认登录',
            cancelText: '取消',
            success: async (modal) => {
                if (!modal.confirm) return
                wx.showLoading({ title: '确认中...' })
                try {
                    await confirmWebLoginSession(code)
                    wx.showToast({ title: '已确认登录', icon: 'success' })
                } catch (error: any) {
                    wx.showToast({ title: error?.message || '确认失败', icon: 'none' })
                } finally {
                    wx.hideLoading()
                }
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
            const result = await bindMerchant(inviteCode.trim())

            this.setData({
                success: true,
                result,
                loading: false
            })

            wx.showToast({ title: '加入成功', icon: 'success' })
        } catch (error: unknown) {
            console.error('绑定失败:', error)
            this.setData({ loading: false })

            // 友好提示已入职情况
            if (isBindMerchantError(error) && (error.statusCode === 409 || error.message?.includes('already'))) {
                wx.showModal({
                    title: '已入职',
                    content: '您已经是该商户的员工，无需重复绑定',
                    showCancel: false,
                    confirmText: '我知道了'
                })
            } else {
                wx.showToast({ title: error instanceof Error ? error.message : '绑定失败', icon: 'none' })
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
