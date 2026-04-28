import {
  getMyMerchantOpenStatus,
  getMyMerchantProfile,
  updateMyMerchantOpenStatus,
  type MerchantOperatorResponse
} from '../api/merchant'

export type MerchantStorefrontProfile = MerchantOperatorResponse

export function fetchMerchantStorefrontProfile() {
  return getMyMerchantProfile()
}

export function fetchMerchantStorefrontOpenStatus() {
  return getMyMerchantOpenStatus()
}

export function updateMerchantStorefrontOpenStatus(nextIsOpen: boolean) {
  return updateMyMerchantOpenStatus(nextIsOpen)
}