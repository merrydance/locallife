import { AnalyticsPageClient } from "@/components/merchant/analytics-page-client";

export const metadata = {
  title: "经营分析 - 商家管理后台",
  description: "全方位的经营数据透视，助力商业决策优化",
};

export default function AnalyticsPage() {
  return <AnalyticsPageClient />;
}
