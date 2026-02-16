import { FinanceAccountPageClient } from "@/components/merchant/finance-account-page-client";

export const metadata = {
  title: "资金账户 - 商家管理后台",
  description: "商户收付通账户余额与提现管理",
};

export default function FinanceAccountPage() {
  return <FinanceAccountPageClient />;
}
