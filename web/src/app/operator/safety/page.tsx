"use client";

import { useCallback, useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { PageContent, PageHeader, PageShell } from "@/components/merchant/layout/page-shell";
import { apiGet, apiPost } from "@/lib/api";
import {
  formatSafetyLevel,
  formatSafetyStatus,
  safetyLevelOptions,
  safetyResolveOptions,
  safetyStatusOptions,
} from "@/lib/operator-display";
import type { SafetyReportItem, SafetyReportListResponse } from "@/types/operator-console";

export default function OperatorSafetyPage() {
  const [status, setStatus] = useState<string>("all");
  const [data, setData] = useState<SafetyReportListResponse | null>(null);
  const [detail, setDetail] = useState<SafetyReportItem | null>(null);
  const [error, setError] = useState<string | null>(null);

  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [level, setLevel] = useState("medium");
  const [merchantIds, setMerchantIds] = useState("");

  const [resolveStatus, setResolveStatus] = useState<"resolved" | "rejected">("resolved");
  const [resolutionNotes, setResolutionNotes] = useState("");

  const load = useCallback(() => {
    apiGet<SafetyReportListResponse>("/operator/reports/safety", {
      page: 1,
      limit: 20,
      status: status === "all" ? undefined : status,
    })
      .then((res) => {
        setData(res);
        setError(null);
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : "加载失败"));
  }, [status]);

  useEffect(() => {
    load();
  }, [load]);

  const submit = async () => {
    const ids = merchantIds
      .split(",")
      .map((value) => Number(value.trim()))
      .filter((value) => Number.isFinite(value) && value > 0);
    await apiPost("/operator/reports/safety", {
      title,
      description,
      level,
      merchant_ids: ids,
      images: [],
    });
    setTitle("");
    setDescription("");
    setMerchantIds("");
    load();
  };

  const loadDetail = (id: number) => {
    apiGet<SafetyReportItem>(`/operator/reports/safety/${id}`)
      .then(setDetail)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : "详情加载失败"));
  };

  const resolve = async () => {
    if (!detail) return;
    await apiPost(`/operator/reports/safety/${detail.id}/resolve`, {
      status: resolveStatus,
      resolution_notes: resolutionNotes,
      recover_merchant_ids: [],
    });
    setResolutionNotes("");
    loadDetail(detail.id);
    load();
  };

  return (
    <PageShell>
      <PageHeader
        title="食安事件"
        description="提交、筛选与处置区域食安事件"
        actions={<Badge variant="secondary">运营商</Badge>}
      />
      <PageContent className="space-y-4">
        <Card>
          <CardHeader>
            <CardTitle>提交事件</CardTitle>
          </CardHeader>
          <CardContent className="grid gap-3">
            <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="标题" />
            <Textarea value={description} onChange={(e) => setDescription(e.target.value)} placeholder="描述" />
            <div className="grid gap-3 md:grid-cols-2">
              <Select value={level} onValueChange={setLevel}>
                <SelectTrigger>
                  <SelectValue placeholder="事件等级" />
                </SelectTrigger>
                <SelectContent>
                  {safetyLevelOptions.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Input
                value={merchantIds}
                onChange={(e) => setMerchantIds(e.target.value)}
                placeholder="涉及商户ID，逗号分隔（可选）"
              />
            </div>
            <Button onClick={submit}>提交事件</Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>筛选条件</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-3 md:flex-row md:items-center">
            <Select value={status} onValueChange={setStatus}>
              <SelectTrigger className="w-full md:w-52">
                <SelectValue placeholder="状态筛选" />
              </SelectTrigger>
              <SelectContent>
                {safetyStatusOptions.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button onClick={load}>重新加载</Button>
          </CardContent>
        </Card>

        {error && (
          <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
            {error}
          </div>
        )}

        <Card>
          <CardHeader>
            <CardTitle>事件列表</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>ID</TableHead>
                  <TableHead>标题</TableHead>
                  <TableHead>等级</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(data?.items ?? []).map((item) => (
                  <TableRow key={item.id}>
                    <TableCell>{item.id}</TableCell>
                    <TableCell>{item.title}</TableCell>
                    <TableCell>{formatSafetyLevel(item.level)}</TableCell>
                    <TableCell>{formatSafetyStatus(item.status)}</TableCell>
                    <TableCell>
                      <Button variant="outline" size="sm" onClick={() => loadDetail(item.id)}>
                        详情
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
                {(!data || data.items.length === 0) && (
                  <TableRow>
                    <TableCell colSpan={5} className="text-muted-foreground">
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
              <CardTitle>事件详情 #{detail.id}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="text-sm">标题：{detail.title}</div>
              <div className="text-sm">描述：{detail.description}</div>
              <div className="text-sm">等级：{formatSafetyLevel(detail.level)}</div>
              <div className="text-sm">状态：{formatSafetyStatus(detail.status)}</div>
              {detail.status === "pending" && (
                <div className="grid gap-3 rounded-lg border p-4">
                  <Select value={resolveStatus} onValueChange={(value: "resolved" | "rejected") => setResolveStatus(value)}>
                    <SelectTrigger className="w-full md:w-48">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {safetyResolveOptions.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <Textarea
                    value={resolutionNotes}
                    onChange={(e) => setResolutionNotes(e.target.value)}
                    placeholder="处置说明"
                  />
                  <Button onClick={resolve}>提交处置</Button>
                </div>
              )}
            </CardContent>
          </Card>
        )}
      </PageContent>
    </PageShell>
  );
}
