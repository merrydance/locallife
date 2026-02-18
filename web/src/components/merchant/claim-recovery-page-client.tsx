"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { PageContent, PageHeader, PageShell } from "@/components/merchant/layout/page-shell";
import { apiGet, apiPost, formatAmount } from "@/lib/api";
import {
  formatClaimType,
  formatDecisionCompensationSource,
  formatDecisionReasonCode,
  formatDecisionResponsibleParty,
  formatDecisionStatus,
} from "@/lib/claim-display";
import {
  formatAppealStatus,
  formatClaimStatus,
  formatRecoveryStatus,
} from "@/lib/operator-display";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import type {
  ClaimRecoveryResponse,
  MerchantClaimBehaviorSummaryResponse,
  MerchantClaimDecision,
  MerchantClaimDecisionResponse,
  MerchantClaimItem,
  MerchantClaimsResponse,
} from "@/types/claim-recovery";

const CLAIM_STATUS_OPTIONS = [
  { value: "all", label: "全部索赔" },
  { value: "approved", label: "已批准" },
  { value: "auto-approved", label: "自动批准" },
  { value: "manual-review", label: "人工复核" },
];

const RECOVERY_STATUS_OPTIONS = [
  { value: "all", label: "全部追偿" },
  { value: "pending", label: "待追偿" },
  { value: "overdue", label: "已逾期" },
  { value: "paid", label: "已支付" },
  { value: "waived", label: "已核销" },
  { value: "appealed", label: "申诉中" },
];

const APPEAL_STATUS_OPTIONS = [
  { value: "all", label: "全部申诉" },
  { value: "pending", label: "申诉待处理" },
  { value: "approved", label: "申诉通过" },
  { value: "rejected", label: "申诉驳回" },
];

const RECOVERY_STATUS_STYLES: Record<string, string> = {
  pending: "bg-amber-100 text-amber-700 border-amber-200",
  overdue: "bg-rose-100 text-rose-700 border-rose-200",
  paid: "bg-emerald-100 text-emerald-700 border-emerald-200",
  waived: "bg-slate-100 text-slate-700 border-slate-200",
  appealed: "bg-blue-100 text-blue-700 border-blue-200",
};

function TableSkeleton({ rows = 5 }: { rows?: number }) {
  return (
    <>
      {Array.from({ length: rows }).map((_, idx) => (
        <TableRow key={idx}>
          {Array.from({ length: 10 }).map((__, col) => (
            <TableCell key={col}>
              <Skeleton className="h-4 w-20" />
            </TableCell>
          ))}
        </TableRow>
      ))}
    </>
  );
}

function formatDateTime(value?: string) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

