/**
 * 设置中心 - 桌面级 SaaS 实现
 * 对齐后端 API：
 * - GET/PATCH /v1/merchants/me - 商户信息
 * - CRUD /v1/merchant/devices - 打印机
 * - GET/PUT /v1/merchant/display-config - 显示配置
 */

import { request, API_BASE } from '../../../utils/request'
import { getToken } from '../../../utils/auth'
import { resolveImageURL } from '../../../utils/image-security'
import { logger } from '../../../utils/logger'

const app = getApp<IAppOption>()

// 类型定义
interface MerchantInfo {
    id: number
    name: string
    description?: string
    logo_url?: string
    phone: string
    address: string
    latitude?: string
    longitude?: string
    is_open: boolean
    version?: number
}

interface PrinterInfo {
    id?: number
    printer_name: string
    printer_sn: string
    printer_key: string
    printer_type: string
    print_takeout: boolean
    print_dine_in: boolean
    print_reservation: boolean
    is_active: boolean
}

interface DisplayConfig {
    enable_print: boolean
    print_takeout: boolean
    print_dine_in: boolean
    print_reservation: boolean
    enable_voice: boolean
    voice_takeout: boolean
    voice_dine_in: boolean
    enable_kds: boolean
    kds_url: string
}

Page({
    data: {
        // SaaS 布局
        sidebarCollapsed: false,
        merchantName: '',
        isOpen: true,

        // 导航
        activeTab: 'profile',

        // 商户信息
        merchant: {} as MerchantInfo,
        originalMerchant: {} as MerchantInfo,
        saving: false,
        descriptionLength: 0,

        // 打印机
        printers: [] as PrinterInfo[],
        showPrinterModal: false,
        editingPrinter: {
            printer_name: '',
            printer_sn: '',
            printer_key: '',
            printer_type: 'feieyun',
            print_takeout: true,
            print_dine_in: true,
            print_reservation: true
        } as PrinterInfo,
        savingPrinter: false,

        // 显示配置
        displayConfig: {
            enable_print: true,
            print_takeout: true,
            print_dine_in: true,
            print_reservation: true,
            enable_voice: false,
            voice_takeout: true,
            voice_dine_in: true,
            enable_kds: false,
            kds_url: ''
        } as DisplayConfig,
        savingConfig: false
    },

    onLoad() {
        this.loadMerchantInfo()
        this.loadPrinters()
        this.loadDisplayConfig()
    },

    // 切换标签
    switchTab(e: WechatMiniprogram.TouchEvent) {
        const tab = e.currentTarget.dataset.tab
        this.setData({ activeTab: tab })
    },

    // ========== 商户信息 ==========
    async loadMerchantInfo() {
        try {
            const res = await request<MerchantInfo>({
                url: '/v1/merchants/me',
                method: 'GET'
            })

            // 处理 logo_url 确保能正确显示
            if (res.logo_url) {
                res.logo_url = await resolveImageURL(res.logo_url)
            }

            this.setData({
                merchant: res,
                originalMerchant: { ...res },
                merchantName: res.name,
                isOpen: res.is_open,
                descriptionLength: (res.description || '').length
            })
        } catch (error) {
            logger.error('加载商户信息失败', error, 'Settings')
        }
    },

    onInput(e: WechatMiniprogram.Input) {
        const field = e.currentTarget.dataset.field
        const value = e.detail.value
        const updates: Record<string, any> = {
            [`merchant.${field}`]: value
        }
        if (field === 'description') {
            updates.descriptionLength = value.length
        }
        this.setData(updates)
    },

    async uploadLogo() {
        try {
            const res = await wx.chooseMedia({
                count: 1,
                mediaType: ['image'],
                sourceType: ['album', 'camera']
            })

            const tempFilePath = res.tempFiles[0].tempFilePath

            wx.showLoading({ title: '上传中...' })

            // 上传图片到服务器
            const uploadRes = await new Promise<string>((resolve, reject) => {
                wx.uploadFile({
                    url: `${API_BASE}/v1/merchants/images/upload`,
                    filePath: tempFilePath,
                    name: 'image',
                    formData: {
                        category: 'logo'
                    },
                    header: {
                        Authorization: `Bearer ${getToken()}`
                    },
                    success: (res) => {
                        try {
                            const data = JSON.parse(res.data)
                            if (data.image_url) {
                                resolve(data.image_url)
                            } else if (data.url) {
                                resolve(data.url)
                            } else {
                                reject(new Error(data.error || '上传失败'))
                            }
                        } catch (e) {
                            reject(new Error('解析响应失败'))
                        }
                    },
                    fail: reject
                })
            })

            // 使用项目标准方法处理图片URL（公共路径直接拼接，私有路径会签名）
            const logoUrl = await resolveImageURL(uploadRes)
            this.setData({ 'merchant.logo_url': logoUrl })
            wx.hideLoading()
            wx.showToast({ title: '上传成功', icon: 'success' })
        } catch (error) {
            wx.hideLoading()
            logger.error('上传 Logo 失败', error, 'Settings')
            wx.showToast({ title: '上传失败', icon: 'error' })
        }
    },

    async chooseLocation() {
        try {
            const res = await wx.chooseLocation({})
            this.setData({
                'merchant.address': res.address,
                'merchant.latitude': res.latitude.toString(),
                'merchant.longitude': res.longitude.toString()
            })
        } catch (error) {
            logger.warn('选择位置取消', error, 'Settings')
        }
    },

    resetProfile() {
        this.setData({
            merchant: { ...this.data.originalMerchant }
        })
        wx.showToast({ title: '已重置', icon: 'success' })
    },

    async saveProfile() {
        const { merchant } = this.data

        // 验证
        if (!merchant.name || merchant.name.length < 2) {
            wx.showToast({ title: '店铺名称至少2个字符', icon: 'none' })
            return
        }
        if (!merchant.phone || merchant.phone.length !== 11) {
            wx.showToast({ title: '请输入11位手机号', icon: 'none' })
            return
        }
        if (!merchant.address || merchant.address.length < 5) {
            wx.showToast({ title: '地址至少5个字符', icon: 'none' })
            return
        }

        this.setData({ saving: true })

        try {
            const res = await request<MerchantInfo>({
                url: '/v1/merchants/me',
                method: 'PATCH',
                data: {
                    name: merchant.name,
                    description: merchant.description,
                    logo_url: merchant.logo_url,
                    phone: merchant.phone,
                    address: merchant.address,
                    latitude: merchant.latitude,
                    longitude: merchant.longitude,
                    version: merchant.version || 1
                }
            })

            // 处理 logo_url 确保能正确显示
            if (res.logo_url) {
                res.logo_url = await resolveImageURL(res.logo_url)
            }

            this.setData({
                merchant: res,
                originalMerchant: { ...res },
                merchantName: res.name
            })

            wx.showToast({ title: '保存成功', icon: 'success' })
        } catch (error: any) {
            logger.error('保存商户信息失败', error, 'Settings')
            if (error.message?.includes('version') || error.message?.includes('conflict')) {
                wx.showToast({ title: '数据已被修改，请刷新', icon: 'none' })
                this.loadMerchantInfo()
            } else {
                wx.showToast({ title: '保存失败', icon: 'error' })
            }
        } finally {
            this.setData({ saving: false })
        }
    },

    // ========== 打印机管理 ==========
    async loadPrinters() {
        try {
            const res = await request<PrinterInfo[]>({
                url: '/v1/merchant/devices',
                method: 'GET'
            })

            this.setData({ printers: res || [] })
        } catch (error) {
            logger.error('加载打印机列表失败', error, 'Settings')
        }
    },

    addPrinter() {
        this.setData({
            showPrinterModal: true,
            editingPrinter: {
                printer_name: '',
                printer_sn: '',
                printer_key: '',
                printer_type: 'feieyun',
                print_takeout: true,
                print_dine_in: true,
                print_reservation: true,
                is_active: true
            }
        })
    },

    editPrinter(e: WechatMiniprogram.TouchEvent) {
        const id = e.currentTarget.dataset.id
        const printer = this.data.printers.find(p => p.id === id)

        if (printer) {
            this.setData({
                showPrinterModal: true,
                editingPrinter: { ...printer }
            })
        }
    },

    closePrinterModal() {
        this.setData({ showPrinterModal: false })
    },

    onPrinterInput(e: WechatMiniprogram.Input) {
        const field = e.currentTarget.dataset.field
        this.setData({
            [`editingPrinter.${field}`]: e.detail.value
        })
    },

    selectPrinterType(e: WechatMiniprogram.TouchEvent) {
        const type = e.currentTarget.dataset.type
        this.setData({ 'editingPrinter.printer_type': type })
    },

    togglePrinterScene(e: WechatMiniprogram.TouchEvent) {
        const field = e.currentTarget.dataset.field
        const current = (this.data.editingPrinter as any)[field]
        this.setData({
            [`editingPrinter.${field}`]: !current
        })
    },

    async savePrinter() {
        const { editingPrinter } = this.data

        // 验证
        if (!editingPrinter.printer_name) {
            wx.showToast({ title: '请输入打印机名称', icon: 'none' })
            return
        }

        if (!editingPrinter.id) {
            // 新增时需要验证更多字段
            if (!editingPrinter.printer_sn) {
                wx.showToast({ title: '请输入打印机序列号', icon: 'none' })
                return
            }
            if (!editingPrinter.printer_key) {
                wx.showToast({ title: '请输入打印机密钥', icon: 'none' })
                return
            }
        }

        this.setData({ savingPrinter: true })

        try {
            if (editingPrinter.id) {
                // 更新
                await request({
                    url: `/v1/merchant/devices/${editingPrinter.id}`,
                    method: 'PATCH',
                    data: {
                        printer_name: editingPrinter.printer_name,
                        print_takeout: editingPrinter.print_takeout,
                        print_dine_in: editingPrinter.print_dine_in,
                        print_reservation: editingPrinter.print_reservation
                    }
                })
            } else {
                // 新增
                await request({
                    url: '/v1/merchant/devices',
                    method: 'POST',
                    data: {
                        printer_name: editingPrinter.printer_name,
                        printer_sn: editingPrinter.printer_sn,
                        printer_key: editingPrinter.printer_key,
                        printer_type: editingPrinter.printer_type,
                        print_takeout: editingPrinter.print_takeout,
                        print_dine_in: editingPrinter.print_dine_in,
                        print_reservation: editingPrinter.print_reservation
                    }
                })
            }

            wx.showToast({ title: '保存成功', icon: 'success' })
            this.closePrinterModal()
            this.loadPrinters()
        } catch (error) {
            logger.error('保存打印机失败', error, 'Settings')
            wx.showToast({ title: '保存失败', icon: 'error' })
        } finally {
            this.setData({ savingPrinter: false })
        }
    },

    async togglePrinter(e: WechatMiniprogram.TouchEvent) {
        const id = e.currentTarget.dataset.id
        const printer = this.data.printers.find(p => p.id === id)

        if (!printer) return

        try {
            await request({
                url: `/v1/merchant/devices/${id}`,
                method: 'PATCH',
                data: {
                    is_active: !printer.is_active
                }
            })

            this.loadPrinters()
            wx.showToast({ title: printer.is_active ? '已禁用' : '已启用', icon: 'success' })
        } catch (error) {
            logger.error('切换打印机状态失败', error, 'Settings')
            wx.showToast({ title: '操作失败', icon: 'error' })
        }
    },

    async deletePrinter(e: WechatMiniprogram.TouchEvent) {
        const id = e.currentTarget.dataset.id

        wx.showModal({
            title: '确认删除',
            content: '删除后无法恢复，确定要删除这台打印机吗？',
            success: async (res) => {
                if (res.confirm) {
                    try {
                        await request({
                            url: `/v1/merchant/devices/${id}`,
                            method: 'DELETE'
                        })

                        this.loadPrinters()
                        wx.showToast({ title: '已删除', icon: 'success' })
                    } catch (error) {
                        logger.error('删除打印机失败', error, 'Settings')
                        wx.showToast({ title: '删除失败', icon: 'error' })
                    }
                }
            }
        })
    },

    // ========== 显示配置 ==========
    async loadDisplayConfig() {
        try {
            const res = await request<DisplayConfig>({
                url: '/v1/merchant/display-config',
                method: 'GET'
            })

            this.setData({ displayConfig: res })
        } catch (error) {
            logger.error('加载显示配置失败', error, 'Settings')
        }
    },

    toggleConfig(e: WechatMiniprogram.TouchEvent) {
        const field = e.currentTarget.dataset.field
        const current = (this.data.displayConfig as any)[field]
        this.setData({
            [`displayConfig.${field}`]: !current
        })
    },

    onConfigInput(e: WechatMiniprogram.Input) {
        const field = e.currentTarget.dataset.field
        this.setData({
            [`displayConfig.${field}`]: e.detail.value
        })
    },

    async saveDisplayConfig() {
        this.setData({ savingConfig: true })

        try {
            const { displayConfig } = this.data

            await request({
                url: '/v1/merchant/display-config',
                method: 'PUT',
                data: {
                    enable_print: displayConfig.enable_print,
                    print_takeout: displayConfig.print_takeout,
                    print_dine_in: displayConfig.print_dine_in,
                    print_reservation: displayConfig.print_reservation,
                    enable_voice: displayConfig.enable_voice,
                    voice_takeout: displayConfig.voice_takeout,
                    voice_dine_in: displayConfig.voice_dine_in,
                    enable_kds: displayConfig.enable_kds,
                    kds_url: displayConfig.kds_url || null
                }
            })

            wx.showToast({ title: '保存成功', icon: 'success' })
        } catch (error) {
            logger.error('保存显示配置失败', error, 'Settings')
            wx.showToast({ title: '保存失败', icon: 'error' })
        } finally {
            this.setData({ savingConfig: false })
        }
    },

    // ========== 通用方法 ==========
    onSidebarCollapse(e: WechatMiniprogram.CustomEvent) {
        this.setData({ sidebarCollapsed: e.detail.collapsed })
    },

    goBack() {
        wx.navigateBack({
            fail: () => {
                wx.redirectTo({ url: '/pages/merchant/dashboard/index' })
            }
        })
    }
})
