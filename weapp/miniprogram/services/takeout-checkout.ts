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
    splitCheckoutNotice: splitCheckoutRequired
      ? (capabilities.combined_payment_unavailable_message || '当前支付通道需按商户分别下单支付')
      : ''
  }
}
