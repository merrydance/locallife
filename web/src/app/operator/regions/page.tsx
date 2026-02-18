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
import type {
  OperatorDailyTrendRow,
  OperatorRegionListResponse,
  OperatorRegionStatsResponse,
} from "@/types/operator-stats";

export default function OperatorRegionsPage() {
  const [regionStats, setRegionStats] = useState<OperatorRegionStatsResponse | null>(null);
  const [trend, setTrend] = useState<OperatorDailyTrendRow[]>([]);
  const [loadState, setLoadState] = useState<"loading" | "loaded" | "error">("loading");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const range = getRecentRange(14);
    Promise.allSettled([
      apiGet<OperatorRegionListResponse>("/operator/regions", { page: 1, limit: 1 }),
      apiGet<OperatorDailyTrendRow[]>("/operator/trend/daily", range),
    ]).then(async ([regionListResult, trendResult]) => {
      const errors: string[] = [];

      if (trendResult.status === "fulfilled") {
        setTrend(trendResult.value ?? []);
      } else {
        setTrend([]);
        errors.push("趋势数据暂不可用");
      }

      if (regionListResult.status === "fulfilled") {
        const regionId = regionListResult.value.regions?.[0]?.id;
        if (regionId) {
          const statsResult = await Promise.allSettled([
            apiGet<OperatorRegionStatsResponse>(`/operator/regions/${regionId}/stats`, range),
          ]);
          if (statsResult[0].status === "fulfilled") {
            setRegionStats(statsResult[0].value);
          } else {
            setRegionStats(null);
            errors.push("区域统计暂不可用");
          }
        } else {
          setRegionStats(null);
          errors.push("未找到区域信息");
        }
      } else {
        setRegionStats(null);
        errors.push("区域信息暂不可用");
      }

      setError(errors.length > 0 ? errors.join("；") : null);
      setLoadState("loaded");
      });
  }, []);

  useEffect(() => {
    if (error) {
      toast.error(error);
    }
  }, [error]);

  const summaryCards = useMemo(() => {
    if (!regionStats)
      return [
        { title: "商户数", value: "--", description: "区域统计暂不可用" },
        { title: "订单数", value: "--", description: "区域统计暂不可用" },
        { title: "GMV", value: "--", description: "区域统计暂不可用" },
        { title: "平台佣金", value: "--", description: "区域统计暂不可用" },
      ] as Array<{ title: string; value: string; description: string }>;
    return [
      {
        title: "商户数",
        value: regionStats.merchant_count.toLocaleString("zh-CN"),
        description: regionStats.region_name,
      },
      {
        title: "订单数",
        value: regionStats.total_orders.toLocaleString("zh-CN"),
        description: "近 14 天",
      },
      {
        title: "GMV",
        value: `¥${formatAmount(regionStats.total_gmv)}`,
        description: "近 14 天",
      },
      {
        title: "平台佣金",
        value: `¥${formatAmount(regionStats.total_commission)}`,
        description: "近 14 天",
      },
    ];
  }, [regionStats]);

  const loading = loadState === "loading";

  return (
    <PageShell>
      <PageHeader
        title="区域趋势"
        description="区域经营指标与日趋势"
        actions={<Badge variant="secondary">近 14 天</Badge>}
      />
      <PageContent className="space-y-4">
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          {(loading
            ? Array.from({ length: 4 }, (_, idx) => ({
                title: `loading-${idx}`,
                value: "--",
                description: "获取统计中",
              }))
            : summaryCards
          ).map((card) => (
            <Card key={card.title}>
              <CardHeader>
                <CardTitle className="text-sm font-medium text-muted-foreground">
                  {loading ? "加载中" : card.title}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                <div className="text-2xl font-semibold">
                  {loading ? "--" : card.value}
                </div>
                <p className="text-sm text-muted-foreground">
                  {loading ? "获取统计中" : card.description}
                </p>
              </CardContent>
            </Card>
          ))}
        </div>

        <Card>
          <CardHeader>
            <CardTitle>每日趋势</CardTitle>
            <CardDescription>订单与运营商收入趋势</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>日期</TableHead>
                  <TableHead>订单数</TableHead>
                  <TableHead>GMV</TableHead>
                  <TableHead>运营商收入</TableHead>
                  <TableHead>活跃用户</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading && (
                  <TableRow>
                    <TableCell colSpan={5} className="text-sm text-muted-foreground">
                      加载中...
                    </TableCell>
                  </TableRow>
                )}
                {!loading && trend.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={5} className="text-sm text-muted-foreground">
                      暂无趋势数据
                    </TableCell>
                  </TableRow>
                )}
                {trend.map((row) => (
                  <TableRow key={row.date}>
                    <TableCell className="font-medium">{row.date}</TableCell>
                    <TableCell>{row.order_count}</TableCell>
                    <TableCell>¥{formatAmount(row.total_gmv)}</TableCell>
                    <TableCell>¥{formatAmount(row.operator_income)}</TableCell>
                    <TableCell>{row.active_users}</TableCell>
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
