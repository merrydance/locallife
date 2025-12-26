"use strict";
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
Object.defineProperty(exports, "__esModule", { value: true });
const responsive_1 = require("../../../utils/responsive");
const logger_1 = require("../../../utils/logger");
Page({
    behaviors: [responsive_1.responsiveBehavior],
    data: {
        loading: false,
        navBarHeight: 0,
        inventoryList: [
            { id: 1, name: '五花肉 (10kg)', stock: 8, unit: 'kg', min_stock: 10, status: 'warning', category: '肉类' },
            { id: 2, name: '大红袍茶叶', stock: 50, unit: 'bag', min_stock: 20, status: 'normal', category: '茶叶' },
            { id: 3, name: '苏打水 (24瓶/箱)', stock: 2, unit: 'box', min_stock: 5, status: 'danger', category: '饮料' },
            { id: 4, name: '精选大米', stock: 15, unit: 'bag', min_stock: 10, status: 'normal', category: '粮油' }
        ],
        selectedItem: null,
        searchKeyword: ''
    },
    onLoad() {
        this.fetchInventory();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.height });
    },
    fetchInventory() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                // TODO: Call API
                // const res = await getInventoryList()
                // this.setData({ inventoryList: res.data })
            }
            catch (e) {
                logger_1.logger.error('Fetch inventory failed', e);
            }
            finally {
                this.setData({ loading: false });
            }
        });
    },
    onItemTap(e) {
        const { item } = e.currentTarget.dataset;
        this.setData({ selectedItem: item });
    },
    onQuickAdjust(e) {
        const { id, delta } = e.currentTarget.dataset;
        // TODO: Quick stock adjustment logic
        wx.showToast({ title: '已发起库存调整建议', icon: 'none' });
    }
});
