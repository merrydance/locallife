"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { RefreshCw, Wallet, ArrowUpRight } from "lucide-react";
import { toast } from "sonner";
import { apiGet, apiPost, formatAmount } from "@/lib/api";
import {
  PageShell,
  PageHeader,
  PageContent,
} from "@/components/merchant/layout/page-shell";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Skeleton } from "@/components/ui/skeleton";

interface MerchantAccountBalance {
  sub_mch_id: string;
  available_amount: number;
  pending_amount: number;
  withdrawable_amount: number;
}

interface MerchantWithdrawalItem {
  id: number;
  amount: number;
  status: "pending" | "success" | "failed" | string;
  channel: string;
  out_request_no?: string;
  withdraw_id?: string;
  sub_mch_id?: string;
  reason?: string;
  created_at: string;
  updated_at: string;
}

interface MerchantWithdrawalListResponse {
  withdrawals: MerchantWithdrawalItem[];
  total_count: number;
}

const statusMeta: Record<string, { label: string; className: string }> = {
  pending: {
    label: "处理中",
    className: "bg-amber-100 text-amber-700 border-amber-200",
  },
  success: {
    label: "成功",
    className: "bg-emerald-100 text-emerald-700 border-emerald-200",
  },
  failed: {
    label: "失败",
    className: "bg-rose-100 text-rose-700 border-rose-200",
  },
};

export function FinanceAccountPageClient() {
  const [loadingBalance, setLoadingBalance] = useState(true);
  const [loadingList, setLoadingList] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [balance, setBalance] = useState<MerchantAccountBalance | null>(null);
  const [withdrawals, setWithdrawals] = useState<MerchantWithdrawalItem[]>([]);
  const [amountYuan, setAmountYuan] = useState("");
  const [remark, setRemark] = useState("");

  const pageSize = 20;

  const loadBalance = useCallback(async () => {
    setLoadingBalance(true);
    try {
      const data = await apiGet<MerchantAccountBalance>("/merchant/finance/account/balance");
      setBalance(data);
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "加载账户余额失败";
      toast.error(message);
    } finally {
      setLoadingBalance(false);
    }
  }, []);

  const loadWithdrawals = useCallback(async () => {
    setLoadingList(true);
    try {
      const data = await apiGet<MerchantWithdrawalListResponse>(
        "/merchant/finance/account/withdrawals",
        {
          page: 1,
          limit: pageSize,
        }
      );
      setWithdrawals(data.withdrawals || []);
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "加载提现记录失败";
      toast.error(message);
    } finally {
      setLoadingList(false);
    }
  }, []);

  const refreshAll = useCallback(async () => {
    await Promise.all([loadBalance(), loadWithdrawals()]);
  }, [loadBalance, loadWithdrawals]);

  useEffect(() => {
    refreshAll();
  }, [refreshAll]);

  const amountFen = useMemo(() => {
    const normalized = amountYuan.trim();
    if (!normalized) return 0;
    const parsed = Number(normalized);
    if (!Number.isFinite(parsed) || parsed <= 0) return 0;
    return Math.round(parsed * 100);
  }, [amountYuan]);

  const canSubmit = amountFen >= 100 && remark.trim().length > 0 && !submitting;

  const handleSubmitWithdraw = async () => {
    if (!canSubmit) return;

    setSubmitting(true);
    try {
      await apiPost<{ withdrawal: MerchantWithdrawalItem }>(
        "/merchant/finance/account/withdraw",
        {
          amount: amountFen,
          remark: remark.trim(),
        }
      );

      toast.success("提现申请已提交");
      setAmountYuan("");
      setRemark("");
      await refreshAll();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "提现申请失败";
      toast.error(message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <PageShell>
      <PageHeader
        title="资金账户"
        description="查看收付通账户余额并发起提现"
        actions={
          <Button variant="outline" onClick={refreshAll}>
            <RefreshCw className="mr-2 h-4 w-4" /> 刷新
          </Button>
        }
      />

      <PageContent>
        <div className="grid gap-4 md:grid-cols-3">
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">可用余额</CardTitle>
            </CardHeader>
            <CardContent>
              {loadingBalance ? (
                <Skeleton className="h-8 w-28" />
              ) : (
                <div className="text-2xl font-bold">¥{formatAmount(balance?.available_amount)}</div>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">可提现余额</CardTitle>
            </CardHeader>
            <CardContent>
              {loadingBalance ? (
                <Skeleton className="h-8 w-28" />
              ) : (
                <div className="text-2xl font-bold">¥{formatAmount(balance?.withdrawable_amount)}</div>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">处理中金额</CardTitle>
            </CardHeader>
            <CardContent>
              {loadingBalance ? (
                <Skeleton className="h-8 w-28" />
              ) : (
                <div className="text-2xl font-bold">¥{formatAmount(balance?.pending_amount)}</div>
              )}
            </CardContent>
          </Card>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <ArrowUpRight className="h-4 w-4" /> 发起提现
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="withdraw-amount">提现金额（元）</Label>
                <Input
                  id="withdraw-amount"
                  type="number"
                  min="1"
                  step="0.01"
                  placeholder="例如 100.00"
                  value={amountYuan}
                  onChange={(event) => setAmountYuan(event.target.value)}
                />
                <p className="text-xs text-muted-foreground">最小提现金额 ¥1.00</p>
              </div>

              <div className="space-y-2">
                <Label htmlFor="withdraw-remark">提现备注</Label>
                <Input
                  id="withdraw-remark"
                  maxLength={128}
                  placeholder="例如：本周结算"
                  value={remark}
                  onChange={(event) => setRemark(event.target.value)}
                />
              </div>
            </div>

            <div className="flex items-center justify-between">
              <div className="text-xs text-muted-foreground">
                当前子商户号：{balance?.sub_mch_id || "-"}
              </div>
              <Button onClick={handleSubmitWithdraw} disabled={!canSubmit}>
                {submitting ? "提交中..." : "提交提现申请"}
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <Wallet className="h-4 w-4" /> 提现记录
            </CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>申请单号</TableHead>
                  <TableHead>金额</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>失败原因</TableHead>
                  <TableHead>申请时间</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loadingList ? (
                  Array.from({ length: 5 }).map((_, index) => (
                    <TableRow key={index}>
                      <TableCell><Skeleton className="h-4 w-32" /></TableCell>
                      <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                      <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                      <TableCell><Skeleton className="h-4 w-36" /></TableCell>
                      <TableCell><Skeleton className="h-4 w-28" /></TableCell>
                    </TableRow>
                  ))
                ) : withdrawals.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={5} className="text-center text-sm text-muted-foreground">
                      暂无提现记录
                    </TableCell>
                  </TableRow>
                ) : (
                  withdrawals.map((item) => {
                    const meta = statusMeta[item.status] || {
                      label: item.status,
                      className: "bg-slate-100 text-slate-700 border-slate-200",
                    };

                    return (
                      <TableRow key={item.id}>
                        <TableCell className="font-mono text-xs">{item.out_request_no || "-"}</TableCell>
                        <TableCell>¥{formatAmount(item.amount)}</TableCell>
                        <TableCell>
                          <Badge className={meta.className}>{meta.label}</Badge>
                        </TableCell>
                        <TableCell className="max-w-75 truncate text-muted-foreground">
                          {item.reason || "-"}
                        </TableCell>
                        <TableCell className="text-xs text-muted-foreground">
                          {item.created_at ? new Date(item.created_at).toLocaleString("zh-CN") : "-"}
                        </TableCell>
                      </TableRow>
                    );
                  })
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </PageContent>
    </PageShell>
  );
}
