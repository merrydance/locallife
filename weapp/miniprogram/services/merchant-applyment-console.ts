import { buildMerchantApplymentStatusView, getMerchantApplymentStatus } from '../api/merchant-applyment'

export async function fetchMerchantApplymentStatusView() {
  const applyment = await getMerchantApplymentStatus()
  return buildMerchantApplymentStatusView(applyment)
}