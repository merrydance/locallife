import { ClaimDetailPageClient } from "@/components/merchant/claim-detail-page-client";

export const metadata = {
  title: "索赔详情 - 商家管理后台",
  description: "查看索赔与追偿单详情",
};

export default function MerchantClaimDetailPage({
  params,
}: {
  params: { id: string };
}) {
  return <ClaimDetailPageClient claimId={params.id} />;
}