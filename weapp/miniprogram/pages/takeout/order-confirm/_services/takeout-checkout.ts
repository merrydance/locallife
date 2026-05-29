import AddressService, { type Address } from '../_main_shared/api/address'
import { getMyMemberships } from '../../../../api/personal'

export type CheckoutAddress = Address

export interface TakeoutMembershipState {
  memberBalances: Record<number, number>
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
