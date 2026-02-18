"use client";

import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { PageContent, PageHeader, PageShell } from "@/components/merchant/layout/page-shell";
import { apiGet } from "@/lib/api";
import type { OperatorRealtimeStatsResponse } from "@/types/operator-console";

export default function OperatorRealtimePage() {
  const [data, setData] = useState<OperatorRealtimeStatsResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiGet<OperatorRealtimeStatsResponse>("/operator/stats/realtime")
      .then(setData)
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : "加载失败");
      });
  }, []);

  const cards = [
    { label: "活跃商户", value: data?.active_merchant_count ?? 0 },
    { label: "活跃骑手", value: data?.active_rider_count ?? 0 },
    { label: "待审商户", value: data?.pending_merchant_count ?? 0 },
    { label: "待审骑手", value: data?.pending_rider_count ?? 0 },
  ];

  return (
    <PageShell>
      <PageHeader
        title="实时统计"
        description="运营区域实时运营状态"
        actions={<Badge variant="secondary">实时</Badge>}
      />
      <PageContent className="space-y-4">
        {error && (
          <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
            {error}
          </div>
        )}
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          {cards.map((card) => (
            <Card key={card.label}>
              <CardHeader>
                <CardTitle className="text-sm font-medium text-muted-foreground">{card.label}</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-semibold">{card.value}</div>
              </CardContent>
            </Card>
          ))}
        </div>
      </PageContent>
    </PageShell>
  );
}
