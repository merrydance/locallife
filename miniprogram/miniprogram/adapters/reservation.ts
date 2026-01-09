/**
 * 预订数据适配器
 * 用于格式化预订数据以供前端展示
 */

import { ReservationResponse, ReservationStatus } from '../api/reservation'

export class ReservationAdapter {

  /**
   * 格式化预订状态文本
   */
  static formatStatus(status: ReservationStatus): string {
    const statusMap: Record<ReservationStatus, string> = {
      'pending': '待支付',
      'paid': '已支付',
      'confirmed': '已确认',
      'checked_in': '已到店',
      'completed': '已完成',
      'cancelled': '已取消',
      'expired': '已过期',
      'no_show': '未到店'
    }
    return statusMap[status] || status
  }

  /**
   * 获取状态对应的颜色 (TDesign Tag Theme)
   */
  static getStatusTheme(status: ReservationStatus): string {
    const themeMap: Record<ReservationStatus, string> = {
      'pending': 'warning',
      'paid': 'primary',
      'confirmed': 'primary',
      'checked_in': 'success',
      'completed': 'success',
      'cancelled': 'default',
      'expired': 'default',
      'no_show': 'danger'
    }
    return themeMap[status] || 'default'
  }

  /**
   * 格式化时间显示 (例如: 12月25日 18:30)
   */
  static formatDateTime(dateStr: string): string {
    if (!dateStr) return ''
    const date = new Date(dateStr)
    const month = date.getMonth() + 1
    const day = date.getDate()
    const hours = ('0' + date.getHours()).slice(-2)
    const minutes = ('0' + date.getMinutes()).slice(-2)
    return `${month}月${day}日 ${hours}:${minutes}`
  }

  /**
   * 格式化完整时间 (例如: 2023-12-25 18:30:00)
   */
  static formatFullDateTime(dateStr: string): string {
    if (!dateStr) return ''
    const date = new Date(dateStr)
    const year = date.getFullYear()
    const month = ('0' + (date.getMonth() + 1)).slice(-2)
    const day = ('0' + date.getDate()).slice(-2)
    const hours = ('0' + date.getHours()).slice(-2)
    const minutes = ('0' + date.getMinutes()).slice(-2)
    const seconds = ('0' + date.getSeconds()).slice(-2)
    return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
  }

  /**
   * 格式化金额 (分 -> 元)
   */
  static formatAmount(cents: number): string {
    return (cents / 100).toFixed(2)
  }

  /**
   * 验证预订信息是否有效
   */
  static validateReservation(data: {
    reservation_time: string,
    party_size: number,
    contact_name: string,
    contact_phone: string
  }): { valid: boolean, message?: string } {
    if (!data.reservation_time) return { valid: false, message: '请选择预订时间' }
    if (!data.party_size || data.party_size <= 0) return { valid: false, message: '请输入正确的用餐人数' }
    if (!data.contact_name) return { valid: false, message: '请输入联系人姓名' }
    if (!data.contact_phone || !/^1\d{10}$/.test(data.contact_phone)) return { valid: false, message: '请输入正确的手机号码' }

    const time = new Date(data.reservation_time).getTime()
    if (time < Date.now()) return { valid: false, message: '预订时间不能早于当前时间' }

    return { valid: true }
  }
}

export default ReservationAdapter
