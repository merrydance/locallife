import AddressService, { type Address } from '../api/address'
import { getMyMemberships } from '../api/personal'
import { formatPriceNoSymbol } from '../utils/util'

export type CheckoutAddress = Address

export interface TakeoutMembershipState {
  memberBalances: Record<number, number>
  memberBalanceDisplays: Record<number, string>
  membershipIds: Record<number, number>
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
  const memberBalanceDisplays: Record<number, string> = {}
  const membershipIds: Record<number, number> = {}

  merchantIds.forEach((merchantId) => {
    const membership = result.memberships?.find((item) => item.merchant_id === merchantId)
    if (!membership) {
      return
    }

    memberBalances[merchantId] = membership.balance
    memberBalanceDisplays[merchantId] = formatPriceNoSymbol(membership.balance)
    membershipIds[merchantId] = membership.id
  })

  return {
    memberBalances,
    memberBalanceDisplays,
    membershipIds
  }
}