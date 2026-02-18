"use client";

import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { PageContent, PageHeader, PageShell } from "@/components/merchant/layout/page-shell";
import { apiGet, formatAmount } from "@/lib/api";
import { formatRiderStatus, riderStatusOptions } from "@/lib/operator-display";
import type { OperatorRiderDetail, OperatorRiderListResponse } from "@/types/operator-console";
import type { OperatorRegionListResponse } from "@/types/operator-stats";

export default function OperatorRidersManagePage() {
  const [status, setStatus] = useState<string>("all");
  const [regions, setRegions] = useState<Array<{ id: number; name: string }>>([]);
  const [regionId, setRegionId] = useState<string>("all");
  const [data, setData] = useState<OperatorRiderListResponse | null>(null);
  const [detail, setDetail] = useState<OperatorRiderDetail | null>(null);
  const [error, setError] = useState<string | null>(null);

  const loadRegions = useCallback(() => {
    apiGet<OperatorRegionListResponse>("/operator/regions", { page: 1, limit: 100 })
      .then((res) => setRegions(res.regions ?? []))
      .catch(() => setRegions([]));
  }, []);

  const load = useCallback(() => {
    apiGet<OperatorRiderListResponse>("/operator/riders", {
      page: 1,
      limit: 20,
      status: status === "all" ? undefined : status,
      region_id: regionId === "all" ? undefined : Number(regionId),
    })
      .then((res) => {
        setData(res);
        setError(null);
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

  const loadDetail = (id: number) => {
    apiGet<OperatorRiderDetail>(`/operator/riders/${id}`)
      .then(setDetail)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : "详情加载失败"));
  };

  return (
    <PageShell>
      <PageHeader
        title="骑手管理"
        description="按状态筛选并查看骑手履约信息"
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
                <SelectValue placeholder="骑手状态" />
              </SelectTrigger>
              <SelectContent>
                {riderStatusOptions.map((option) => (
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
            <CardTitle>骑手列表</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>ID</TableHead>
                  <TableHead>姓名</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>在线</TableHead>
                  <TableHead>总收入</TableHead>
                  <TableHead>操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(data?.riders ?? []).map((item) => (
                  <TableRow key={item.id}>
                    <TableCell>{item.id}</TableCell>
                    <TableCell>{item.real_name}</TableCell>
                    <TableCell>{formatRiderStatus(item.status)}</TableCell>
                    <TableCell>{item.is_online ? "是" : "否"}</TableCell>
                    <TableCell>¥{formatAmount(item.total_earnings)}</TableCell>
                    <TableCell>
                      <Button variant="outline" size="sm" onClick={() => loadDetail(item.id)}>
                        详情
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
                {(!data || data.riders.length === 0) && (
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
            <CardHeader>
              <CardTitle>骑手详情 #{detail.id}</CardTitle>
            </CardHeader>
            <CardContent className="grid gap-2 text-sm md:grid-cols-2">
              <div>姓名：{detail.real_name}</div>
              <div>电话：{detail.phone}</div>
              <div>状态：{formatRiderStatus(detail.status)}</div>
              <div>信用分：{detail.credit_score}</div>
              <div>高价值单资格：{detail.high_value_qualified ? "是" : "否"}</div>
              <div>总单量：{detail.total_orders}</div>
            </CardContent>
          </Card>
        )}
      </PageContent>
    </PageShell>
  );
}
