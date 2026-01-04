/**
 * 包间详情页面
 */

import { getRoomDetail, RoomDetailResponse } from '../../../api/room'
import { getPublicImageUrl } from '../../../utils/image'

Page({
    data: {
        roomId: 0,
        room: null as RoomDetailResponse | null,
        loading: true,
        navBarHeight: 88
    },

    onLoad(options: any) {
        const roomId = parseInt(options.id)
        if (!roomId) {
            wx.showToast({ title: '包间ID缺失', icon: 'error' })
            setTimeout(() => wx.navigateBack(), 1500)
            return
        }
        this.setData({ roomId })
        this.loadRoomDetail()
    },

    onNavHeight(e: WechatMiniprogram.CustomEvent) {
        this.setData({ navBarHeight: e.detail.navBarHeight })
    },

    async loadRoomDetail() {
        this.setData({ loading: true })

        try {
            const room = await getRoomDetail(this.data.roomId)

            // 处理图片URL
            const processedRoom = {
                ...room,
                primary_image: room.primary_image ? getPublicImageUrl(room.primary_image) : '',
                images: (room.images || []).map((url: string) => getPublicImageUrl(url)),
                merchant_logo: room.merchant_logo ? getPublicImageUrl(room.merchant_logo) : ''
            }

            this.setData({
                room: processedRoom,
                loading: false
            })
        } catch (error) {
            console.error('加载包间详情失败:', error)
            wx.showToast({ title: '加载失败', icon: 'error' })
            this.setData({ loading: false })
        }
    },

    onPreviewImage(e: WechatMiniprogram.CustomEvent) {
        const index = e.currentTarget.dataset.index
        const room = this.data.room
        if (!room) return

        const images = room.images || []
        const urls: string[] = images.length > 0 ? images : (room.primary_image ? [room.primary_image] : [])
        if (urls.length > 0) {
            wx.previewImage({
                current: urls[index] || urls[0],
                urls
            })
        }
    },

    onMerchantTap() {
        const room = this.data.room
        if (room?.merchant_id) {
            wx.navigateTo({
                url: `/pages/takeout/restaurant-detail/index?id=${room.merchant_id}`
            })
        }
    },

    onCallMerchant() {
        const room = this.data.room
        if (room?.merchant_phone) {
            wx.makePhoneCall({ phoneNumber: room.merchant_phone })
        } else {
            wx.showToast({ title: '暂无联系电话', icon: 'none' })
        }
    },

    onNavigate() {
        const room = this.data.room
        if (room?.merchant_latitude && room?.merchant_longitude) {
            wx.openLocation({
                latitude: room.merchant_latitude,
                longitude: room.merchant_longitude,
                name: room.merchant_name,
                address: room.merchant_address || ''
            })
        } else {
            wx.showToast({ title: '暂无位置信息', icon: 'none' })
        }
    },

    onReserve() {
        // TODO: 跳转到预订页面
        wx.showToast({
            title: '预订功能开发中',
            icon: 'none'
        })
    }
})
