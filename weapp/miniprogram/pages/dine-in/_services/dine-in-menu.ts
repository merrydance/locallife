import {
  addToCart,
  calculateCart,
  getCart,
  removeFromCart,
  updateCartItem,
  type CartItemResponse,
  type CartResponse
} from '../../../api/cart'
import { type CustomizationGroup } from '../_main_shared/api/dish'
import { getDiningSessionMenu } from '../_main_shared/api/dining-session'
import { getMerchantDishes, getPublicMerchantDetail, type PublicDish } from '../../../api/merchant'
import { getReservationDetail } from '../_main_shared/api/reservation'
import type {
  ScanTableCategoryInfo,
  ScanTableComboInfo,
  ScanTableDishInfo,
  ScanTableMerchantInfo,
  ScanTablePromotionInfo,
  ScanTableTableInfo
} from '../_main_shared/api/table'

export type MenuCart = CartResponse
export type MenuCartItem = CartItemResponse
export type MenuCustomizationGroup = CustomizationGroup
export type MenuPublicDish = PublicDish
export type MenuCategoryInfo = ScanTableCategoryInfo
export type MenuDishInfo = ScanTableDishInfo
export type MenuMerchantInfo = ScanTableMerchantInfo
export type MenuTableInfo = ScanTableTableInfo
export type MenuComboInfo = ScanTableComboInfo
export type MenuPromotionInfo = ScanTablePromotionInfo

export function loadDineInSessionMenu(sessionId: number) {
  return getDiningSessionMenu(sessionId)
}

export async function loadReservationMenuSource(reservationId: number, merchantId: number) {
  const [reservation, merchantDetail, dishesResponse] = await Promise.all([
    getReservationDetail(reservationId),
    getPublicMerchantDetail(merchantId),
    getMerchantDishes(merchantId)
  ])

  return {
    reservation,
    merchantDetail,
    dishesResponse
  }
}

export function getMenuCart(params: Parameters<typeof getCart>[0]) {
  return getCart(params)
}

export function addMenuItemToCart(payload: Parameters<typeof addToCart>[0]) {
  return addToCart(payload)
}

export function updateMenuCartItem(itemId: number, payload: Parameters<typeof updateCartItem>[1], options?: Parameters<typeof updateCartItem>[2]) {
  return updateCartItem(itemId, payload, options)
}

export function removeMenuCartItem(itemId: number, options?: Parameters<typeof removeFromCart>[1]) {
  return removeFromCart(itemId, options)
}

export function calculateMenuCart(params: Parameters<typeof calculateCart>[0], options?: Parameters<typeof calculateCart>[1]) {
  return calculateCart(params, options)
}