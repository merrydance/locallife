"use client";

import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
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
import type { HourlyDistributionRow, PlatformRealtimeDashboardResponse } from "@/types/platform-stats";

export default function PlatformAuditPage() {
  const [realtime, setRealtime] = useState<PlatformRealtimeDashboardResponse | null>(null);
  const [hourly, setHourly] = useState<HourlyDistributionRow[]>([]);
  const [loadState, setLoadState] = useState<"loading" | "loaded" | "error">("loading");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const range = getRecentRange(7);
    Promise.all([
      apiGet<PlatformRealtimeDashboardResponse>("/platform/stats/realtime"),
      apiGet<HourlyDistributionRow[]>("/platform/stats/hourly", range),
    ])
      .then(([realtimeData, hourlyData]) => {
        setRealtime(realtimeData);
        setHourly(hourlyData ?? []);
        setLoadState("loaded");
      })
      .catch((err: unknown) => {
        const message = err instanceof Error ? err.message : "加载失败";
        setError(message);
        setLoadState("error");
      });
  }, []);

  useEffect(() => {
    if (error) {
      toast.error(error);
    }
  }, [error]);

  const loading = loadState === "loading";
  const statusRows = useMemo(() => {
    if (!realtime) return [];
    return [
      { label: "待接单", value: realtime.pending_orders },
      { label: "制作中", value: realtime.preparing_orders },
      { label: "待取餐", value: realtime.ready_orders },
      { label: "代取中", value: realtime.delivering_orders },
    ];
  }, [realtime]);

  return (
    <PageShell>
      <PageHeader
        title="风控审计"
        description="统一查看平台审计与风控事件"
        actions={<Badge variant="secondary">实时</Badge>}
      />
      <PageContent className="space-y-6">
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          {[
            {
              title: "24h 订单数",
              value: realtime?.orders_24h ?? 0,
              description: "最近 24 小时",
            },
            {
              title: "24h GMV",
              value: realtime?.gmv_24h ?? 0,
              description: "最近 24 小时",
              isMoney: true,
            },
            {
              title: "24h 活跃商户",
              value: realtime?.active_merchants_24h ?? 0,
              description: "最近 24 小时",
            },
            {
              title: "24h 活跃用户",
              value: realtime?.active_users_24h ?? 0,
              description: "最近 24 小时",
            },
          ].map((card, index) => (
            <Card key={`${card.title}-${index}`}>
              <CardHeader>
                <CardTitle className="text-sm font-medium text-muted-foreground">
                  {card.title}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                <div className="text-2xl font-semibold">
                  {loading
                    ? "--"
                    : card.isMoney
                    ? `¥${formatAmount(Number(card.value))}`
                    : Number(card.value).toLocaleString("zh-CN")}
                </div>
                <p className="text-xs text-muted-foreground">
                  {loading ? "获取统计中" : card.description}
                </p>
              </CardContent>
            </Card>
          ))}
        </div>

        <Card>
          <CardHeader>
            <CardTitle>订单状态分布</CardTitle>
            <CardDescription>实时订单状态分布</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>状态</TableHead>
                  <TableHead>数量</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading && (
                  <TableRow>
                    <TableCell colSpan={2} className="text-sm text-muted-foreground">
                      加载中...
                    </TableCell>
                  </TableRow>
                )}
                {!loading && statusRows.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={2} className="text-sm text-muted-foreground">
                      暂无实时数据
                    </TableCell>
                  </TableRow>
                )}
                {statusRows.map((row) => (
                  <TableRow key={row.label}>
                    <TableCell className="font-medium">{row.label}</TableCell>
                    <TableCell>{row.value}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>时段分布</CardTitle>
            <CardDescription>近 7 天订单时段分布</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>小时</TableHead>
                  <TableHead>订单数</TableHead>
                  <TableHead>GMV</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading && (
                  <TableRow>
                    <TableCell colSpan={3} className="text-sm text-muted-foreground">
                      加载中...
                    </TableCell>
                  </TableRow>
                )}
                {!loading && hourly.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={3} className="text-sm text-muted-foreground">
                      暂无时段分布数据
                    </TableCell>
                  </TableRow>
                )}
                {hourly.map((row) => (
                  <TableRow key={row.hour}>
                    <TableCell className="font-medium">{row.hour}:00</TableCell>
                    <TableCell>{row.order_count}</TableCell>
                    <TableCell>¥{formatAmount(row.total_gmv)}</TableCell>
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
