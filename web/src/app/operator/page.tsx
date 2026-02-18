"use client";

import { useEffect, useMemo, useState } from "react";
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
import type { OperatorRealtimeStatsResponse } from "@/types/operator-console";
import type { OperatorDailyTrendRow, OperatorFinanceOverviewResponse } from "@/types/operator-stats";

export default function OperatorDashboardPage() {
  const [finance, setFinance] = useState<OperatorFinanceOverviewResponse | null>(null);
  const [realtime, setRealtime] = useState<OperatorRealtimeStatsResponse | null>(null);
  const [trend, setTrend] = useState<OperatorDailyTrendRow[]>([]);
  const [loadState, setLoadState] = useState<"loading" | "loaded" | "error">("loading");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const range = getRecentRange(14);
    Promise.allSettled([
      apiGet<OperatorFinanceOverviewResponse>("/operators/me/finance/overview"),
      apiGet<OperatorDailyTrendRow[]>("/operator/trend/daily", range),
      apiGet<OperatorRealtimeStatsResponse>("/operator/stats/realtime"),
    ])
      .then(([financeResult, trendResult, realtimeResult]) => {
        const errors: string[] = [];

        if (financeResult.status === "fulfilled") {
          setFinance(financeResult.value);
        } else {
          setFinance(null);
          errors.push("财务概览暂不可用");
        }

        if (trendResult.status === "fulfilled") {
          setTrend(trendResult.value ?? []);
        } else {
          setTrend([]);
          errors.push("趋势数据暂不可用");
        }

        if (realtimeResult.status === "fulfilled") {
          setRealtime(realtimeResult.value);
        } else {
          setRealtime(null);
        }

        setError(errors.length > 0 ? errors.join("；") : null);
        setLoadState("loaded");
      });
  }, []);

  const summaryCards = useMemo(() => {
    if (!finance)
      return [
        { title: "当月 GMV", value: "--", description: "财务数据暂不可用" },
        { title: "当月运营商收入", value: "--", description: "财务数据暂不可用" },
        { title: "当月订单数", value: "--", description: "财务数据暂不可用" },
        { title: "累计结算平台佣金", value: "--", description: "财务数据暂不可用" },
      ] as Array<{ title: string; value: string; description: string }>;
    return [
      {
        title: "当月 GMV",
        value: `¥${formatAmount(finance.current_month.total_gmv)}`,
        description: `${finance.region_name} · 当月统计`,
      },
      {
        title: "当月运营商收入",
        value: `¥${formatAmount(finance.current_month.operator_income)}`,
        description: `分成比例 ${(finance.operator_share_ratio * 100).toFixed(0)}%`,
      },
      {
        title: "当月订单数",
        value: finance.current_month.total_orders.toLocaleString("zh-CN"),
        description: "分账成功订单",
      },
      {
        title: "累计结算平台佣金",
        value: `¥${formatAmount(finance.total.settled_commission)}`,
        description: "历史累计", 
      },
    ];
  }, [finance]);

  const loading = loadState === "loading";
  const realtimeCards = [
    { label: "活跃商户", value: realtime?.active_merchant_count ?? 0 },
    { label: "活跃骑手", value: realtime?.active_rider_count ?? 0 },
    { label: "待审商户", value: realtime?.pending_merchant_count ?? 0 },
    { label: "待审骑手", value: realtime?.pending_rider_count ?? 0 },
  ];

  return (
    <PageShell>
      <PageHeader
        title="运营商控制台总览"
        description="区域经营与分账收入概览"
        actions={<Badge variant="secondary">运营商</Badge>}
      />
      <PageContent className="space-y-4">
        {error && (
          <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
            {error}
          </div>
        )}

        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          {realtimeCards.map((card) => (
            <Card key={card.label}>
              <CardHeader>
                <CardTitle className="text-sm font-medium text-muted-foreground">{card.label}</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-semibold">{loading ? "--" : card.value}</div>
              </CardContent>
            </Card>
          ))}
        </div>

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
            <CardTitle>近 14 日趋势</CardTitle>
            <CardDescription>订单、GMV 与运营商收入趋势</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>日期</TableHead>
                  <TableHead>订单数</TableHead>
                  <TableHead>GMV</TableHead>
                  <TableHead>运营商收入</TableHead>
                  <TableHead>活跃商户</TableHead>
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
                    <TableCell>{row.active_merchants}</TableCell>
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
