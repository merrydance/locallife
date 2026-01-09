"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
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
const CartAPI = __importStar(require("../api/cart"));
const logger_1 = require("../utils/logger");
const global_store_1 = require("../utils/global-store");
/**
 * CartService - Backend Synchronized Version
 * Manages cart state by communicating with the backend API.
 * Acts as a centralized store for the current merchant's cart.
 */
class CartService {
    constructor() {
        // Cache the current cart to avoid excessive network requests
        this.currentCart = null;
        this.currentMerchantId = null;
    }
    static getInstance() {
        if (!CartService.instance) {
            CartService.instance = new CartService();
        }
        return CartService.instance;
    }
    /**
     * Get the current cached cart.
     * Note: This might be stale. Use refreshCart() to ensure latest data.
     */
    getCart() {
        return this.currentCart;
    }
    /**
     * Get the current merchant ID being operated on
     */
    getMerchantId() {
        return this.currentMerchantId;
    }
    /**
     * Initialize or switch to a specific merchant's cart
     */
    loadCart(merchantId) {
        return __awaiter(this, void 0, void 0, function* () {
            this.currentMerchantId = merchantId;
            return this.refreshCart();
        });
    }
    /**
     * Refresh the cart data from backend
     */
    refreshCart() {
        return __awaiter(this, void 0, void 0, function* () {
            if (!this.currentMerchantId) {
                throw new Error('No merchant selected for CartService');
            }
            try {
                logger_1.logger.debug('Refreshing cart from backend', { merchantId: this.currentMerchantId }, 'CartService.refreshCart');
                const cart = yield CartAPI.getCart({ merchant_id: this.currentMerchantId });
                this.currentCart = cart;
                this.notifyListeners();
                return cart;
            }
            catch (error) {
                logger_1.logger.error('Failed to refresh cart', error, 'CartService.refreshCart');
                throw error;
            }
        });
    }
    /**
     * Add item to backend cart
     */
    addItem(item) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const merchantId = Number(item.merchantId);
                const quantity = item.quantity || 1;
                const req = {
                    merchant_id: merchantId,
                    dish_id: item.dishId ? Number(item.dishId) : undefined,
                    combo_id: item.comboId ? Number(item.comboId) : undefined,
                    quantity: quantity,
                    customizations: item.customizations
                };
                logger_1.logger.info('Adding item to backend cart', req, 'CartService.addItem');
                const updatedCart = yield CartAPI.addToCart(req);
                // Update local state
                this.currentMerchantId = merchantId;
                this.currentCart = updatedCart;
                this.notifyListeners();
                return true;
            }
            catch (error) {
                logger_1.logger.error('Failed to add item to cart', error, 'CartService.addItem');
                // Handle simple error reporting
                wx.showToast({
                    title: '添加失败，请重试',
                    icon: 'none'
                });
                return false;
            }
        });
    }
    /**
     * Update item quantity or specs
     */
    updateItem(itemId, updates) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const updatedCart = yield CartAPI.updateCartItem(itemId, updates);
                this.currentCart = updatedCart;
                this.notifyListeners();
                return true;
            }
            catch (error) {
                logger_1.logger.error('Failed to update cart item', error, 'CartService.updateItem');
                return false;
            }
        });
    }
    /**
     * Remove item from cart
     */
    removeItem(itemId) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const updatedCart = yield CartAPI.removeFromCart(itemId);
                this.currentCart = updatedCart;
                this.notifyListeners();
                return true;
            }
            catch (error) {
                logger_1.logger.error('Failed to remove item', error, 'CartService.removeItem');
                return false;
            }
        });
    }
    /**
     * Update quantity helper
     */
    updateQuantity(itemId, quantity) {
        return __awaiter(this, void 0, void 0, function* () {
            if (quantity <= 0) {
                return this.removeItem(itemId);
            }
            return this.updateItem(itemId, { quantity });
        });
    }
    /**
     * Clear current merchant's cart
     */
    clear() {
        return __awaiter(this, void 0, void 0, function* () {
            if (!this.currentMerchantId)
                return false;
            try {
                yield CartAPI.clearCart(this.currentMerchantId);
                // Reset local state to empty structure manually or refetch
                // Refetching is safer to ensure backend state
                return this.refreshCart().then(() => true);
            }
            catch (error) {
                logger_1.logger.error('Failed to clear cart', error, 'CartService.clear');
                return false;
            }
        });
    }
    /**
     * Notify global store or event system about cart changes
     * This adapts the new API structure to the old global store format if necessary
     */
    notifyListeners() {
        if (!this.currentCart)
            return;
        // Adapt to the format expected by the frontend
        // The previous frontend might expect { totalCount, totalPrice }
        // We map the backend response to that structure
        const cartSummary = {
            items: this.currentCart.items || [],
            totalCount: this.currentCart.total_count,
            totalPrice: this.currentCart.subtotal,
            totalPriceDisplay: `¥${(this.currentCart.subtotal / 100).toFixed(2)}`
        };
        // You can use a dedicated event emitter or the global store
        // For now, we update the global store entry 'cart'
        global_store_1.globalStore.set('cart', cartSummary);
    }
}
exports.default = CartService.getInstance();
