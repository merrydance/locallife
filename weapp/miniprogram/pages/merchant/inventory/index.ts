import { responsiveBehavior } from '../../../utils/responsive'
import { logger } from '../../../utils/logger'

Page({
    behaviors: [responsiveBehavior],
    data: {
        loading: false,
        navBarHeight: 0,
        inventoryList: [
            { id: 1, name: '五花肉 (10kg)', stock: 8, unit: 'kg', min_stock: 10, status: 'warning', category: '肉类' },
            { id: 2, name: '大红袍茶叶', stock: 50, unit: 'bag', min_stock: 20, status: 'normal', category: '茶叶' },
            { id: 3, name: '苏打水 (24瓶/箱)', stock: 2, unit: 'box', min_stock: 5, status: 'danger', category: '饮料' },
            { id: 4, name: '精选大米', stock: 15, unit: 'bag', min_stock: 10, status: 'normal', category: '粮油' }
        ],
        selectedItem: null as any,
        searchKeyword: ''
    },

    onLoad() {
        this.fetchInventory()
    },

    onNavHeight(e: any) {
        this.setData({ navBarHeight: e.detail.height })
    },

    async fetchInventory() {
        this.setData({ loading: true })
        try {
            // TODO: Call API
            // const res = await getInventoryList()
            // this.setData({ inventoryList: res.data })
        } catch (e) {
            logger.error('Fetch inventory failed', e)
        } finally {
            this.setData({ loading: false })
        }
    },

    onItemTap(e: any) {
        const { item } = e.currentTarget.dataset
        this.setData({ selectedItem: item })
    },

    onQuickAdjust(e: any) {
        const { id, delta } = e.currentTarget.dataset
        // TODO: Quick stock adjustment logic
        wx.showToast({ title: '已发起库存调整建议', icon: 'none' })
    }
})
