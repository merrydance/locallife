import { buildBaofuSettlementAccountView } from '../_main_shared/api/baofu-account-view'
import { getMerchantBaofuSettlementAccount } from '../_main_shared/api/baofu-account'

export async function fetchMerchantBaofuSettlementAccountView() {
  const account = await getMerchantBaofuSettlementAccount()
  return buildBaofuSettlementAccountView(account)
}
