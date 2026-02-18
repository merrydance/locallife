"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
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
import { PageContent, PageHeader, PageShell } from "@/components/merchant/layout/page-shell";
import { apiGet, apiPost, formatAmount } from "@/lib/api";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import type {
  ClaimRecoveryResponse,
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
            <CardTitle>追偿单列表</CardTitle>
            <CardDescription>只展示已批准索赔的追偿信息。</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>订单号</TableHead>
                  <TableHead>索赔类型</TableHead>
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
                    <TableCell colSpan={10} className="text-center text-muted-foreground">
                      暂无符合条件的索赔追偿
                    </TableCell>
                  </TableRow>
                ) : (
                  filteredClaims.map((claim) => {
                    const recovery = recoveryMap[claim.id];
                    const canPay = recovery && (recovery.status === "pending" || recovery.status === "overdue");
                    return (
                      <TableRow key={claim.id}>
                        <TableCell className="font-medium">
                          <Link
                            href={`/merchant/claims/${claim.id}`}
                            className="text-primary underline-offset-4 hover:underline"
                          >
                            {claim.order_no}
                          </Link>
                        </TableCell>
                        <TableCell>{claim.claim_type}</TableCell>
                        <TableCell>¥{formatAmount(claim.claim_amount)}</TableCell>
                        <TableCell>
                          {typeof claim.approved_amount === "number"
                            ? `¥${formatAmount(claim.approved_amount)}`
                            : "-"}
                        </TableCell>
                        <TableCell>{claim.status}</TableCell>
                        <TableCell>{claim.appeal_status || "-"}</TableCell>
                        <TableCell>
                          {recovery ? (
                            <Badge
                              variant="outline"
                              className={cn("font-normal", RECOVERY_STATUS_STYLES[recovery.status])}
                            >
                              {recovery.status}
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
      </PageContent>
    </PageShell>
  );
}