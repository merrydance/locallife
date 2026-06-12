"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  Activity,
  RefreshCw,
  RadioTower,
  ShieldAlert,
  TimerReset,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiGet } from "@/lib/api";
import { cn } from "@/lib/utils";
import type { PlatformTrafficSummaryResponse, PlatformTrafficRouteSummary } from "@/types/platform-stats";

const WINDOW_OPTIONS = [
  { value: 60, label: "近 1 分钟" },
  { value: 300, label: "近 5 分钟" },
  { value: 900, label: "近 15 分钟" },
  { value: 1800, label: "近 30 分钟" },
];

const ROUTE_LIMIT_OPTIONS = [10, 20, 50];

function formatBytes(value: number) {
  if (!Number.isFinite(value) || value <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let size = value;
  let unitIndex = 0;
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex += 1;
  }
  const fractionDigits = unitIndex === 0 ? 0 : size >= 10 ? 1 : 2;
  return `${size.toFixed(fractionDigits)} ${units[unitIndex]}`;
}

function formatStatusCountLabel(status: string) {
  if (status.startsWith("2")) return "2xx";
  if (status.startsWith("3")) return "3xx";
  if (status.startsWith("4")) return "4xx";
  if (status.startsWith("5")) return "5xx";
  return status;
}

function getRouteBadgeVariant(route: PlatformTrafficRouteSummary) {
  return route.error_requests > 0 ? "destructive" : "secondary";
}

function TrafficSummaryError({ hasSnapshot }: { hasSnapshot: boolean }) {
  return (
    <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
      <div className="font-medium">流量数据加载失败</div>
      <div className="mt-1 text-sm text-destructive/90">
        {hasSnapshot ? "最近一次刷新失败，已保留上次成功数据。" : "当前统计接口暂不可用，请稍后重试。"}
      </div>
    </div>
  );
}

