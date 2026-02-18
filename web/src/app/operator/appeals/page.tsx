"use client";

import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { PageContent, PageHeader, PageShell } from "@/components/merchant/layout/page-shell";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { apiGet, apiPost, formatAmount } from "@/lib/api";
import {
  appealReviewOptions,
  appealStatusOptions,
  formatAppealStatus,
  formatClaimStatus,
  formatOrderStatus,
  formatRecoveryStatus,
} from "@/lib/operator-display";
import type {
  ClaimRecoveryResponse,
  OperatorAppealDetail,
  OperatorAppealListResponse,
} from "@/types/operator-console";

export default function OperatorAppealsPage() {
  const [status, setStatus] = useState<string>("all");
  const [data, setData] = useState<OperatorAppealListResponse | null>(null);
  const [detail, setDetail] = useState<OperatorAppealDetail | null>(null);
  const [recovery, setRecovery] = useState<ClaimRecoveryResponse | null>(null);
  const [reviewStatus, setReviewStatus] = useState<"approved" | "rejected">("approved");
  const [reviewNotes, setReviewNotes] = useState<string>("");
  const [compensationAmount, setCompensationAmount] = useState<string>("");
  const [error, setError] = useState<string | null>(null);
  const [reviewSubmitting, setReviewSubmitting] = useState(false);
  const [waiveSubmitting, setWaiveSubmitting] = useState(false);
  const [reviewConfirmOpen, setReviewConfirmOpen] = useState(false);
  const [waiveConfirmOpen, setWaiveConfirmOpen] = useState(false);

  const load = useCallback(() => {
    apiGet<OperatorAppealListResponse>("/operator/appeals", {
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
    if (error) {
      toast.error(error);
    }
  }, [error]);

  const loadDetail = (id: number) => {
    apiGet<OperatorAppealDetail>(`/operator/appeals/${id}`)
      .then((res) => {
        setDetail(res);
        setRecovery(null);
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : "详情加载失败"));
  };

  const loadRecovery = () => {
    if (!detail) return;
    apiGet<ClaimRecoveryResponse>(`/operator/claims/${detail.claim_id}/recovery`)
      .then(setRecovery)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : "追偿单加载失败"));
  };

  const submitReview = async () => {
    if (!detail) return;

    const compensationValue = Number(compensationAmount || 0);
    if (reviewStatus === "approved" && (!Number.isFinite(compensationValue) || compensationValue <= 0)) {
      setError("申诉通过时必须填写大于0的补偿金额（分）");
      return;
    }

    setReviewSubmitting(true);
    try {
      await apiPost(`/operator/appeals/${detail.id}/review`, {
        status: reviewStatus,
        review_notes: reviewNotes,
        compensation_amount: reviewStatus === "approved" ? compensationValue : undefined,
      });
      setReviewNotes("");
      setCompensationAmount("");
      load();
      loadDetail(detail.id);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "审核提交失败");
    } finally {
      setReviewSubmitting(false);
    }
  };

  const waiveRecovery = async () => {
    if (!detail) return;
    setWaiveSubmitting(true);
    try {
      await apiPost(`/operator/claims/${detail.claim_id}/recovery/waive`, {});
      loadRecovery();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "核销失败");
    } finally {
      setWaiveSubmitting(false);
    }
  };

  return (
    <PageShell>
      <PageHeader
        title="申诉处理"
        description="审核申诉并处理追偿流程"
        actions={<Badge variant="secondary">运营商</Badge>}
      />
      <PageContent className="space-y-4">
        <Card>
          <CardHeader>
            <CardTitle>筛选条件</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-3 md:flex-row md:items-center">
            <Select value={status} onValueChange={setStatus}>
              <SelectTrigger className="w-full md:w-52">
                <SelectValue placeholder="申诉状态" />
              </SelectTrigger>
              <SelectContent>
                {appealStatusOptions.map((option) => (
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
            <CardTitle>申诉列表</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>ID</TableHead>
                  <TableHead>索赔ID</TableHead>
                  <TableHead>申诉方</TableHead>
                  <TableHead>金额</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(data?.appeals ?? []).map((item) => (
                  <TableRow key={item.id}>
                    <TableCell>{item.id}</TableCell>
                    <TableCell>{item.claim_id}</TableCell>
                    <TableCell>{item.appellant_name}</TableCell>
                    <TableCell>¥{formatAmount(item.claim_amount)}</TableCell>
                    <TableCell>{formatAppealStatus(item.status)}</TableCell>
                    <TableCell>
                      <Button variant="outline" size="sm" onClick={() => loadDetail(item.id)}>
                        详情
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
                {(!data || data.appeals.length === 0) && (
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
              <CardTitle>申诉详情 #{detail.id}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-2 text-sm md:grid-cols-2">
                <div>索赔ID：{detail.claim_id}</div>
                <div>申诉状态：{formatAppealStatus(detail.status)}</div>
                <div>申诉方：{detail.appellant_type}</div>
                <div>申诉金额：¥{formatAmount(detail.claim_amount)}</div>
                <div>索赔状态：{formatClaimStatus(detail.claim_status)}</div>
                <div>订单状态：{formatOrderStatus(detail.order_status)}</div>
                <div>订单号：{detail.order_no}</div>
                <div>商户：{detail.merchant_name}</div>
                <div>商户电话：{detail.merchant_phone || "-"}</div>
                <div>用户：{detail.user_name || "-"}</div>
                <div>用户电话：{detail.user_phone || "-"}</div>
                <div>索赔创建：{new Date(detail.claim_created_at).toLocaleString()}</div>
                <div>申诉创建：{new Date(detail.created_at).toLocaleString()}</div>
                <div>订单创建：{new Date(detail.order_created_at).toLocaleString()}</div>
                <div>区域ID：{detail.region_id}</div>
                {typeof detail.claim_approved_amount === "number" && (
                  <div>索赔核准金额：¥{formatAmount(detail.claim_approved_amount)}</div>
                )}
                {typeof detail.compensation_amount === "number" && (
                  <div>申诉补偿金额：¥{formatAmount(detail.compensation_amount)}</div>
                )}
                {typeof detail.reviewer_id === "number" && <div>审核人ID：{detail.reviewer_id}</div>}
                {detail.reviewed_at && <div>审核时间：{new Date(detail.reviewed_at).toLocaleString()}</div>}
                {detail.lookback_result && <div className="md:col-span-2">回溯结果：{detail.lookback_result}</div>}
                {detail.review_notes && <div className="md:col-span-2">审核意见：{detail.review_notes}</div>}
                <div className="md:col-span-2">原因：{detail.reason}</div>
              </div>

              {detail.status === "pending" && (
                <div className="grid gap-3 rounded-lg border p-4">
                  <div className="text-sm font-medium">审核操作</div>
                  <Select value={reviewStatus} onValueChange={(v: "approved" | "rejected") => setReviewStatus(v)}>
                    <SelectTrigger className="w-full md:w-48">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {appealReviewOptions.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  {reviewStatus === "approved" && (
                    <Input
                      value={compensationAmount}
                      onChange={(e) => setCompensationAmount(e.target.value)}
                      placeholder="补偿金额（分）"
                    />
                  )}
                  <Textarea
                    value={reviewNotes}
                    onChange={(e) => setReviewNotes(e.target.value)}
                    placeholder="审核意见（至少5字）"
                  />
                  <Button onClick={() => setReviewConfirmOpen(true)} disabled={reviewSubmitting}>
                    {reviewSubmitting ? "提交中" : "提交审核"}
                  </Button>
                </div>
              )}

              <div className="flex flex-wrap gap-2">
                <Button variant="outline" onClick={loadRecovery}>
                  查询追偿单
                </Button>
                {recovery && recovery.status !== "waived" && (
                  <Button variant="destructive" onClick={() => setWaiveConfirmOpen(true)} disabled={waiveSubmitting}>
                    {waiveSubmitting ? "提交中" : "核销追偿"}
                  </Button>
                )}
              </div>

              {recovery && (
                <div className="grid gap-2 rounded-lg border p-4 text-sm md:grid-cols-2">
                  <div>追偿单ID：{recovery.id}</div>
                  <div>状态：{formatRecoveryStatus(recovery.status)}</div>
                  <div>追偿方：{recovery.responsible_party}</div>
                  <div>金额：¥{formatAmount(recovery.recovery_amount)}</div>
                </div>
              )}
            </CardContent>
          </Card>
        )}

        <ConfirmDialog
          open={reviewConfirmOpen}
          onOpenChange={setReviewConfirmOpen}
          title="确认提交审核？"
          description={
            reviewStatus === "approved"
              ? `将按补偿金额 ${compensationAmount || "0"} 分通过申诉并触发后续处理。`
              : "将驳回该申诉并触发后续处理。"
          }
          confirmText="确认提交"
          onConfirm={submitReview}
        />

        <ConfirmDialog
          open={waiveConfirmOpen}
          onOpenChange={setWaiveConfirmOpen}
          title="确认核销追偿？"
          description="核销后将恢复对应限制，该操作会记录审计日志。"
          confirmText="确认核销"
          variant="destructive"
          onConfirm={waiveRecovery}
        />
      </PageContent>
    </PageShell>
  );
}
