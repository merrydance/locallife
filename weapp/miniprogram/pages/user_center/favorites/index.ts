import FavoriteService, { FavoriteMerchantListItem, FavoriteType } from './_api/favorite'
import ConsumerProfileAdapter from './_main_shared/adapters/consumer-profile'
import { ErrorHandler } from '../../../utils/error-handler'
import Navigation from '../../../utils/navigation'
import { formatPriceNoSymbol } from '../../../utils/util'

type FavoriteDishItem = {
    id: number
    dish_id: number
    dish_name: string
    image_url?: string
    description?: string
    merchant_name?: string
    price: number
    created_at?: string
}

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
    isOrderingSuspended?: boolean
}

Page({
    data: {
        activeTab: 'dish' as FavoriteType, // 'dish' | 'merchant'
        favorites: [] as FavoriteViewModel[],
        navBarHeight: 88,
        loading: false,
        initialLoading: true,
        error: null as string | null,
        
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
        if (this.data.loading && !this.data.initialLoading) return

        this.setData({ loading: true, error: null })

        try {
            let list: FavoriteViewModel[] = []
            let total = 0

            if (this.data.activeTab === 'dish') {
                const res = await FavoriteService.getFavoriteDishes(this.data.page, this.data.pageSize)
                list = res.dishes.map((dish) => {
                    const d = dish as unknown as FavoriteDishItem
                    return ({
                    id: d.id,
                    targetId: d.dish_id,
                    type: 'dish',
                    typeText: '菜品',
                    title: d.dish_name,
                    image: d.image_url || '/assets/icons/dish.svg',
                    subTitle: d.description || d.merchant_name || '',
                    priceDisplay: formatPriceNoSymbol(d.price),
                    createdAt: d.created_at?.split('T')[0] || ''
                    })
                })
                total = res.total
            } else {
                const res = await FavoriteService.getFavoriteMerchants(this.data.page, this.data.pageSize)
                list = res.merchants.map((merchant) => {
                    const m = merchant as FavoriteMerchantListItem
                    const viewModel = ConsumerProfileAdapter.toFavoriteMerchantViewModel(m)
                    return ({
                    ...viewModel,
                    type: 'merchant',
                    typeText: '餐厅'
                    })
                })
                total = res.total
            }

            const newFavorites = reset ? list : [...this.data.favorites, ...list]
            this.setData({
                favorites: newFavorites,
                hasMore: newFavorites.length < total,
                page: this.data.page + 1,
                loading: false,
                initialLoading: false
            })
        } catch (error) {
            this.setData({ 
                loading: false,
                initialLoading: false,
                error: '加载收藏列表失败'
            })
            ErrorHandler.handle(error, 'Favorites.load')
        }
    },

    onRetry() {
        this.loadFavorites(true)
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

                         // Refresh list
                         const favorites = this.data.favorites.filter((f) => f.id !== item.id)
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
            Navigation.toDishDetail(String(item.targetId))
        } else {
            Navigation.toRestaurantDetail(item.targetId)
        }
    },
    
    onGoHome() {
        if (this.data.activeTab === 'merchant') {
            Navigation.toReservationHome()
        } else {
            Navigation.toTakeoutHome()
        }
    }
})
