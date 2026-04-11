"use client";

import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { PageContent, PageHeader, PageShell } from "@/components/merchant/layout/page-shell";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { apiGet, apiPost } from "@/lib/api";
import {
  formatSafetyLevel,
  formatSafetyStatus,
  safetyLevelOptions,
  safetyResolveOptions,
  safetyStatusOptions,
} from "@/lib/operator-display";
import type {
  OperatorMerchantListResponse,
  SafetyReportItem,
  SafetyReportListResponse,
} from "@/types/operator-console";

export default function OperatorSafetyPage() {
  const [status, setStatus] = useState<string>("all");
  const [data, setData] = useState<SafetyReportListResponse | null>(null);
  const [detail, setDetail] = useState<SafetyReportItem | null>(null);
  const [error, setError] = useState<string | null>(null);

  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [level, setLevel] = useState("medium");
  const [selectedMerchantId, setSelectedMerchantId] = useState<string>("none");
  const [merchantOptions, setMerchantOptions] = useState<Array<{ id: number; name: string }>>([]);

  const [resolveStatus, setResolveStatus] = useState<"resolved" | "rejected">("resolved");
  const [resolutionNotes, setResolutionNotes] = useState("");
  const [recoverMerchantId, setRecoverMerchantId] = useState<string>("none");
  const [recoverReason, setRecoverReason] = useState("");
  const [submittingReport, setSubmittingReport] = useState(false);
  const [resolvingReport, setResolvingReport] = useState(false);
  const [reportDialogOpen, setReportDialogOpen] = useState(false);
  const [submitConfirmOpen, setSubmitConfirmOpen] = useState(false);
  const [resolveConfirmOpen, setResolveConfirmOpen] = useState(false);

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

  useEffect(() => {
    apiGet<OperatorMerchantListResponse>("/operator/merchants", { page: 1, limit: 200 })
      .then((res) => {
        const merchants = (res.merchants ?? []).map((item) => ({ id: item.id, name: item.name }));
        setMerchantOptions(merchants);
      })
      .catch(() => setMerchantOptions([]));
  }, []);

  useEffect(() => {
    if (error) {
      toast.error(error);
    }
  }, [error]);

  const submit = async () => {
    const selectedID = Number(selectedMerchantId);
    const ids = selectedMerchantId !== "none" && Number.isFinite(selectedID) && selectedID > 0 ? [selectedID] : [];
    setSubmittingReport(true);
    try {
      await apiPost("/operator/reports/safety", {
        title,
        description,
        level,
        merchant_ids: ids,
        images: [],
      });
      setTitle("");
      setDescription("");
      setSelectedMerchantId("none");
      setReportDialogOpen(false);
      load();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "提交失败");
    } finally {
      setSubmittingReport(false);
    }
  };

  const loadDetail = (id: number) => {
    apiGet<SafetyReportItem>(`/operator/reports/safety/${id}`)
      .then(setDetail)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : "详情加载失败"));
  };

  const resolve = async () => {
    if (!detail) return;

    const selectedID = Number(recoverMerchantId);
    const ids = recoverMerchantId !== "none" && Number.isFinite(selectedID) && selectedID > 0 ? [selectedID] : [];

    if (ids.length > 0 && recoverReason.trim().length < 2) {
      setError("填写恢复商户时必须提供恢复原因（至少2个字符）");
      return;
    }

    setResolvingReport(true);
    try {
      await apiPost(`/operator/reports/safety/${detail.id}/resolve`, {
        status: resolveStatus,
        resolution_notes: resolutionNotes,
        recover_merchant_ids: ids,
        recover_reason: ids.length > 0 ? recoverReason.trim() : undefined,
      });
      setResolutionNotes("");
      setRecoverMerchantId("none");
      setRecoverReason("");
      loadDetail(detail.id);
      load();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "处置失败");
    } finally {
      setResolvingReport(false);
    }
  };

  return (
    <PageShell>
      <PageHeader
        title="食安事件"
        description="提交、筛选与处置区域食安事件"
        actions={
          <div className="flex items-center gap-2">
            <Badge variant="secondary">运营商</Badge>
            <Button size="sm" onClick={() => setReportDialogOpen(true)}>
              手动上报
            </Button>
          </div>
        }
      />
      <PageContent className="space-y-4">
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
              <div className="text-sm">上报人：{detail.reporter_id}</div>
              <div className="text-sm">涉及商户：{detail.merchant_ids.length > 0 ? detail.merchant_ids.join(", ") : "-"}</div>
              <div className="text-sm">创建时间：{new Date(detail.created_at).toLocaleString()}</div>
              <div className="text-sm">更新时间：{new Date(detail.updated_at).toLocaleString()}</div>
              {detail.resolution_notes && <div className="text-sm">处置说明：{detail.resolution_notes}</div>}
              {detail.images.length > 0 && <div className="text-sm">图片数量：{detail.images.length}</div>}
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
                  <Select value={recoverMerchantId} onValueChange={setRecoverMerchantId}>
                    <SelectTrigger>
                      <SelectValue placeholder="选择恢复商户（可选）" />
                    </SelectTrigger>
                    <SelectContent className="max-h-72">
                      <SelectItem value="none">不恢复指定商户</SelectItem>
                      {merchantOptions.map((merchant) => (
                        <SelectItem key={merchant.id} value={String(merchant.id)}>
                          {merchant.name}（{merchant.id}）
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <Textarea
                    value={recoverReason}
                    onChange={(e) => setRecoverReason(e.target.value)}
                    placeholder="恢复原因（填写恢复商户时必填）"
                  />
                  <Button onClick={() => setResolveConfirmOpen(true)} disabled={resolvingReport}>
                    {resolvingReport ? "提交中" : "提交处置"}
                  </Button>
                </div>
              )}
            </CardContent>
          </Card>
        )}

        <Dialog open={reportDialogOpen} onOpenChange={setReportDialogOpen}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>手动上报食安事件</DialogTitle>
              <DialogDescription>低频操作，按需填写后提交。</DialogDescription>
            </DialogHeader>
            <div className="grid gap-3">
              <Textarea value={title} onChange={(e) => setTitle(e.target.value)} placeholder="标题（至少5字）" />
              <Textarea value={description} onChange={(e) => setDescription(e.target.value)} placeholder="描述（至少10字）" />
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
              <Select value={selectedMerchantId} onValueChange={setSelectedMerchantId}>
                <SelectTrigger>
                  <SelectValue placeholder="选择商户（可选）" />
                </SelectTrigger>
                <SelectContent className="max-h-72">
                  <SelectItem value="none">不关联指定商户</SelectItem>
                  {merchantOptions.map((merchant) => (
                    <SelectItem key={merchant.id} value={String(merchant.id)}>
                      {merchant.name}（{merchant.id}）
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setReportDialogOpen(false)}>
                取消
              </Button>
              <Button onClick={() => setSubmitConfirmOpen(true)} disabled={submittingReport}>
                {submittingReport ? "提交中" : "提交事件"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <ConfirmDialog
          open={submitConfirmOpen}
          onOpenChange={setSubmitConfirmOpen}
          title="确认提交食安事件？"
          description="提交后将进入区域食安事件流转，关键操作会记录审计日志。"
          confirmText="确认提交"
          onConfirm={submit}
        />

        <ConfirmDialog
          open={resolveConfirmOpen}
          onOpenChange={setResolveConfirmOpen}
          title="确认提交处置结果？"
          description="处置结果生效后将更新事件状态，并可能恢复指定商户。"
          confirmText="确认处置"
          onConfirm={resolve}
        />
      </PageContent>
    </PageShell>
  );
}
