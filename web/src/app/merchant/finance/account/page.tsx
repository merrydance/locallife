import { FinanceAccountPageClient } from "@/components/merchant/finance-account-page-client";

export const metadata = {
  title: "结算账户 - 商家管理后台",
  description: "普通服务商进件、结算账户与资金操作指引",
};

export default function FinanceAccountPage() {
  return <FinanceAccountPageClient />;
}
