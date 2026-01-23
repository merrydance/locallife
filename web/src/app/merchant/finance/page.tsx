import { FinancePageClient } from "@/components/merchant/finance-page-client";

export const metadata = {
  title: "财务管理 - 商家管理后台",
  description: "查看财务明细、服务费和结算记录",
};

export default function FinancePage() {
  return <FinancePageClient />;
}
