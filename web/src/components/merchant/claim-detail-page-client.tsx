"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { ArrowLeft, ClipboardList } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import {
  PageContent,
  PageHeader,
  PageShell,
} from "@/components/merchant/layout/page-shell";
import { apiGet, apiPost, formatAmount } from "@/lib/api";
import { cn } from "@/lib/utils";
import type {
  ClaimRecoveryResponse,
  MerchantClaimDetailResponse,
} from "@/types/claim-recovery";

const RECOVERY_STATUS_STYLES: Record<string, string> = {
  pending: "bg-amber-100 text-amber-700 border-amber-200",
  overdue: "bg-rose-100 text-rose-700 border-rose-200",
  paid: "bg-emerald-100 text-emerald-700 border-emerald-200",
  waived: "bg-slate-100 text-slate-700 border-slate-200",
  appealed: "bg-blue-100 text-blue-700 border-blue-200",
};

const APPEAL_STATUS_LABELS: Record<string, string> = {
  pending: "待处理",
  approved: "已通过",
  rejected: "已驳回",
};

const APPEAL_STATUS_STYLES: Record<string, string> = {
  pending: "bg-amber-100 text-amber-700 border-amber-200",
  approved: "bg-emerald-100 text-emerald-700 border-emerald-200",
  rejected: "bg-rose-100 text-rose-700 border-rose-200",
};

function DetailSkeleton() {
  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <Skeleton className="h-5 w-32" />
          <Skeleton className="h-4 w-48" />
        </CardHeader>
        <CardContent className="grid gap-3">
          {Array.from({ length: 6 }).map((_, idx) => (
            <Skeleton key={idx} className="h-4 w-60" />
          ))}
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <Skeleton className="h-5 w-32" />
          <Skeleton className="h-4 w-40" />
        </CardHeader>
        <CardContent className="grid gap-3">
          {Array.from({ length: 4 }).map((_, idx) => (
            <Skeleton key={idx} className="h-4 w-48" />
          ))}
        </CardContent>
      </Card>
    </div>
  );
}

