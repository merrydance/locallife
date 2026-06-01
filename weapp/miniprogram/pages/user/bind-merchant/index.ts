/**
 * 员工绑定商户页面
 * 用户扫描邀请码二维码或手动输入邀请码加入商户
 */

import { bindMerchant, BindMerchantResponse } from '../../../api/personal'
import { getWebLoginSessionStatus, confirmWebLoginSession } from '../../../api/auth'
import { getErrorDebugMessage, getErrorUserMessage } from '../../../utils/user-facing'
import { invalidateConsoleAccessUserInfoCache } from '../../../utils/console-access'

const USER_CENTER_FORCE_REFRESH_FLAG = 'user_center_force_refresh_after_bind_merchant'

interface WebLoginSessionLookupResult {
    code?: string
    status?: string
}

const isBindMerchantError = (error: unknown): error is { statusCode?: number, message?: string } => {
    return !!error && typeof error === 'object'
}

const getErrorStatusCode = (error: unknown): number => {
    if (!error || typeof error !== 'object') return 0
    const knownError = error as { statusCode?: number | string, code?: number | string }
    const candidates = [knownError.statusCode, knownError.code]
    for (const candidate of candidates) {
        const numericStatusCode = typeof candidate === 'number' ? candidate : Number(candidate)
        if (Number.isFinite(numericStatusCode)) {
            return numericStatusCode
        }
    }
    return 0
}

const isUsableWebLoginSession = (session?: WebLoginSessionLookupResult | null): boolean => {
    if (!session?.code) return false
    return session.status !== 'expired' && session.status !== 'consumed'
}

const getErrorMessage = getErrorUserMessage

