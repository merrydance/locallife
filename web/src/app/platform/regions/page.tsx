"use client";

import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  PageContent,
  PageHeader,
  PageShell,
} from "@/components/merchant/layout/page-shell";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiGet, formatAmount, getRecentRange } from "@/lib/api";
import type { RegionComparisonRow } from "@/types/platform-stats";

export default function PlatformRegionsPage() {
  const [rows, setRows] = useState<RegionComparisonRow[]>([]);
  const [loadState, setLoadState] = useState<"loading" | "loaded" | "error">("loading");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const range = getRecentRange(30);
    apiGet<RegionComparisonRow[]>("/platform/stats/regions/compare", range)
      .then((data) => {
        setRows(data ?? []);
        setLoadState("loaded");
      })
      .catch((err: unknown) => {
        const message = err instanceof Error ? err.message : "加载失败";
        setError(message);
        setLoadState("error");
      })
  }, []);

  const loading = loadState === "loading";

  return (
    <PageShell>
      <PageHeader
        title="跨区县运营监控"
        description="多区县运营健康度与异常趋势"
        actions={<Badge variant="outline">实时</Badge>}
      />
      <PageContent>
        {error && (
          <div className="mb-4 rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
            {error}
          </div>
        )}
        <Card>
          <CardHeader>
            <CardTitle>区域概览</CardTitle>
            <CardDescription>按区县聚合 GMV、订单量与活跃度</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>区域</TableHead>
                  <TableHead>商户数</TableHead>
                  <TableHead>订单量</TableHead>
                  <TableHead>GMV</TableHead>
                  <TableHead>平台佣金</TableHead>
                  <TableHead>活跃用户</TableHead>
                  <TableHead>平均客单</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading && (
                  <TableRow>
                    <TableCell colSpan={8} className="text-sm text-muted-foreground">
                      加载中...
                    </TableCell>
                  </TableRow>
                )}
                {!loading && rows.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={8} className="text-sm text-muted-foreground">
                      暂无区域统计数据
                    </TableCell>
                  </TableRow>
                )}
                {rows.map((row) => (
                  <TableRow key={row.region_id}>
                    <TableCell className="font-medium">{row.region_name}</TableCell>
                    <TableCell>{row.merchant_count}</TableCell>
                    <TableCell>{row.order_count}</TableCell>
                    <TableCell>¥{formatAmount(row.total_gmv)}</TableCell>
                    <TableCell>¥{formatAmount(row.total_commission)}</TableCell>
                    <TableCell>{row.active_users}</TableCell>
                    <TableCell>¥{formatAmount(row.avg_order_amount)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </PageContent>
    </PageShell>
  );
}
