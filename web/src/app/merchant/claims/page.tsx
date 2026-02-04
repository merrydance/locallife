import { ClaimRecoveryPageClient } from "@/components/merchant/claim-recovery-page-client";

export const metadata = {
  title: "索赔追偿 - 商家管理后台",
  description: "查看索赔追偿单状态并完成追偿支付",
};

export default function MerchantClaimRecoveryPage() {
  return <ClaimRecoveryPageClient />;
}