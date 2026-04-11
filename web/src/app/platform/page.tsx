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
import { Separator } from "@/components/ui/separator";
import { apiGet, formatAmount, formatGrowthRate, getGrowthColor, getRecentRange } from "@/lib/api";
import type { PlatformDailyStatRow, PlatformOverviewResponse, RuleSummary } from "@/types/platform-stats";

export default function PlatformDashboardPage() {
  const [overview, setOverview] = useState<PlatformOverviewResponse | null>(null);
  const [dailyStats, setDailyStats] = useState<PlatformDailyStatRow[]>([]);
  const [rules, setRules] = useState<RuleSummary[]>([]);
  const [rulesWarning, setRulesWarning] = useState<string | null>(null);
  const [loadState, setLoadState] = useState<"loading" | "loaded" | "error">("loading");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const range = getRecentRange(30);
    Promise.all([
      apiGet<PlatformOverviewResponse>("/platform/stats/overview", range),
      apiGet<PlatformDailyStatRow[]>("/platform/stats/daily", range),
    ])
      .then(([overviewData, dailyData]) => {
        setOverview(overviewData);
        setDailyStats(dailyData ?? []);
        setLoadState("loaded");
      })
      .catch((err: unknown) => {
        const message = err instanceof Error ? err.message : "加载失败";
        setError(message);
        setLoadState("error");
      });

    apiGet<{ rules: RuleSummary[] }>("/platform/rules", { limit: 6, offset: 0 })
      .then((rulesData) => {
        setRules(rulesData?.rules ?? []);
      })
      .catch((err: unknown) => {
        const message = err instanceof Error ? err.message : "规则数据加载失败";
        setRulesWarning(message);
        setRules([]);
      });
  }, []);

  useEffect(() => {
    if (error) {
      toast.error(error);
    }
  }, [error]);

  const orderGrowth = useMemo(() => {
    if (dailyStats.length < 2) return null;
    const last = dailyStats[dailyStats.length - 1]?.order_count ?? 0;
    const prev = dailyStats[dailyStats.length - 2]?.order_count ?? 0;
    if (prev === 0) return null;
    return (last - prev) / prev;
  }, [dailyStats]);

  const summaryCards = useMemo(() => {
    if (!overview)
      return [] as Array<{
        title: string;
        value: string;
        description: string;
        valueClassName?: string;
      }>;
    return [
      {
        title: "订单总量",
        value: overview.total_orders.toLocaleString("zh-CN"),
        description: orderGrowth === null ? "近 30 天" : `较昨日 ${formatGrowthRate(orderGrowth)}`,
        valueClassName: orderGrowth === null ? undefined : getGrowthColor(orderGrowth),
      },
      {
        title: "GMV",
        value: `¥${formatAmount(overview.total_gmv)}`,
        description: "近 30 天",
      },
      {
        title: "平台佣金",
        value: `¥${formatAmount(overview.total_commission)}`,
        description: "近 30 天",
      },
      {
        title: "活跃商户",
        value: overview.active_merchants.toLocaleString("zh-CN"),
        description: `活跃用户 ${overview.active_users.toLocaleString("zh-CN")}`,
      },
    ];
  }, [overview, orderGrowth]);

  const loading = loadState === "loading";

  return (
    <PageShell>
      <PageHeader
        title="平台控制台总览"
        description="风控、分账与跨区县运营统一看板"
        actions={<Badge variant="secondary">平台端</Badge>}
      />
      <PageContent className="space-y-8">
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          {(
            loading
              ? (Array.from({ length: 4 }, (_, idx) => ({
                  title: `loading-${idx}`,
                  value: "--",
                  description: "获取统计中",
                })) as Array<{
                  title: string;
                  value: string;
                  description: string;
                  valueClassName?: string;
                }>)
              : summaryCards
          ).map((card) => (
            <Card key={card.title}>
              <CardHeader>
                <CardTitle className="text-sm font-medium text-muted-foreground">
                  {loading ? "加载中" : card.title}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                <div className={`text-2xl font-semibold ${loading ? "text-muted-foreground" : card.valueClassName ?? ""}`}>
                  {loading ? "--" : card.value}
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
            <CardTitle>规则变更动态</CardTitle>
            <CardDescription>
              展示最近发布与回滚的规则版本
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {rulesWarning && (
              <div className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700">
                规则中心数据暂不可用：{rulesWarning}
              </div>
            )}
            {rules.length === 0 && !loading ? (
              <div className="text-sm text-muted-foreground">暂无规则变更记录</div>
            ) : (
              rules.map((item, index) => (
                <div key={item.id}>
                  <div className="flex flex-wrap items-center justify-between gap-4">
                    <div>
                      <div className="font-medium">{item.name}</div>
                      <div className="text-xs text-muted-foreground">
                        {item.category} · {new Date(item.updated_at).toLocaleString("zh-CN")}
                      </div>
                    </div>
                    <Badge
                      variant={item.status === "active" ? "default" : "secondary"}
                    >
                      {item.status === "active" ? "已生效" : item.status}
                    </Badge>
                  </div>
                  {index !== rules.length - 1 && <Separator className="my-4" />}
                </div>
              ))
            )}
          </CardContent>
        </Card>
      </PageContent>
    </PageShell>
  );
}
