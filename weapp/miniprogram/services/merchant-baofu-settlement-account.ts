import { buildBaofuSettlementAccountView } from '../api/baofu-account-view'
import { getMerchantBaofuSettlementAccount } from '../api/baofu-account'

export async function fetchMerchantBaofuSettlementAccountView() {
  const account = await getMerchantBaofuSettlementAccount()
  return buildBaofuSettlementAccountView(account)
}