export function ClaimDetailPageClient({ claimId }: { claimId: string }) {
  const [loading, setLoading] = useState(true);
  const [claim, setClaim] = useState<MerchantClaimDetailResponse | null>(null);
  const [recovery, setRecovery] = useState<ClaimRecoveryResponse | null>(null);
  const [paying, setPaying] = useState(false);
  const [appealReason, setAppealReason] = useState("");
  const [appealSubmitting, setAppealSubmitting] = useState(false);

  const loadDetail = useCallback(async () => {
    setLoading(true);
    try {
      const detail = await apiGet<MerchantClaimDetailResponse>(
        `/merchant/claims/${claimId}`
      );
      setClaim(detail);
      try {
        const recoveryDetail = await apiGet<ClaimRecoveryResponse>(
          `/merchant/claims/${claimId}/recovery`
        );
        setRecovery(recoveryDetail);
      } catch {
        setRecovery(null);
      }
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "加载索赔详情失败";
      toast.error(message);
    } finally {
      setLoading(false);
    }
  }, [claimId]);

  useEffect(() => {
    loadDetail();
  }, [loadDetail]);

  const handlePayRecovery = async () => {
    if (!claim) return;
    setPaying(true);
    try {
      await apiPost(`/merchant/claims/${claim.id}/recovery/pay`);
      toast.success("追偿已支付");
      await loadDetail();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "支付失败";
      toast.error(message);
    } finally {
      setPaying(false);
    }
  };

  const canCreateAppeal =
    !!claim && !claim.appeal_id && (claim.status === "approved" || claim.status === "auto-approved");

  const handleCreateAppeal = async () => {
    if (!claim) return;
    if (appealReason.trim().length < 10) {
      toast.error("申诉原因至少10个字符");
      return;
    }
    setAppealSubmitting(true);
    try {
      await apiPost("/merchant/appeals", {
        claim_id: claim.id,
        reason: appealReason.trim(),
      });
      toast.success("申诉已提交");
      setAppealReason("");
      await loadDetail();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "提交申诉失败";
      toast.error(message);
    } finally {
      setAppealSubmitting(false);
    }
  };

  const canPayRecovery = recovery && (recovery.status === "pending" || recovery.status === "overdue");

  return (
    <PageShell>
      <PageHeader
        title="索赔详情"
        description="查看索赔与追偿处理情况。"
        actions={
          <Button variant="outline" asChild>
            <Link href="/merchant/claims">
              <ArrowLeft className="mr-2 h-4 w-4" />
              返回列表
            </Link>
          </Button>
        }
      />
      <PageContent>
        {loading ? (
          <DetailSkeleton />
        ) : !claim ? (
          <Card>
            <CardContent className="py-10 text-center text-muted-foreground">
              未找到索赔信息
            </CardContent>
          </Card>
        ) : (
          <div className="grid gap-6 xl:grid-cols-[2fr_1fr]">
            <div className="space-y-6">
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <ClipboardList className="h-5 w-5" />
                    索赔信息
                  </CardTitle>
                  <CardDescription>索赔基础信息与申诉状态</CardDescription>
                </CardHeader>
                <CardContent className="grid gap-3 text-sm">
                  <div className="flex flex-wrap items-center gap-2">
                    <Badge variant="outline">索赔状态：{claim.status}</Badge>
                    {claim.appeal_status && (
                      <Badge
                        variant="outline"
                        className={cn(
                          "font-normal",
                          APPEAL_STATUS_STYLES[claim.appeal_status] || ""
                        )}
                      >
                        申诉状态：{APPEAL_STATUS_LABELS[claim.appeal_status] || claim.appeal_status}
                      </Badge>
                    )}
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">订单号</span>
                    <span className="font-medium">{claim.order_no}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">索赔类型</span>
                    <span>{claim.claim_type}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">索赔金额</span>
                    <span>¥{formatAmount(claim.claim_amount)}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">批准金额</span>
                    <span>
                      {typeof claim.approved_amount === "number"
                        ? `¥${formatAmount(claim.approved_amount)}`
                        : "-"}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">索赔状态</span>
                    <span>{claim.status}</span>
                  </div>
                  <div className="flex flex-col gap-1">
                    <span className="text-muted-foreground">索赔说明</span>
                    <p className="text-sm text-foreground">{claim.description}</p>
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>处理时间线</CardTitle>
                  <CardDescription>关键处理节点与时间</CardDescription>
                </CardHeader>
                <CardContent className="space-y-4 text-sm">
                  <div className="flex items-start justify-between">
                    <div>
                      <div className="font-medium">索赔提交</div>
                      <div className="text-muted-foreground">系统收到索赔</div>
                    </div>
                    <span className="text-muted-foreground">{claim.created_at}</span>
                  </div>
                  <Separator />
                  <div className="flex items-start justify-between">
                    <div>
                      <div className="font-medium">索赔审核</div>
                      <div className="text-muted-foreground">自动裁决或复核完成</div>
                    </div>
                    <span className="text-muted-foreground">{claim.reviewed_at || "-"}</span>
                  </div>
                  {claim.appeal_id && (
                    <>
                      <Separator />
                      <div className="flex items-start justify-between">
                        <div>
                          <div className="font-medium">申诉处理</div>
                          <div className="text-muted-foreground">申诉已提交并进入审核</div>
                        </div>
                        <span className="text-muted-foreground">{claim.appeal_status || "-"}</span>
                      </div>
                    </>
                  )}
                </CardContent>
              </Card>

              {claim.appeal_id && (
                <Card>
                  <CardHeader>
                    <CardTitle>申诉信息</CardTitle>
                    <CardDescription>商户申诉处理结果</CardDescription>
                  </CardHeader>
                  <CardContent className="grid gap-3 text-sm">
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">申诉状态</span>
                      <span>{claim.appeal_status || "-"}</span>
                    </div>
                    <div className="flex flex-col gap-1">
                      <span className="text-muted-foreground">申诉原因</span>
                      <p className="text-sm text-foreground">{claim.appeal_reason || "-"}</p>
                    </div>
                    <div className="flex flex-col gap-1">
                      <span className="text-muted-foreground">审核备注</span>
                      <p className="text-sm text-foreground">{claim.appeal_review_notes || "-"}</p>
                    </div>
                  </CardContent>
                </Card>
              )}

              {canCreateAppeal && (
                <Card>
                  <CardHeader>
                    <CardTitle>提交申诉</CardTitle>
                    <CardDescription>仅可申诉一次，请说明原因</CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-3">
                    <div className="rounded-lg border border-dashed border-muted-foreground/40 bg-muted/30 p-3 text-xs text-muted-foreground">
                      <p className="font-medium text-foreground">申诉提示</p>
                      <ul className="mt-2 list-disc space-y-1 pl-4">
                        <li>申诉仅用于纠错复核，不影响已完成的用户赔付。</li>
                        <li>申诉通过将回滚追偿并恢复接单限制。</li>
                        <li>请提供清晰的事实描述，避免无效申诉。</li>
                      </ul>
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="appeal-reason">申诉原因</Label>
                      <Textarea
                        id="appeal-reason"
                        className="min-h-32"
                        placeholder="请输入申诉原因（至少10个字符）"
                        value={appealReason}
                        onChange={(event) => setAppealReason(event.target.value)}
                      />
                    </div>
                    <Button
                      className="w-full"
                      disabled={appealSubmitting}
                      onClick={handleCreateAppeal}
                    >
                      {appealSubmitting ? "提交中" : "提交申诉"}
                    </Button>
                  </CardContent>
                </Card>
              )}
            </div>

            <Card className="h-fit">
              <CardHeader>
                <CardTitle>追偿单</CardTitle>
                <CardDescription>追偿状态与支付操作</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4 text-sm">
                {recovery ? (
                  <>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">追偿状态</span>
                      <Badge
                        variant="outline"
                        className={cn("font-normal", RECOVERY_STATUS_STYLES[recovery.status])}
                      >
                        {recovery.status}
                      </Badge>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">追偿金额</span>
                      <span>¥{formatAmount(recovery.recovery_amount)}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">到期时间</span>
                      <span>{recovery.due_at}</span>
                    </div>
                    <Button
                      className="w-full"
                      disabled={!canPayRecovery || paying}
                      onClick={handlePayRecovery}
                    >
                      {paying ? "处理中" : "支付追偿"}
                    </Button>
                  </>
                ) : (
                  <div className="text-muted-foreground">暂无追偿单信息</div>
                )}
              </CardContent>
            </Card>
          </div>
        )}
      </PageContent>
    </PageShell>
  );
}