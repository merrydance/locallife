import { AnalyticsTabs } from "@/components/merchant/analytics-tabs";
import { Badge } from "@/components/ui/badge";
import { PageShell, PageHeader, PageContent } from "@/components/merchant/layout/page-shell";

export default function AnalyticsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <PageShell>
      <PageHeader
        title="数据分析"
        description="查看店铺经营数据与趋势分析"
        actions={<Badge variant="secondary">经营看板</Badge>}
      >
        <div className="mt-4 w-full border-t pt-4">
          <AnalyticsTabs />
        </div>
      </PageHeader>
      <PageContent className="space-y-6">
        {children}
      </PageContent>
    </PageShell>
  );
}
