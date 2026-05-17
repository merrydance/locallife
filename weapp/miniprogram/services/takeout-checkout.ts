import AddressService, { type Address } from '../api/address'
import { getMyMemberships } from '../api/personal'
import { getPaymentCapabilities } from '../api/payment'

export type CheckoutAddress = Address

export interface TakeoutMembershipState {
  memberBalances: Record<number, number>
  membershipIds: Record<number, number>
}

export interface CheckoutPaymentCapabilities {
  mainBusinessPaymentChannel: string
  combinedPaymentSupported: boolean
  splitCheckoutRequired: boolean
  splitCheckoutNotice: string
}

const SPLIT_CHECKOUT_UNAVAILABLE_NOTICE = '暂不支持合单支付，请一次选择一家商户下单。'

export function getDefaultCheckoutAddress() {
  return AddressService.getDefaultAddress()
}

export function getCheckoutAddressDetail(addressId: number) {
  return AddressService.getAddressDetail(addressId)
}

export async function loadTakeoutMembershipState(merchantIds: number[]): Promise<TakeoutMembershipState> {
  const result = await getMyMemberships()
  const memberBalances: Record<number, number> = {}
  const membershipIds: Record<number, number> = {}

  merchantIds.forEach((merchantId) => {
    const membership = result.memberships?.find((item) => item.merchant_id === merchantId)
    if (!membership) {
      return
    }

    memberBalances[merchantId] = membership.balance
    membershipIds[merchantId] = membership.id
  })

  return {
    memberBalances,
    membershipIds
  }
}

export async function loadCheckoutPaymentCapabilities(): Promise<CheckoutPaymentCapabilities> {
  const capabilities = await getPaymentCapabilities()
  const splitCheckoutRequired = !!capabilities.split_checkout_required

  return {
    mainBusinessPaymentChannel: capabilities.main_business_payment_channel,
    combinedPaymentSupported: !!capabilities.combined_payment_supported,
    splitCheckoutRequired,
    splitCheckoutNotice: splitCheckoutRequired ? SPLIT_CHECKOUT_UNAVAILABLE_NOTICE : ''
  }
}