export function ClaimRecoveryPageClient() {
  const [loading, setLoading] = useState(true);
  const [claims, setClaims] = useState<MerchantClaimItem[]>([]);
  const [recoveryMap, setRecoveryMap] = useState<Record<number, ClaimRecoveryResponse | null>>({});
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [claimStatus, setClaimStatus] = useState("all");
  const [recoveryStatus, setRecoveryStatus] = useState("all");
  const [appealStatus, setAppealStatus] = useState("all");
  const [paying, setPaying] = useState<Record<number, boolean>>({});
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const [batchPaying, setBatchPaying] = useState(false);
  const [decisionOpen, setDecisionOpen] = useState(false);
  const [decisionLoading, setDecisionLoading] = useState(false);
  const [activeClaim, setActiveClaim] = useState<MerchantClaimItem | null>(null);
  const [activeDecision, setActiveDecision] = useState<MerchantClaimDecision | null>(null);
  const [activeSummary, setActiveSummary] = useState<MerchantClaimBehaviorSummaryResponse | null>(null);

  const pagination = useMemo(() => ({ page_id: page, page_size: 10 }), [page]);

  const loadClaims = useCallback(async () => {
    setLoading(true);
    try {
      const data = await apiGet<MerchantClaimsResponse>("/merchant/claims", pagination);
      const items = data.claims || [];
      setClaims(items);
      const total = data.total_count || data.total || items.length;
      setTotalPages(Math.max(1, Math.ceil(total / (data.page_size || 10))));

      const recoveryEntries = await Promise.all(
        items.map(async (item) => {
          try {
            const recovery = await apiGet<ClaimRecoveryResponse>(
              `/merchant/claims/${item.id}/recovery`
            );
            return [item.id, recovery] as const;
          } catch {
            return [item.id, null] as const;
          }
        })
      );

      const nextMap: Record<number, ClaimRecoveryResponse | null> = {};
      recoveryEntries.forEach(([id, recovery]) => {
        nextMap[id] = recovery;
      });
      setRecoveryMap(nextMap);
      setSelectedIds((prev) => prev.filter((id) => items.some((item) => item.id === id)));
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "加载索赔失败";
      toast.error(message);
    } finally {
      setLoading(false);
    }
  }, [pagination]);

  useEffect(() => {
    loadClaims();
  }, [loadClaims]);

  const filteredClaims = useMemo(() => {
    return claims.filter((claim) => {
      const recovery = recoveryMap[claim.id];
      if (claimStatus !== "all" && claim.status !== claimStatus) {
        return false;
      }
      if (recoveryStatus !== "all") {
        return recovery?.status === recoveryStatus;
      }
      if (appealStatus !== "all") {
        return claim.appeal_status === appealStatus;
      }
      return true;
    });
  }, [claims, recoveryMap, claimStatus, recoveryStatus, appealStatus]);

  const handlePayRecovery = async (claimId: number) => {
    setPaying((prev) => ({ ...prev, [claimId]: true }));
    try {
      await apiPost<ClaimRecoveryResponse>(`/merchant/claims/${claimId}/recovery/pay`);
      toast.success("追偿已支付");
      await loadClaims();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "支付失败";
      toast.error(message);
    } finally {
      setPaying((prev) => ({ ...prev, [claimId]: false }));
    }
  };

  const isRecoverable = (claimId: number) => {
    const recovery = recoveryMap[claimId];
    return !!recovery && (recovery.status === "pending" || recovery.status === "overdue");
  };

  const visibleRecoverableIds = filteredClaims
    .filter((claim) => isRecoverable(claim.id))
    .map((claim) => claim.id);

  const selectedRecoverableIds = selectedIds.filter((id) =>
    visibleRecoverableIds.includes(id)
  );

  const allVisibleSelected =
    visibleRecoverableIds.length > 0 &&
    selectedRecoverableIds.length === visibleRecoverableIds.length;

  const handleToggleSelect = (claimId: number, checked: boolean) => {
    setSelectedIds((prev) => {
      if (checked) {
        if (prev.includes(claimId)) return prev;
        return [...prev, claimId];
      }
      return prev.filter((id) => id !== claimId);
    });
  };

  const handleToggleSelectAll = (checked: boolean) => {
    if (checked) {
      setSelectedIds((prev) => Array.from(new Set([...prev, ...visibleRecoverableIds])));
      return;
    }
    setSelectedIds((prev) => prev.filter((id) => !visibleRecoverableIds.includes(id)));
  };

  const handleBatchPay = async () => {
    if (selectedRecoverableIds.length === 0) {
      toast.error("请先选择待支付追偿单");
      return;
    }

    setBatchPaying(true);
    try {
      const results = await Promise.allSettled(
        selectedRecoverableIds.map((claimId) =>
          apiPost<ClaimRecoveryResponse>(`/merchant/claims/${claimId}/recovery/pay`)
        )
      );

      const successCount = results.filter((result) => result.status === "fulfilled").length;
      const failCount = results.length - successCount;

      if (successCount > 0) {
        toast.success(`批量支付成功 ${successCount} 笔`);
      }
      if (failCount > 0) {
        toast.error(`批量支付失败 ${failCount} 笔，请重试`);
      }

      setSelectedIds([]);
      await loadClaims();
    } finally {
      setBatchPaying(false);
    }
  };

  const formatAbnormalRate = (value: number) => `${(value * 100).toFixed(2)}%`;

  const handleOpenDecision = async (claim: MerchantClaimItem) => {
    setDecisionOpen(true);
    setDecisionLoading(true);
    setActiveClaim(claim);
    setActiveDecision(null);
    setActiveSummary(null);

    try {
      const [decisionRes, summaryRes] = await Promise.allSettled([
        apiGet<MerchantClaimDecisionResponse>(`/merchant/claims/${claim.id}/decision`),
        apiGet<MerchantClaimBehaviorSummaryResponse>(`/merchant/claims/behavior-summary`, {
          order_id: claim.order_id,
        }),
      ]);

      if (decisionRes.status === "fulfilled") {
        setActiveDecision(decisionRes.value.decision);
      }
      if (summaryRes.status === "fulfilled") {
        setActiveSummary(summaryRes.value);
      }

      if (decisionRes.status === "rejected" && summaryRes.status === "rejected") {
        toast.error("加载判定依据失败");
      }
    } finally {
      setDecisionLoading(false);
    }
  };

  return (
    <PageShell>
      <PageHeader
        title="索赔追偿"
        description="查看索赔追偿单状态，确认支付并恢复接单。"
        actions={
          <Button variant="outline" onClick={() => loadClaims()} disabled={loading}>
            刷新数据
          </Button>
        }
      >
        <div className="flex flex-wrap gap-3">
          <Select value={claimStatus} onValueChange={setClaimStatus}>
            <SelectTrigger className="w-40">
              <SelectValue placeholder="索赔状态" />
            </SelectTrigger>
            <SelectContent>
              {CLAIM_STATUS_OPTIONS.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={recoveryStatus} onValueChange={setRecoveryStatus}>
            <SelectTrigger className="w-40">
              <SelectValue placeholder="追偿状态" />
            </SelectTrigger>
            <SelectContent>
              {RECOVERY_STATUS_OPTIONS.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={appealStatus} onValueChange={setAppealStatus}>
            <SelectTrigger className="w-40">
              <SelectValue placeholder="申诉状态" />
            </SelectTrigger>
            <SelectContent>
              {APPEAL_STATUS_OPTIONS.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </PageHeader>
      <PageContent>
        <Card>
          <CardHeader>
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <CardTitle>追偿单列表</CardTitle>
                <CardDescription>只展示已批准索赔的追偿信息。</CardDescription>
              </div>
              <Button
                onClick={handleBatchPay}
                disabled={selectedRecoverableIds.length === 0 || batchPaying}
              >
                {batchPaying
                  ? "批量支付中"
                  : `批量支付（${selectedRecoverableIds.length}）`}
              </Button>
            </div>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-10">
                    <Checkbox
                      checked={allVisibleSelected}
                      onCheckedChange={(checked) => handleToggleSelectAll(checked === true)}
                      aria-label="全选待支付追偿"
                    />
                  </TableHead>
                  <TableHead>订单号</TableHead>
                  <TableHead>索赔类型</TableHead>
                  <TableHead>索赔原因</TableHead>
                  <TableHead>索赔金额</TableHead>
                  <TableHead>批准金额</TableHead>
                  <TableHead>索赔状态</TableHead>
                  <TableHead>申诉状态</TableHead>
                  <TableHead>追偿状态</TableHead>
                  <TableHead>追偿金额</TableHead>
                  <TableHead>到期时间</TableHead>
                  <TableHead>操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading ? (
                  <TableSkeleton rows={6} />
                ) : filteredClaims.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={12} className="text-center text-muted-foreground">
                      暂无符合条件的索赔追偿
                    </TableCell>
                  </TableRow>
                ) : (
                  filteredClaims.map((claim) => {
                    const recovery = recoveryMap[claim.id];
                    const canPay = recovery && (recovery.status === "pending" || recovery.status === "overdue");
                    return (
                      <TableRow key={claim.id}>
                        <TableCell>
                          <Checkbox
                            checked={selectedIds.includes(claim.id)}
                            onCheckedChange={(checked) =>
                              handleToggleSelect(claim.id, checked === true)
                            }
                            disabled={!canPay}
                            aria-label={`选择索赔 ${claim.id}`}
                          />
                        </TableCell>
                        <TableCell className="font-medium">
                          <Link
                            href={`/merchant/claims/${claim.id}`}
                            className="text-primary underline-offset-4 hover:underline"
                          >
                            {claim.order_no}
                          </Link>
                        </TableCell>
                        <TableCell>{formatClaimType(claim.claim_type)}</TableCell>
                        <TableCell className="max-w-52 truncate" title={claim.description || ""}>
                          {claim.description || "-"}
                        </TableCell>
                        <TableCell>¥{formatAmount(claim.claim_amount)}</TableCell>
                        <TableCell>
                          {typeof claim.approved_amount === "number"
                            ? `¥${formatAmount(claim.approved_amount)}`
                            : "-"}
                        </TableCell>
                        <TableCell>{formatClaimStatus(claim.status)}</TableCell>
                        <TableCell>
                          {claim.appeal_status ? formatAppealStatus(claim.appeal_status) : "-"}
                        </TableCell>
                        <TableCell>
                          {recovery ? (
                            <Badge
                              variant="outline"
                              className={cn("font-normal", RECOVERY_STATUS_STYLES[recovery.status])}
                            >
                              {formatRecoveryStatus(recovery.status)}
                            </Badge>
                          ) : (
                            "-"
                          )}
                        </TableCell>
                        <TableCell>
                          {recovery ? `¥${formatAmount(recovery.recovery_amount)}` : "-"}
                        </TableCell>
                        <TableCell>
                          {recovery ? formatDateTime(recovery.due_at) : "-"}
                        </TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-2">
                            <Button size="sm" variant="outline" asChild>
                              <Link href={`/merchant/claims/${claim.id}`}>
                                查看详情
                              </Link>
                            </Button>
                            <Button
                              size="sm"
                              variant="outline"
                              onClick={() => handleOpenDecision(claim)}
                            >
                              判定依据
                            </Button>
                            <Button
                              size="sm"
                              variant={canPay ? "default" : "outline"}
                              disabled={!canPay || paying[claim.id]}
                              onClick={() => handlePayRecovery(claim.id)}
                            >
                              {paying[claim.id] ? "处理中" : "支付追偿"}
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    );
                  })
                )}
              </TableBody>
            </Table>

            <div className="flex items-center justify-between pt-4 text-sm text-muted-foreground">
              <span>第 {page} / {totalPages} 页</span>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setPage((prev) => Math.max(1, prev - 1))}
                  disabled={page <= 1}
                >
                  上一页
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setPage((prev) => Math.min(totalPages, prev + 1))}
                  disabled={page >= totalPages}
                >
                  下一页
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>

        <Dialog open={decisionOpen} onOpenChange={setDecisionOpen}>
          <DialogContent className="sm:max-w-2xl">
            <DialogHeader>
              <DialogTitle>判定依据与回溯摘要</DialogTitle>
              <DialogDescription>
                {activeClaim
                  ? `订单 ${activeClaim.order_no} · 索赔类型 ${formatClaimType(activeClaim.claim_type)}`
                  : "查看平台判责结果与索赔回溯信息"}
              </DialogDescription>
            </DialogHeader>

            {decisionLoading ? (
              <div className="py-6 text-sm text-muted-foreground">加载中...</div>
            ) : (
              <div className="space-y-4 text-sm">
                <Card>
                  <CardHeader>
                    <CardTitle className="text-base">平台判定</CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-2">
                    {activeDecision ? (
                      <>
                        <div className="flex justify-between">
                          <span className="text-muted-foreground">责任方</span>
                          <span>{formatDecisionResponsibleParty(activeDecision.responsible_party)}</span>
                        </div>
                        <div className="flex justify-between">
                          <span className="text-muted-foreground">赔付来源</span>
                          <span>{formatDecisionCompensationSource(activeDecision.compensation_source)}</span>
                        </div>
                        <div className="flex justify-between">
                          <span className="text-muted-foreground">判定状态</span>
                          <span>{formatDecisionStatus(activeDecision.decision_status)}</span>
                        </div>
                        <div>
                          <div className="text-muted-foreground">原因码</div>
                          <div className="mt-1 flex flex-wrap gap-2">
                            {activeDecision.reason_codes?.length ? (
                              activeDecision.reason_codes.map((code) => (
                                <Badge key={code} variant="outline" className="font-normal">
                                  {formatDecisionReasonCode(code)}
                                </Badge>
                              ))
                            ) : (
                              <span>-</span>
                            )}
                          </div>
                        </div>
                        <div>
                          <div className="text-muted-foreground">判定摘要</div>
                          <div className="mt-1">{activeDecision.trace_summary || "-"}</div>
                        </div>
                      </>
                    ) : (
                      <div className="text-muted-foreground">暂无判定信息</div>
                    )}
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader>
                    <CardTitle className="text-base">行为回溯摘要</CardTitle>
                    {activeSummary && (
                      <CardDescription>
                        统计窗口：{activeSummary.window.start_date} ~ {activeSummary.window.end_date}
                      </CardDescription>
                    )}
                  </CardHeader>
                  <CardContent>
                    {activeSummary ? (
                      <div className="grid gap-3 md:grid-cols-3">
                        <div className="rounded-lg border p-3">
                          <div className="mb-1 font-medium">用户</div>
                          <div className="text-muted-foreground">订单数：{activeSummary.user.total_orders}</div>
                          <div className="text-muted-foreground">异常索赔：{activeSummary.user.abnormal_claims}</div>
                          <div className="text-muted-foreground">
                            异常率：{formatAbnormalRate(activeSummary.user.abnormal_rate)}
                          </div>
                        </div>
                        <div className="rounded-lg border p-3">
                          <div className="mb-1 font-medium">商户</div>
                          <div className="text-muted-foreground">订单数：{activeSummary.merchant.total_orders}</div>
                          <div className="text-muted-foreground">异常索赔：{activeSummary.merchant.abnormal_claims}</div>
                          <div className="text-muted-foreground">
                            异常率：{formatAbnormalRate(activeSummary.merchant.abnormal_rate)}
                          </div>
                        </div>
                        <div className="rounded-lg border p-3">
                          <div className="mb-1 font-medium">骑手</div>
                          {activeSummary.rider ? (
                            <>
                              <div className="text-muted-foreground">订单数：{activeSummary.rider.total_orders}</div>
                              <div className="text-muted-foreground">异常索赔：{activeSummary.rider.abnormal_claims}</div>
                              <div className="text-muted-foreground">
                                异常率：{formatAbnormalRate(activeSummary.rider.abnormal_rate)}
                              </div>
                            </>
                          ) : (
                            <div className="text-muted-foreground">无骑手数据</div>
                          )}
                        </div>
                      </div>
                    ) : (
                      <div className="text-muted-foreground">暂无回溯摘要</div>
                    )}
                  </CardContent>
                </Card>
              </div>
            )}
          </DialogContent>
        </Dialog>
      </PageContent>
    </PageShell>
  );
}