export default function PlatformTrafficPage() {
  const [windowSeconds, setWindowSeconds] = useState(300);
  const [routeLimit, setRouteLimit] = useState(20);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [data, setData] = useState<PlatformTrafficSummaryResponse | null>(null);
  const [lastError, setLastError] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(true);
  const requestSeqRef = useRef(0);
  const visibleRequestSeqRef = useRef(0);
  const mountedRef = useRef(true);

  useEffect(() => {
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const loadTrafficSummary = useCallback(async (silent = false) => {
    const requestSeq = ++requestSeqRef.current;
    const visibleRequestSeq = silent
      ? visibleRequestSeqRef.current
      : ++visibleRequestSeqRef.current;
    if (!silent) {
      setRefreshing(true);
    }
    try {
      const summary = await apiGet<PlatformTrafficSummaryResponse>("/platform/stats/traffic/summary", {
        window_seconds: windowSeconds,
        route_limit: routeLimit,
      });
      if (!mountedRef.current || requestSeq !== requestSeqRef.current) {
        return;
      }
      setData(summary);
      setLastError(null);
    } catch (err: unknown) {
      if (!mountedRef.current || requestSeq !== requestSeqRef.current) {
        return;
      }
      const message = err instanceof Error ? err.message : "加载流量数据失败";
      setLastError(message);
    } finally {
      if (!mountedRef.current) {
        return;
      }
      if (!silent && visibleRequestSeq === visibleRequestSeqRef.current) {
        setRefreshing(false);
      }
    }
  }, [routeLimit, windowSeconds]);

  useEffect(() => {
    void loadTrafficSummary(false);
  }, [loadTrafficSummary]);

  useEffect(() => {
    if (!autoRefresh) return;
    const timer = window.setInterval(() => {
      void loadTrafficSummary(true);
    }, 5000);
    return () => window.clearInterval(timer);
  }, [autoRefresh, loadTrafficSummary]);

  const routes = data?.routes ?? [];
  const totals = data?.totals;
  const generatedAtLabel = data?.generated_at ? new Date(data.generated_at).toLocaleString("zh-CN") : "-";
  const hasSnapshot = data !== null;
  const isInitialLoading = refreshing && !hasSnapshot && !lastError;
  const errorSummary = lastError ? (
    <TrafficSummaryError hasSnapshot={hasSnapshot} />
  ) : null;

  const topRoute = routes[0];

  return (
    <PageShell>
      <PageHeader
        title="流量监控"
        description="查看平台控制台实时出口、路径排行与异常请求占比"
        actions={
          <div className="flex items-center gap-2">
            <Badge variant={autoRefresh ? "default" : "secondary"}>
              {autoRefresh ? "自动刷新中" : "手动刷新"}
            </Badge>
            <Button variant="outline" size="sm" onClick={() => setAutoRefresh((prev) => !prev)}>
              <TimerReset className="mr-2 size-4" />
              {autoRefresh ? "暂停" : "恢复"}
            </Button>
            <Button variant="outline" size="sm" onClick={() => void loadTrafficSummary(false)} disabled={refreshing}>
              <RefreshCw className={cn("mr-2 size-4", refreshing && "animate-spin")} />
              刷新
            </Button>
          </div>
        }
      />

      <PageContent className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>监控条件</CardTitle>
            <CardDescription>调整时间窗口和路由排行数量，页面会按当前窗口重新取数</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4 md:grid-cols-3">
            <div className="space-y-2">
              <div className="text-sm font-medium">统计窗口</div>
              <Select value={String(windowSeconds)} onValueChange={(value) => setWindowSeconds(Number(value))}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {WINDOW_OPTIONS.map((option) => (
                    <SelectItem key={option.value} value={String(option.value)}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <div className="text-sm font-medium">路由条数</div>
              <Select value={String(routeLimit)} onValueChange={(value) => setRouteLimit(Number(value))}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {ROUTE_LIMIT_OPTIONS.map((option) => (
                    <SelectItem key={option} value={String(option)}>
                      Top {option}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex items-end gap-3">
              <div className="space-y-2">
                <div className="text-sm font-medium">实时刷新</div>
                <div className="text-xs text-muted-foreground">每 5 秒更新一次</div>
              </div>
              <Switch checked={autoRefresh} onCheckedChange={setAutoRefresh} />
            </div>
          </CardContent>
        </Card>

        {errorSummary}

        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          {[
            { label: "请求数", value: totals?.requests ?? 0, hint: "窗口内总请求" },
            { label: "请求字节", value: totals?.request_bytes ?? 0, hint: "入站请求体" },
            { label: "响应字节", value: totals?.response_bytes ?? 0, hint: "出站响应体" },
            { label: "5xx 请求", value: totals?.error_requests ?? 0, hint: "服务端错误" },
          ].map((card) => (
            <Card key={card.label}>
              <CardHeader>
                <CardTitle className="text-sm font-medium text-muted-foreground">{card.label}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                <div className="text-2xl font-semibold tabular-nums">
                  {!hasSnapshot
                    ? "--"
                    : card.label.includes("字节")
                      ? formatBytes(card.value)
                      : Number(card.value).toLocaleString("zh-CN")}
                </div>
                <p className="text-xs text-muted-foreground">{card.hint}</p>
              </CardContent>
            </Card>
          ))}
        </div>

        <div className="grid gap-4 lg:grid-cols-[1.3fr_0.7fr]">
          <Card>
            <CardHeader>
              <CardTitle>路由排行</CardTitle>
              <CardDescription>
                按响应字节排序，帮助定位最可能吞带宽的接口
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>接口</TableHead>
                    <TableHead>请求</TableHead>
                    <TableHead>请求字节</TableHead>
                    <TableHead>响应字节</TableHead>
                    <TableHead>5xx</TableHead>
                    <TableHead>平均耗时</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {isInitialLoading && (
                    <TableRow>
                      <TableCell colSpan={6} className="text-sm text-muted-foreground">
                        加载中...
                      </TableCell>
                    </TableRow>
                  )}
                  {!isInitialLoading && routes.length === 0 && (
                    <TableRow>
                      <TableCell colSpan={6} className="text-sm text-muted-foreground">
                        {hasSnapshot ? "当前窗口内暂无请求" : "当前数据暂不可用"}
                      </TableCell>
                    </TableRow>
                  )}
                  {hasSnapshot && routes.map((route) => (
                    <TableRow key={`${route.method}-${route.path}`}>
                      <TableCell>
                        <div className="space-y-1">
                          <div className="flex items-center gap-2">
                            <Badge variant={getRouteBadgeVariant(route)}>{route.method}</Badge>
                            <span className="font-medium">{route.path}</span>
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>{route.requests.toLocaleString("zh-CN")}</TableCell>
                      <TableCell>{formatBytes(route.request_bytes)}</TableCell>
                      <TableCell>{formatBytes(route.response_bytes)}</TableCell>
                      <TableCell>{route.error_requests.toLocaleString("zh-CN")}</TableCell>
                      <TableCell>{route.average_latency_ms.toFixed(1)} ms</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>

          <div className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>Top 路由状态码</CardTitle>
                <CardDescription>展示响应字节最高路由的状态码分布</CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                {isInitialLoading && <div className="text-sm text-muted-foreground">加载中...</div>}
                {!isInitialLoading && !hasSnapshot && (
                  <div className="text-sm text-muted-foreground">没有可展示的路由状态。</div>
                )}
                {hasSnapshot && topRoute && (
                  <>
                    <div className="flex items-center justify-between gap-3">
                      <div>
                        <div className="text-sm font-medium">{topRoute.path}</div>
                        <div className="text-xs text-muted-foreground">{topRoute.method}</div>
                      </div>
                      <Badge variant="outline">响应字节 Top 1</Badge>
                    </div>
                    <Separator />
                    <div className="space-y-2">
                      {Object.entries(topRoute.status_counts)
                        .sort(([a], [b]) => Number(a) - Number(b))
                        .map(([status, count]) => (
                          <div key={status} className="flex items-center justify-between text-sm">
                            <span>{formatStatusCountLabel(status)}</span>
                            <span className="font-medium">{count.toLocaleString("zh-CN")}</span>
                          </div>
                        ))}
                    </div>
                  </>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>最近窗口</CardTitle>
                <CardDescription>当前统计窗口内的聚合视图</CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="flex items-center gap-3 text-sm">
                  <Activity className="size-4 text-muted-foreground" />
                  <span>窗口：{windowSeconds} 秒</span>
                </div>
                <div className="flex items-center gap-3 text-sm">
                  <ShieldAlert className="size-4 text-muted-foreground" />
                  <span>异常请求：{hasSnapshot ? totals?.error_requests ?? 0 : "--"}</span>
                </div>
                <div className="flex items-center gap-3 text-sm">
                  <RadioTower className="size-4 text-muted-foreground" />
                  <span>路由排行：Top {routeLimit}</span>
                </div>
                <div className="flex items-center gap-3 text-sm">
                  <Activity className="size-4 text-muted-foreground" />
                  <span>最近刷新：{generatedAtLabel}</span>
                </div>
              </CardContent>
            </Card>
          </div>
        </div>
      </PageContent>
    </PageShell>
  );
}
