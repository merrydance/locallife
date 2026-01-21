import { AnalyticsTabs } from "@/components/merchant/analytics-tabs";
import { Badge } from "@/components/ui/badge";

export default function AnalyticsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <>
      <header className="page-header flex-col items-start gap-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <h1 className="text-xl font-semibold">数据分析</h1>
            <Badge variant="secondary">经营看板</Badge>
          </div>
          <div className="text-xs text-muted-foreground">数据来源 /v1/merchant/stats/*</div>
        </div>
        <AnalyticsTabs />
      </header>
      <main className="page-content">{children}</main>
    </>
  );
}
