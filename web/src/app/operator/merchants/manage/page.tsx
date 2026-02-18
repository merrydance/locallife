"use client";

import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { PageContent, PageHeader, PageShell } from "@/components/merchant/layout/page-shell";
import { apiGet, apiPost } from "@/lib/api";
import { formatMerchantStatus, merchantStatusOptions } from "@/lib/operator-display";
import type { OperatorMerchantDetail, OperatorMerchantListResponse } from "@/types/operator-console";
import type { OperatorRegionListResponse } from "@/types/operator-stats";

export default function OperatorMerchantsManagePage() {
  const [status, setStatus] = useState<string>("all");
  const [regions, setRegions] = useState<Array<{ id: number; name: string }>>([]);
  const [regionId, setRegionId] = useState<string>("all");
  const [data, setData] = useState<OperatorMerchantListResponse | null>(null);
  const [detail, setDetail] = useState<OperatorMerchantDetail | null>(null);
  const [resumeReason, setResumeReason] = useState<string>("");
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  const loadRegions = useCallback(() => {
    apiGet<OperatorRegionListResponse>("/operator/regions", { page: 1, limit: 100 })
      .then((res) => setRegions(res.regions ?? []))
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
    apiGet<OperatorMerchantDetail>(`/operator/merchants/${id}`)
      .then(setDetail)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : "详情加载失败"));
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
                  <TableRow key={item.id}>
                    <TableCell>{item.id}</TableCell>
                    <TableCell>{item.name}</TableCell>
                    <TableCell>{formatMerchantStatus(item.status)}</TableCell>
                    <TableCell>{item.is_open ? "是" : "否"}</TableCell>
                    <TableCell>{item.region_id}</TableCell>
                    <TableCell>
                      <Button variant="outline" size="sm" onClick={() => loadDetail(item.id)}>
                        详情
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
            <CardHeader>
              <CardTitle>商户详情 #{detail.id}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-2 text-sm md:grid-cols-2">
                <div>名称：{detail.name}</div>
                <div>电话：{detail.phone}</div>
                <div>状态：{formatMerchantStatus(detail.status)}</div>
                <div>区域：{detail.region_id}</div>
                <div className="md:col-span-2">地址：{detail.address}</div>
                <div className="md:col-span-2">简介：{detail.description || "-"}</div>
              </div>
              {detail.status === "suspended" && (
                <div className="grid gap-3 rounded-lg border p-4">
                  <div className="text-sm font-medium">恢复上线</div>
                  <Textarea
                    value={resumeReason}
                    onChange={(e) => setResumeReason(e.target.value)}
                    placeholder="恢复原因（至少5字）"
                  />
                  <Button onClick={resumeMerchant}>恢复上线</Button>
                </div>
              )}
            </CardContent>
          </Card>
        )}
      </PageContent>
    </PageShell>
  );
}
