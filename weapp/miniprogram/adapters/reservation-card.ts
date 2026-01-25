/**
 * 预订卡片适配器
 * 将ReservationResponse转换为列表展示所需的ReservationCardViewModel
 */

import { ReservationResponse, ReservationStatus } from '../api/reservation'

export interface ReservationCardViewModel {
    id: number
    merchantName: string
    merchantId: number
    status: ReservationStatus
    statusText: string
    statusClass: string
    statusTagTheme: 'primary' | 'success' | 'warning' | 'danger' | 'default'
    dateTimeDisplay: string
    guestCount: number
    itemsPreview: string
    depositDisplay?: string
    canCancel: boolean
    canPay: boolean
    canOrder: boolean
    canViewDetail: boolean
    notes?: string
}

export interface ReservationItemViewModel {
    id?: number
    name?: string
    quantity: number
    unitPriceDisplay: string
    totalPriceDisplay: string
    imageUrl: string
}

export interface ReservationDetailViewModel extends ReservationCardViewModel {
    merchantAddress: string
    merchantPhone: string
    tableNo?: string
    contactName: string
    contactPhone: string
    createdAt: string
    paymentMode: string
    paymentModeText: string
    items: ReservationItemViewModel[]
    itemsTotalDisplay: string
}

export const ReservationCardAdapter = {
    /**
     * 将API响应转换为CardViewModel
     */
    toCardViewModel(dto: ReservationResponse): ReservationCardViewModel {
        const statusInfo = getStatusStyle(dto.status)
        
        // 格式化时间
        const dateTimeDisplay = formatReservationTime(dto.reservation_date, dto.reservation_time)

        // 拼接菜品预览 (如果有)
        let itemsPreview = ''
        if (dto.items && dto.items.length > 0) {
            const names = dto.items.slice(0, 3).map(i => i.name).join('、')
            itemsPreview = `已点餐品: ${names}${dto.items.length > 3 ? ' 等' : ''}`
        }

        return {
            id: dto.id,
            merchantName: dto.merchant_name || '未知商户',
            merchantId: dto.merchant_id,
            status: dto.status,
            statusText: statusInfo.text,
            statusClass: dto.status, // 用于CSS类
            statusTagTheme: statusInfo.theme,
            dateTimeDisplay,
            guestCount: dto.guest_count,
            itemsPreview,
            depositDisplay: dto.deposit_amount > 0 ? `¥${(dto.deposit_amount / 100).toFixed(2)}` : undefined,
            canCancel: ['pending', 'paid', 'confirmed'].includes(dto.status),
            canPay: dto.status === 'pending',
            canOrder: ['confirmed', 'checked_in'].includes(dto.status),
            canViewDetail: true,
            notes: dto.notes
        }
    },

    /**
     * 将API响应转换为详情ViewModel
     */
    toDetailViewModel(dto: ReservationResponse): ReservationDetailViewModel {
        const base = ReservationCardAdapter.toCardViewModel(dto)
        const items = (dto.items || []).map(item => ({
             ...item,
             unitPriceDisplay: `¥${((item.unit_price || 0) / 100).toFixed(2)}`,
             totalPriceDisplay: `¥${((item.total_price || (item.unit_price || 0) * item.quantity) / 100).toFixed(2)}`,
             imageUrl: item.image_url || ''
        }))

        // 计算预点菜总价
        const itemsTotal = items.reduce((sum, item) => sum + (item.total_price || (item.unit_price || 0) * item.quantity), 0)

        // 格式化创建时间
        let createdAt = dto.created_at
        try {
             createdAt = dto.created_at.substring(0, 16).replace('T', ' ')
        } catch(e) {}

        return {
            ...base,
            merchantAddress: dto.merchant_address || '',
            merchantPhone: dto.merchant_phone || '',
            tableNo: dto.table_type ? `${dto.table_type} ${dto.table_no || ''}` : dto.table_no,
            contactName: dto.contact_name,
            contactPhone: dto.contact_phone,
            createdAt,
            paymentMode: dto.payment_mode,
            paymentModeText: dto.payment_mode === 'deposit' ? '定金留座' : '在线点餐',
            items,
            itemsTotalDisplay: `¥${(itemsTotal / 100).toFixed(2)}`
        }
    }
}

function getStatusStyle(status: ReservationStatus): { text: string; theme: ReservationCardViewModel['statusTagTheme'] } {
    const map: Record<ReservationStatus, { text: string; theme: ReservationCardViewModel['statusTagTheme'] }> = {
        'pending': { text: '待支付', theme: 'danger' },
        'paid': { text: '已支付', theme: 'primary' },
        'confirmed': { text: '已确认', theme: 'success' },
        'checked_in': { text: '已入座', theme: 'success' },
        'completed': { text: '已完成', theme: 'default' },
        'cancelled': { text: '已取消', theme: 'default' },
        'expired': { text: '已过期', theme: 'default' },
        'no_show': { text: '未到店', theme: 'warning' }
    }
    return map[status] || { text: status, theme: 'default' }
}

function formatReservationTime(dateStr: string, timeStr: string): string {
    if (!dateStr || !timeStr) return ''
    try {
        // 简单处理：如果是当年的，不显示年份；如果是今天/明天/后天，显示相对词
        const target = new Date(dateStr.replace(/-/g, '/'))
        const now = new Date()
        const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
        const targetDate = new Date(target.getFullYear(), target.getMonth(), target.getDate())
        
        const diffTime = targetDate.getTime() - today.getTime()
        const diffDays = Math.round(diffTime / (1000 * 60 * 60 * 24))

        const time = timeStr.substring(0, 5) // HH:mm

        if (diffDays === 0) return `今天 ${time}`
        if (diffDays === 1) return `明天 ${time}`
        if (diffDays === 2) return `后天 ${time}`
        
        // 格式化日期 MM-DD
        const month = ('0' + (target.getMonth() + 1)).slice(-2)
        const day = ('0' + target.getDate()).slice(-2)
        return `${month}月${day}日 ${time}`
    } catch (e) {
        return `${dateStr} ${timeStr}`
    }
}
