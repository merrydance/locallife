/**
 * 商户打印机管理页面
 * 使用真实后端API
 */

import { isLargeScreen } from '../../../utils/responsive'
import { deviceManagementService, PrinterResponse } from '../../../api/table-device-management'

Page({
  data: {
    printers: [] as any[],
    isLargeScreen: false,
    navBarHeight: 88,
    loading: false
  },

  onLoad() {
    this.setData({ isLargeScreen: isLargeScreen() })
    this.loadPrinters()
  },

  onShow() {
    // 返回时刷新
    if (this.data.printers.length > 0) {
      this.loadPrinters()
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadPrinters() {
    this.setData({ loading: true })

    try {
      const result = await deviceManagementService.listPrinters()

      const printers = (result.printers || []).map((printer: PrinterResponse) => ({
        id: printer.id,
        name: printer.printer_name,
        type: printer.printer_type?.toUpperCase() || 'UNKNOWN',
        sn: printer.printer_sn,
        status: printer.is_active ? 'ONLINE' : 'OFFLINE',
        auto_print: true,
        print_takeout: printer.print_takeout,
        print_dine_in: printer.print_dine_in,
        print_reservation: printer.print_reservation
      }))

      this.setData({
        printers,
        loading: false
      })
    } catch (error) {
      console.error('加载打印机失败:', error)
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
  },

  onAddPrinter() {
    wx.navigateTo({ url: '/pages/merchant/printers/add/index' })
  },

  async onTestPrint(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset

    try {
      wx.showLoading({ title: '测试打印中...' })
      await deviceManagementService.testPrinter(id)
      wx.hideLoading()
      wx.showToast({ title: '打印成功', icon: 'success' })
    } catch (error) {
      wx.hideLoading()
      console.error('测试打印失败:', error)
      wx.showToast({ title: '打印失败', icon: 'error' })
    }
  },

  onEditPrinter(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    wx.navigateTo({ url: `/pages/merchant/printers/edit/index?id=${id}` })
  },

  onDeletePrinter(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    wx.showModal({
      title: '删除确认',
      content: '确认删除此打印机?',
      success: async (res) => {
        if (res.confirm) {
          try {
            await deviceManagementService.deletePrinter(id)
            wx.showToast({ title: '已删除', icon: 'success' })
            this.loadPrinters()
          } catch (error) {
            console.error('删除打印机失败:', error)
            wx.showToast({ title: '删除失败', icon: 'error' })
          }
        }
      }
    })
  }
})
