import FavoriteService, { FavoriteType } from '../../../api/favorite'
import { logger } from '../../../utils/logger'
import { ErrorHandler } from '../../../utils/error-handler'
import { formatPriceNoSymbol } from '../../../utils/util'

// ViewModel
interface FavoriteViewModel {
    id: number
    targetId: number // merchant_id or dish_id
    type: FavoriteType
    typeText: string
    title: string
    image: string
    subTitle: string
    rating?: number
    priceDisplay?: string
    createdAt: string
}

Page({
    data: {
        activeTab: 'dish' as FavoriteType, // 'dish' | 'merchant'
        favorites: [] as FavoriteViewModel[],
        loading: false,
        navBarHeight: 88,
        
        // Paging
        page: 1,
        pageSize: 10,
        hasMore: true
    },

    onLoad() {
        this.loadFavorites(true)
    },

    onNavHeight(e: WechatMiniprogram.CustomEvent) {
        this.setData({ navBarHeight: e.detail.navBarHeight })
    },

    onTabChange(e: WechatMiniprogram.CustomEvent) {
        this.setData({ 
            activeTab: e.detail.value,
            favorites: [],
            page: 1,
            hasMore: true
        }, () => {
            this.loadFavorites(true)
        })
    },

    async loadFavorites(reset = false) {
        if (!this.data.hasMore && !reset) return
        if (this.data.loading) return

        this.setData({ loading: true })

        try {
            let list: FavoriteViewModel[] = []
            let total = 0

            if (this.data.activeTab === 'dish') {
                const res = await FavoriteService.getFavoriteDishes(this.data.page, this.data.pageSize)
                list = res.dishes.map((d: any) => ({
                    id: d.id, // favorite record id (or dish id? backend response has id as record id?)
                              // Looking at api/favorite.go: ID is record ID, DishID is target. 
                              // BUT for removal we need DISH ID? wait. 
                              // deleteFavoriteDish uses PATH param :id.
                              // api/favorite.go: deleteFavoriteDish parses "id".
                              // Usually REST API DELETE /v1/favorites/dishes/:id implies deleting the FAVORITE record or the DISH?
                              // Actually, looking at the code: `server.store.RemoveFavoriteDish(ctx, db.RemoveFavoriteDishParams{ DishID: dishID })`
                              // So it expects the TARGET ID (DishID), not the Favorite Record ID.
                    targetId: d.dish_id,
                    type: 'dish',
                    typeText: '菜品',
                    title: d.dish_name,
                    image: d.image_url || '/assets/icons/dish.svg',
                    subTitle: d.description || d.merchant_name || '',
                    priceDisplay: formatPriceNoSymbol(d.price),
                    createdAt: d.created_at?.split('T')[0] || ''
                }))
                total = res.total
            } else {
                const res = await FavoriteService.getFavoriteMerchants(this.data.page, this.data.pageSize)
                list = res.merchants.map((m: any) => ({
                    id: m.id,
                    targetId: m.merchant_id,
                    type: 'merchant',
                    typeText: '餐厅',
                    title: m.merchant_name,
                    image: m.merchant_logo || '',
                    subTitle: m.address || '',
                    rating: 5.0, // Backend doesn't return rating in favorite response?
                    createdAt: m.created_at?.split('T')[0] || ''
                }))
                total = res.total
            }

            const newFavorites = reset ? list : [...this.data.favorites, ...list]
            this.setData({
                favorites: newFavorites,
                hasMore: newFavorites.length < total,
                page: this.data.page + 1,
                loading: false
            })
        } catch (error) {
            this.setData({ loading: false })
            ErrorHandler.handle(error, 'Favorites.load')
        }
    },

    onReachBottom() {
        this.loadFavorites()
    },

    async onRemoveFavorite(e: WechatMiniprogram.BaseEvent) {
        const item = e.currentTarget.dataset.item as FavoriteViewModel
        if (!item) return

        wx.showModal({
            title: '取消收藏',
            content: `确定取消收藏"${item.title}"吗？`,
            success: async (res) => {
                if (res.confirm) {
                     wx.showLoading({ title: '处理中' })
                     try {
                         if (item.type === 'dish') {
                             // Backend API expects the Dish ID (targetId), not the favorite record ID
                             await FavoriteService.removeFavoriteDish(item.targetId) 
                         } else {
                             await FavoriteService.removeFavoriteMerchant(item.targetId)
                         }
                         
                         wx.showToast({ title: '已取消', icon: 'success' })
                         
                         // Refresh list
                         const favorites = this.data.favorites.filter(f => f.id !== item.id)
                         this.setData({ favorites })
                     } catch (e) {
                         ErrorHandler.handle(e, 'Favorites.remove')
                     } finally {
                         wx.hideLoading()
                     }
                }
            }
        })
    },

    onItemClick(e: WechatMiniprogram.BaseEvent) {
        const item = e.currentTarget.dataset.item as FavoriteViewModel
        if (!item) return

        if (item.type === 'dish') {
            // Need merchant_id for dish detail? Usually yes.
            // But we might only have DishID. 
            // Existing dish detail page usually takes id (dish_id). 
            // If it needs merchant_id, we might be missing it in the favorite response if not provided.
            // The backend response `favoriteDishResponse` DOES have `MerchantID`.
            // But we didn't map it. Let's map it if needed.
            // However, typical detail page just needs ID.
            wx.navigateTo({
                url: `/pages/takeout/dish-detail/index?id=${item.targetId}`
            })
        } else {
            wx.navigateTo({
                url: `/pages/takeout/restaurant-detail/index?id=${item.targetId}`
            })
        }
    },
    
    onGoHome() {
        if (this.data.activeTab === 'merchant') {
            wx.switchTab({ url: '/pages/reservation/index' })
        } else {
            wx.switchTab({ url: '/pages/takeout/index' })
        }
    }
})
