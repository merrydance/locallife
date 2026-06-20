"use client";

import { useCallback, useEffect, useState } from "react";
import Image from "next/image";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Separator } from "@/components/ui/separator";
import { Textarea } from "@/components/ui/textarea";
import { PageContent, PageHeader, PageShell } from "@/components/merchant/layout/page-shell";
import { apiGet, apiPost, formatAmount } from "@/lib/api";
import { formatMerchantStatus, getActiveOperatorRegions, merchantStatusOptions } from "@/lib/operator-display";
import type {
  OperatorMerchantDetail,
  OperatorMerchantListResponse,
  OperatorMerchantStats,
} from "@/types/operator-console";
import type { OperatorRegionListResponse } from "@/types/operator-stats";

export default function OperatorMerchantsManagePage() {
  const [status, setStatus] = useState<string>("all");
  const [regions, setRegions] = useState<Array<{ id: number; name: string }>>([]);
  const [regionId, setRegionId] = useState<string>("all");
  const [data, setData] = useState<OperatorMerchantListResponse | null>(null);
  const [detail, setDetail] = useState<OperatorMerchantDetail | null>(null);
  const [stats, setStats] = useState<OperatorMerchantStats | null>(null);
  const [statsLoading, setStatsLoading] = useState(false);
  const [resumeReason, setResumeReason] = useState<string>("");
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  const loadRegions = useCallback(() => {
    apiGet<OperatorRegionListResponse>("/operator/regions", { page: 1, limit: 100 })
      .then((res) => setRegions(getActiveOperatorRegions(res.regions)))
      .catch(() => setRegions([]));
  }, []);

  const load = useCallback(() => {
    apiGet<OperatorMerchantListResponse>("/operator/merchants", {
      page: 1,
      limit: 20,
      status: status === "all" ? undefined : status,
      region_id: regionId === "all" ? undefined : Number(regionId),
    })
      .then((res) => {
        setData(res);
        setError(null);
        setMessage(null);
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : "加载失败"));
  }, [status, regionId]);

  useEffect(() => {
    loadRegions();
  }, [loadRegions]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (error) {
      toast.error(error);
    }
  }, [error]);

  useEffect(() => {
    if (message) {
      toast.success(message);
    }
  }, [message]);

  const loadDetail = (id: number) => {
    setStats(null);
    apiGet<OperatorMerchantDetail>(`/operator/merchants/${id}`)
      .then((d) => {
        setDetail(d);
        setStatsLoading(true);
        return apiGet<OperatorMerchantStats>(`/operator/merchants/${id}/stats`, { days: 30 });
      })
      .then((s) => setStats(s))
      .catch((err: unknown) => setError(err instanceof Error ? err.message : "详情加载失败"))
      .finally(() => setStatsLoading(false));
  };

  const resumeMerchant = async () => {
    if (!detail) return;
    await apiPost(`/operator/merchants/${detail.id}/resume`, {
      reason: resumeReason,
    });
    setMessage(`商户 ${detail.id} 已恢复上线`);
    setResumeReason("");
    load();
    loadDetail(detail.id);
  };

  const repurchaseRate = stats
    ? (stats.repurchase_rate_basis_points / 100).toFixed(1)
    : null;
  const avgOrdersPerUser = stats
    ? (stats.avg_orders_per_user_cents / 100).toFixed(2)
    : null;

  return (
    <PageShell>
      <PageHeader
        title="商户管理"
        description="按状态筛选并处理商户运营状态"
        actions={<Badge variant="secondary">运营商</Badge>}
      />
      <PageContent className="space-y-4">
        <Card>
          <CardHeader>
            <CardTitle>筛选条件</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-3 md:flex-row md:items-center">
            <Select value={status} onValueChange={setStatus}>
              <SelectTrigger className="w-full md:w-48">
                <SelectValue placeholder="商户状态" />
              </SelectTrigger>
              <SelectContent>
                {merchantStatusOptions.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select value={regionId} onValueChange={setRegionId}>
              <SelectTrigger className="w-full md:w-72">
                <SelectValue placeholder="选择区域" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部可管辖区域</SelectItem>
                {regions.map((region) => (
                  <SelectItem key={region.id} value={String(region.id)}>
                    {region.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button onClick={load}>重新加载</Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>商户列表</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>ID</TableHead>
                  <TableHead>商户</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>营业中</TableHead>
                  <TableHead>区域</TableHead>
                  <TableHead>操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(data?.merchants ?? []).map((item) => (
                  <TableRow
                    key={item.id}
                    className={detail?.id === item.id ? "bg-muted/50" : undefined}
                  >
                    <TableCell>{item.id}</TableCell>
                    <TableCell>{item.name}</TableCell>
                    <TableCell>{formatMerchantStatus(item.status)}</TableCell>
                    <TableCell>{item.is_open ? "是" : "否"}</TableCell>
                    <TableCell>{item.region_id}</TableCell>
                    <TableCell>
                      <Button variant="outline" size="sm" onClick={() => loadDetail(item.id)}>
                        查看详情
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
                {(!data || data.merchants.length === 0) && (
                  <TableRow>
                    <TableCell colSpan={6} className="text-muted-foreground">
                      暂无数据
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        {detail && (
          <Card>
            <CardHeader className="flex flex-row items-start gap-4">
              {detail.logo_url ? (
                <Image
                  src={detail.logo_url}
                  alt={detail.name}
                  width={64}
                  height={64}
                  className="rounded-lg object-cover"
                />
              ) : (
                <div className="flex h-16 w-16 shrink-0 items-center justify-center rounded-lg bg-muted text-2xl font-bold text-muted-foreground">
                  {detail.name.slice(0, 1)}
                </div>
              )}
              <div className="flex-1">
                <CardTitle className="text-lg">{detail.name}</CardTitle>
                <CardDescription className="mt-1 line-clamp-2">
                  {detail.description || "暂无简介"}
                </CardDescription>
              </div>
              <Badge variant={detail.is_open ? "default" : "secondary"}>
                {detail.is_open ? "营业中" : "已打烊"}
              </Badge>
            </CardHeader>

            <CardContent className="space-y-6">
              {/* 基本资料 */}
              <div>
                <div className="mb-3 text-sm font-semibold text-muted-foreground uppercase tracking-wide">
                  基本资料
                </div>
                <div className="grid gap-3 text-sm md:grid-cols-2">
                  <div className="flex gap-2">
                    <span className="text-muted-foreground w-14 shrink-0">状态</span>
                    <span>{formatMerchantStatus(detail.status)}</span>
                  </div>
                  <div className="flex gap-2">
                    <span className="text-muted-foreground w-14 shrink-0">联系电话</span>
                    <span>{detail.phone}</span>
                  </div>
                  <div className="flex gap-2">
                    <span className="text-muted-foreground w-14 shrink-0">所属区域</span>
                    <span>{detail.region_id}</span>
                  </div>
                  <div className="flex gap-2">
                    <span className="text-muted-foreground w-14 shrink-0">入驻时间</span>
                    <span>{detail.created_at}</span>
                  </div>
                  <div className="flex gap-2 md:col-span-2">
                    <span className="text-muted-foreground w-14 shrink-0">地址</span>
                    <span>{detail.address}</span>
                  </div>
                  <div className="flex gap-2 md:col-span-2">
                    <span className="text-muted-foreground w-14 shrink-0">坐标</span>
                    <span className="text-xs text-muted-foreground">
                      {detail.latitude.toFixed(6)}, {detail.longitude.toFixed(6)}
                    </span>
                  </div>
                </div>
              </div>

              <Separator />

              {/* 经营情况 */}
              <div>
                <div className="mb-3 flex items-center justify-between">
                  <div className="text-sm font-semibold text-muted-foreground uppercase tracking-wide">
                    经营情况
                  </div>
                  {stats && (
                    <Badge variant="outline" className="text-xs">
                      近 {stats.days} 天
                    </Badge>
                  )}
                </div>

                {statsLoading && (
                  <p className="text-sm text-muted-foreground">加载经营数据中...</p>
                )}

                {stats && !statsLoading && (
                  <div className="space-y-6">
                    {/* 概览指标 */}
                    <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
                      <div className="rounded-lg border p-3">
                        <div className="text-xs text-muted-foreground">总订单数</div>
                        <div className="mt-1 text-xl font-bold">{stats.total_orders}</div>
                      </div>
                      <div className="rounded-lg border p-3">
                        <div className="text-xs text-muted-foreground">GMV</div>
                        <div className="mt-1 text-xl font-bold">¥{formatAmount(stats.total_sales)}</div>
                      </div>
                      <div className="rounded-lg border p-3">
                        <div className="text-xs text-muted-foreground">平台佣金</div>
                        <div className="mt-1 text-xl font-bold">¥{formatAmount(stats.total_commission)}</div>
                      </div>
                      <div className="rounded-lg border p-3">
                        <div className="text-xs text-muted-foreground">日均销售额</div>
                        <div className="mt-1 text-xl font-bold">¥{formatAmount(stats.avg_daily_sales)}</div>
                      </div>
                    </div>

                    {/* 用户复购 */}
                    <div>
                      <div className="mb-2 text-xs font-medium text-muted-foreground">用户复购</div>
                      <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
                        <div className="rounded-lg border p-3">
                          <div className="text-xs text-muted-foreground">下单用户数</div>
                          <div className="mt-1 text-lg font-semibold">{stats.total_customers}</div>
                        </div>
                        <div className="rounded-lg border p-3">
                          <div className="text-xs text-muted-foreground">复购用户数</div>
                          <div className="mt-1 text-lg font-semibold">{stats.repeat_customers}</div>
                        </div>
                        <div className="rounded-lg border p-3">
                          <div className="text-xs text-muted-foreground">复购率</div>
                          <div className="mt-1 text-lg font-semibold">{repurchaseRate}%</div>
                        </div>
                        <div className="rounded-lg border p-3">
                          <div className="text-xs text-muted-foreground">人均下单次数</div>
                          <div className="mt-1 text-lg font-semibold">{avgOrdersPerUser} 次</div>
                        </div>
                      </div>
                    </div>

                    {/* 热销菜品 */}
                    {stats.top_dishes.length > 0 && (
                      <div>
                        <div className="mb-2 text-xs font-medium text-muted-foreground">热销菜品 Top 5</div>
                        <Table>
                          <TableHeader>
                            <TableRow>
                              <TableHead>菜品</TableHead>
                              <TableHead className="text-right">销量</TableHead>
                              <TableHead className="text-right">营业额</TableHead>
                            </TableRow>
                          </TableHeader>
                          <TableBody>
                            {stats.top_dishes.map((dish, idx) => (
                              <TableRow key={idx}>
                                <TableCell>{dish.dish_name}</TableCell>
                                <TableCell className="text-right">{dish.total_sold}</TableCell>
                                <TableCell className="text-right">¥{formatAmount(dish.total_revenue)}</TableCell>
                              </TableRow>
                            ))}
                          </TableBody>
                        </Table>
                      </div>
                    )}

                    {stats.top_dishes.length === 0 && (
                      <p className="text-sm text-muted-foreground">近 {stats.days} 天内暂无订单数据</p>
                    )}
                  </div>
                )}
              </div>

              {/* 恢复上线操作 */}
              {detail.status === "suspended" && (
                <>
                  <Separator />
                  <div className="grid gap-3 rounded-lg border border-destructive/30 p-4">
                    <div className="text-sm font-medium">恢复上线</div>
                    <Textarea
                      value={resumeReason}
                      onChange={(e) => setResumeReason(e.target.value)}
                      placeholder="恢复原因（至少5字）"
                    />
                    <Button onClick={resumeMerchant}>恢复上线</Button>
                  </div>
                </>
              )}
            </CardContent>
          </Card>
        )}
      </PageContent>
    </PageShell>
  );
}