Page({
    data: {
        inviteCode: '',
        loading: false,
        success: false,
        result: null as BindMerchantResponse | null,
        navBarHeight: 88
    },

    onLoad(options: { code?: string }) {
        // 如果扫码进入，自动填入邀请码
        if (options.code) {
            this.setData({ inviteCode: options.code })
            // 自动绑定
            this.bindMerchant()
        }
    },

    onNavHeight(e: WechatMiniprogram.CustomEvent) {
        this.setData({ navBarHeight: e.detail.navBarHeight })
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
        const webLoginMeta = this.extractWebLoginMeta(raw)
        const code = webLoginMeta.code || this.extractCode(payload.codeCandidate)

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

        await this.handleCodeCandidate(raw, code, webLoginMeta)
    },

    async handleCodeCandidate(raw: string, code: string, webLoginMeta?: { code?: string, sig?: string, ts?: number }) {
        const isWebLoginHint = raw.includes('web-login') || raw.includes('/merchant/login') || raw.includes('sig=') || raw.includes('ts=')
        const isInviteHint = raw.includes('invite-merchant') || raw.includes('bind-merchant')

        if (isWebLoginHint) {
            const loginCode = webLoginMeta?.code || code
            try {
                const session = await getWebLoginSessionStatus(loginCode)
                if (isUsableWebLoginSession(session)) {
                    this.confirmWebLogin(loginCode, webLoginMeta?.sig, webLoginMeta?.ts)
                    return
                }
            } catch (error) {
                if (getErrorStatusCode(error) === 404) {
                    this.showInvalidWebLoginCode()
                } else {
                    this.showWebLoginStatusCheckFailed()
                }
                return
            }

            this.showInvalidWebLoginCode()
            return
        }

        if (isInviteHint || (!isWebLoginHint && raw.includes('code='))) {
            this.setData({ inviteCode: code })
            this.bindMerchant()
            return
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

    extractWebLoginMeta(raw: string) {
        if (!raw) return { code: '', sig: '', ts: undefined }
        const decoded = decodeURIComponent(raw)
        const queryCodeMatch = decoded.match(/code=([^&]+)/)
        const webLoginMatch = decoded.match(/web-login:([0-9a-fA-F]{32})/)
        const code = queryCodeMatch ? queryCodeMatch[1] : webLoginMatch ? webLoginMatch[1] : ''
        if (!code) return { code: '', sig: '', ts: undefined }
        const sigMatch = decoded.match(/sig=([0-9a-fA-F]+)/)
        const tsMatch = decoded.match(/ts=(\d+)/)
        return {
            code,
            sig: sigMatch ? sigMatch[1] : '',
            ts: tsMatch ? Number(tsMatch[1]) : undefined
        }
    },

    extractRawPayload(res: WechatMiniprogram.ScanCodeSuccessCallbackResult) {
        const payload = res as WechatMiniprogram.ScanCodeSuccessCallbackResult & {
            path?: string
            result?: string
            rawData?: string
            scene?: string
            query?: { code?: string }
        }
        const path = payload.path || ''
        const result = payload.result || ''
        const rawData = payload.rawData || ''
        const scene = payload.scene || ''
        const query = payload.query || {}
        const codeFromQuery = query.code || ''
        const candidate = [path, result, rawData, scene, codeFromQuery].find((val) => !!val) || ''
        return {
            raw: String(candidate),
            codeCandidate: String(codeFromQuery || candidate || '')
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
                const webLoginMeta = this.extractWebLoginMeta(raw)
                const code = webLoginMeta.code || this.extractCode(raw)
                if (!code) {
                    wx.showToast({ title: '内容无效', icon: 'none' })
                    return
                }
                await this.handleCodeCandidate(raw, code, webLoginMeta)
            }
        })
    },

    showInvalidWebLoginCode() {
        wx.showModal({
            title: '网页登录码无效',
            content: '当前网页登录码无效或已失效，请返回网页重新生成二维码后重试。',
            showCancel: false,
            confirmText: '我知道了'
        })
    },

    showWebLoginStatusCheckFailed() {
        wx.showModal({
            title: '状态校验失败',
            content: '网页登录状态校验失败，请稍后重试。',
            showCancel: false,
            confirmText: '我知道了'
        })
    },

    confirmWebLogin(code: string, sig?: string, ts?: number) {
        wx.showModal({
            title: 'Web 登录确认',
            content: '检测到 Web 登录码，是否确认登录网页端？',
            confirmText: '确认登录',
            cancelText: '取消',
            success: async (modal) => {
                if (!modal.confirm) return
                if (!sig || !ts) {
                    wx.showModal({
                        title: '二维码无效',
                        content: '当前二维码缺少校验信息，请在网页端重新生成二维码后重试。',
                        showCancel: false,
                        confirmText: '我知道了'
                    })
                    return
                }
                wx.showLoading({ title: '确认中...' })
                try {
                    await confirmWebLoginSession(code, sig, ts)
                    wx.showToast({ title: '已确认登录', icon: 'success' })
                } catch (error: unknown) {
                    wx.showToast({ title: getErrorMessage(error, '确认失败'), icon: 'none' })
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
            invalidateConsoleAccessUserInfoCache()
            wx.setStorageSync(USER_CENTER_FORCE_REFRESH_FLAG, '1')

            this.setData({
                success: true,
                result,
                loading: false
            })
        } catch (error: unknown) {
            console.error('绑定失败:', error)
            this.setData({ loading: false })

            // 友好提示已入职情况
            const debugMessage = getErrorDebugMessage(error).toLowerCase()
            if (isBindMerchantError(error) && (error.statusCode === 409 || debugMessage.includes('already'))) {
                wx.showModal({
                    title: '已入职',
                    content: '您已经是该商户的员工，无需重复绑定',
                    showCancel: false,
                    confirmText: '我知道了'
                })
            } else {
                wx.showToast({ title: getErrorUserMessage(error, '绑定失败，请稍后重试'), icon: 'none' })
            }
        }
    },

    // 前往商户工作台
    goToMerchant() {
        if (this.data.result?.role === 'pending') {
            wx.switchTab({
                url: '/pages/user_center/index'
            })
            return
        }
        wx.reLaunch({
            url: '/pages/merchant/dashboard/index'
        })
    },

    // 返回首页
    goHome() {
        wx.switchTab({
            url: '/pages/takeout/index'
        })
    }
})
