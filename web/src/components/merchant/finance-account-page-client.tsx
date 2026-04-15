"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  RefreshCw,
  Wallet,
  ArrowUpRight,
  Building2,
  CreditCard,
  ExternalLink,
  CheckCircle2,
  Clock,
  XCircle,
  FileSignature,
} from "lucide-react";
import { toast } from "sonner";
import { apiGet, apiPost, formatAmount } from "@/lib/api";
import {
  PageShell,
  PageHeader,
  PageContent,
} from "@/components/merchant/layout/page-shell";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
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
import { ApplymentBankForm } from "@/components/applyment/applyment-bank-form";
import type { ApplymentBindBankPayload } from "@/types/applyment-bank";

/* ─────────────────────────── Interfaces ─────────────────────────── */

interface MerchantAccountBalance {
  sub_mch_id: string;
  available_amount: number;
  pending_amount: number;
  withdrawable_amount: number;
  account_status?: string;
  status_desc?: string;
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
  total: number;
  account_status?: string;
  status_desc?: string;
}

interface ApplymentStatusResponse {
  status: string;
  status_desc: string;
  can_submit?: boolean;
  block_reason?: string;
  sign_url?: string;
  sign_state?: string;
  legal_validation_url?: string;
  account_validation?: ApplymentAccountValidationResponse;
  sub_mch_id?: string;
  reject_reason?: string;
}

interface ApplymentAccountValidationResponse {
  account_name?: string;
  account_no?: string;
  pay_amount?: number;
  destination_account_number?: string;
  destination_account_name?: string;
  destination_account_bank?: string;
  city?: string;
  remark?: string;
  deadline?: string;
}

/* ─────────────────────────── Helpers ─────────────────────────── */

const withdrawStatusMeta: Record<string, { label: string; className: string }> =
  {
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

const applymentStatusMeta: Record<
  string,
  { label: string; className: string }
> = {
  submitted: {
    label: "已提交",
    className: "bg-blue-100 text-blue-700 border-blue-200",
  },
  bindbank_submitted: {
    label: "已提交",
    className: "bg-blue-100 text-blue-700 border-blue-200",
  },
  checking: {
    label: "校验中",
    className: "bg-blue-100 text-blue-700 border-blue-200",
  },
  auditing: {
    label: "审核中",
    className: "bg-amber-100 text-amber-700 border-amber-200",
  },
  account_need_verify: {
    label: "待验证",
    className: "bg-amber-100 text-amber-700 border-amber-200",
  },
  to_be_confirmed: {
    label: "待确认",
    className: "bg-amber-100 text-amber-700 border-amber-200",
  },
  to_be_signed: {
    label: "待签约",
    className: "bg-indigo-100 text-indigo-700 border-indigo-200",
  },
  signing: {
    label: "签约中",
    className: "bg-indigo-100 text-indigo-700 border-indigo-200",
  },
  finish: {
    label: "已开通",
    className: "bg-emerald-100 text-emerald-700 border-emerald-200",
  },
  active: {
    label: "已开通",
    className: "bg-emerald-100 text-emerald-700 border-emerald-200",
  },
  rejected: {
    label: "已拒绝",
    className: "bg-rose-100 text-rose-700 border-rose-200",
  },
  frozen: {
    label: "已冻结",
    className: "bg-rose-100 text-rose-700 border-rose-200",
  },
  canceled: {
    label: "已作废",
    className: "bg-slate-100 text-slate-700 border-slate-200",
  },
};

function normalizeApplymentSignState(signState?: string): string {
  return String(signState ?? "").trim().toUpperCase();
}

function shouldApplymentNeedSign(status: string, signState?: string): boolean {
  const normalizedSignState = normalizeApplymentSignState(signState);
  if (normalizedSignState === "UNSIGNED") {
    return true;
  }
  if (normalizedSignState === "SIGNED" || normalizedSignState === "NOT_SIGNABLE") {
    return false;
  }
  return status === "to_be_signed" || status === "signing";
}

function getApplymentSignStateText(signState?: string): string {
  switch (normalizeApplymentSignState(signState)) {
    case "UNSIGNED":
      return "未签约";
    case "SIGNED":
      return "已签约";
    case "NOT_SIGNABLE":
      return "当前不可签约";
    default:
      return "";
  }
}

/* ─────────────────────────── Main component ─────────────────────────── */

export function FinanceAccountPageClient() {
  /* Balance / withdraw */
  const [loadingBalance, setLoadingBalance] = useState(true);
  const [loadingList, setLoadingList] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [balance, setBalance] = useState<MerchantAccountBalance | null>(null);
  const [notConfigured, setNotConfigured] = useState(false);
  const [withdrawals, setWithdrawals] = useState<MerchantWithdrawalItem[]>([]);
  const [amountYuan, setAmountYuan] = useState("");
  const [remark, setRemark] = useState("");

  /* Applyment: undefined=loading, null=404 (not applied), object=has record */
  const [loadingApplyment, setLoadingApplyment] = useState(true);
  const [applymentStatus, setApplymentStatus] = useState<
    ApplymentStatusResponse | null | undefined
  >(undefined);

  /* Bind bank form */
  const [showBindForm, setShowBindForm] = useState(false);
  const [submittingBind, setSubmittingBind] = useState(false);

  const pageSize = 20;

  /* ── Loaders ── */

  const loadBalance = useCallback(async () => {
    setLoadingBalance(true);
    try {
      const data = await apiGet<MerchantAccountBalance>(
        "/merchant/finance/account/balance"
      );
      setBalance(data);
      setNotConfigured((data.account_status ?? "active") !== "active");
    } catch (error: unknown) {
      toast.error(
        error instanceof Error ? error.message : "加载账户余额失败"
      );
    } finally {
      setLoadingBalance(false);
    }
  }, []);

  const loadWithdrawals = useCallback(async () => {
    setLoadingList(true);
    try {
      const data = await apiGet<MerchantWithdrawalListResponse>(
        "/merchant/finance/account/withdrawals",
        { page: 1, limit: pageSize }
      );
      setWithdrawals(data.withdrawals || []);
    } catch (error: unknown) {
      toast.error(
        error instanceof Error ? error.message : "加载提现记录失败"
      );
    } finally {
      setLoadingList(false);
    }
  }, []);

  const loadApplymentStatus = useCallback(async () => {
    setLoadingApplyment(true);
    try {
      const data = await apiGet<ApplymentStatusResponse>(
        "/merchant/applyment/status"
      );
      if (data.status === "not_applied") {
        setApplymentStatus(null);
      } else {
        setApplymentStatus(data);
        setShowBindForm(false);
      }
    } catch (error: unknown) {
      toast.error(
        error instanceof Error ? error.message : "查询进件状态失败"
      );
      setApplymentStatus(null);
    } finally {
      setLoadingApplyment(false);
    }
  }, []);

  const refreshAll = useCallback(async () => {
    await Promise.all([
      loadBalance(),
      loadWithdrawals(),
      loadApplymentStatus(),
    ]);
  }, [loadBalance, loadWithdrawals, loadApplymentStatus]);

  useEffect(() => {
    refreshAll();
  }, [refreshAll]);

  /* ── Withdraw ── */

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
        { amount: amountFen, remark: remark.trim() }
      );
      toast.success("提现申请已提交");
      setAmountYuan("");
      setRemark("");
      await refreshAll();
    } catch (error: unknown) {
      toast.error(error instanceof Error ? error.message : "提现申请失败");
    } finally {
      setSubmitting(false);
    }
  };

  /* ── Bind bank ── */

  const handleSubmitBindBank = async (payload: ApplymentBindBankPayload) => {
    if (submittingBind) return;
    setSubmittingBind(true);
    try {
      await apiPost("/merchant/applyment/bindbank", payload);
      toast.success("银行账户信息已提交，请等待微信审核");
      setShowBindForm(false);
      await loadApplymentStatus();
    } catch (error: unknown) {
      toast.error(
        error instanceof Error ? error.message : "提交银行账户信息失败"
      );
    } finally {
      setSubmittingBind(false);
    }
  };

  /* ─────────────────────────── Render ─────────────────────────── */

  return (
    <PageShell>
      <PageHeader
        title="资金账户"
        description="收付通账户余额、提现及开户管理"
        actions={
          <Button variant="outline" onClick={refreshAll}>
            <RefreshCw className="mr-2 h-4 w-4" /> 刷新
          </Button>
        }
      />

      <PageContent>
        {/* ── 收付通进件状态 ── */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <Building2 className="h-4 w-4" /> 收付通进件状态
            </CardTitle>
            <CardDescription>
              微信支付二级商户进件，开通后方可收款和提现
            </CardDescription>
          </CardHeader>
          <CardContent>
            {loadingApplyment ? (
              <div className="space-y-2">
                <Skeleton className="h-5 w-40" />
                <Skeleton className="h-4 w-64" />
              </div>
            ) : applymentStatus == null ? (
              <div className="space-y-4">
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Clock className="h-4 w-4 shrink-0" />
                  <span>尚未提交进件申请，请填写银行结算账户信息。</span>
                </div>
                {!showBindForm ? (
                  <Button size="sm" onClick={() => setShowBindForm(true)}>
                    <CreditCard className="mr-2 h-4 w-4" />
                    填写银行账户信息
                  </Button>
                ) : (
                  <ApplymentBankForm
                    apiBasePath="/merchant/applyment"
                    defaultAccountType="ACCOUNT_TYPE_BUSINESS"
                    submitting={submittingBind}
                    submitLabel="提交银行账户信息"
                    onSubmit={handleSubmitBindBank}
                    onCancel={() => setShowBindForm(false)}
                  />
                )}
              </div>
            ) : (
              <div className="space-y-3">
                {(() => {
                  const needsSign = shouldApplymentNeedSign(
                    applymentStatus.status,
                    applymentStatus.sign_state
                  );
                  const signStateText = getApplymentSignStateText(
                    applymentStatus.sign_state
                  );
                  const accountValidation = applymentStatus.account_validation;
                  const payAmount = accountValidation?.pay_amount ?? 0;

                  return (
                    <>
                <div className="flex flex-wrap items-center gap-3">
                  {(() => {
                    const meta = applymentStatusMeta[applymentStatus.status] ?? {
                      label: applymentStatus.status,
                      className: "bg-slate-100 text-slate-700 border-slate-200",
                    };
                    return <Badge className={meta.className}>{meta.label}</Badge>;
                  })()}
                  <span className="text-sm text-muted-foreground">
                    {applymentStatus.status_desc}
                  </span>
                  {applymentStatus.sub_mch_id && (
                    <span className="font-mono text-xs text-muted-foreground">
                      子商户号：{applymentStatus.sub_mch_id}
                    </span>
                  )}
                  {signStateText && (
                    <span className="text-xs text-muted-foreground">
                      签约状态：{signStateText}
                    </span>
                  )}
                </div>

                {needsSign && applymentStatus.sign_url && (
                  <div className="flex items-center gap-2">
                    <FileSignature className="h-4 w-4 shrink-0 text-indigo-500" />
                    <span className="text-sm">需要完成签约：</span>
                    <a
                      href={applymentStatus.sign_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1 text-sm text-primary underline-offset-4 hover:underline"
                    >
                      前往签约 <ExternalLink className="h-3 w-3" />
                    </a>
                  </div>
                )}

                {needsSign && !applymentStatus.sign_url && (
                  <div className="flex items-center gap-2 rounded-md border border-indigo-200 bg-indigo-50 p-3 text-sm text-indigo-700">
                    <FileSignature className="h-4 w-4 shrink-0" />
                    当前存在待签约事项，请完成签约后再刷新状态。
                  </div>
                )}

                {applymentStatus.legal_validation_url && (
                  <div className="flex items-center gap-2 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-700">
                    <ExternalLink className="h-4 w-4 shrink-0" />
                    <span>可优先通过法人扫码完成账户验证：</span>
                    <a
                      href={applymentStatus.legal_validation_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1 text-sm font-medium underline-offset-4 hover:underline"
                    >
                      前往扫码验证 <ExternalLink className="h-3 w-3" />
                    </a>
                  </div>
                )}

                {accountValidation && (
                  <div className="rounded-lg border border-amber-200 bg-amber-50/60 p-4">
                    <div className="mb-3 flex items-center gap-2 text-sm font-medium text-amber-800">
                      <CreditCard className="h-4 w-4" />
                      账户验证汇款指引
                    </div>
                    <div className="grid gap-3 text-sm text-amber-900 md:grid-cols-2">
                      <div>
                        <div className="text-xs text-amber-700">收款户名</div>
                        <div>{accountValidation.destination_account_name || "-"}</div>
                      </div>
                      <div>
                        <div className="text-xs text-amber-700">收款账号</div>
                        <div className="font-mono text-xs">
                          {accountValidation.destination_account_number || "-"}
                        </div>
                      </div>
                      <div>
                        <div className="text-xs text-amber-700">收款银行</div>
                        <div>{accountValidation.destination_account_bank || "-"}</div>
                      </div>
                      <div>
                        <div className="text-xs text-amber-700">汇款金额</div>
                        <div>{payAmount > 0 ? `¥${formatAmount(payAmount)}` : "-"}</div>
                      </div>
                      <div>
                        <div className="text-xs text-amber-700">验证账户名</div>
                        <div>{accountValidation.account_name || "-"}</div>
                      </div>
                      <div>
                        <div className="text-xs text-amber-700">验证账号</div>
                        <div className="font-mono text-xs">{accountValidation.account_no || "-"}</div>
                      </div>
                      <div>
                        <div className="text-xs text-amber-700">汇款城市</div>
                        <div>{accountValidation.city || "-"}</div>
                      </div>
                      <div>
                        <div className="text-xs text-amber-700">截止时间</div>
                        <div>{accountValidation.deadline || "-"}</div>
                      </div>
                    </div>
                    <div className="mt-3 text-xs text-amber-700">
                      备注：{accountValidation.remark || "请按微信返回指引完成汇款验证"}
                    </div>
                  </div>
                )}

                {applymentStatus.block_reason && (
                  <div className="rounded-md border border-slate-200 bg-slate-50 p-3 text-sm text-slate-700">
                    当前暂不可重新提交：{applymentStatus.block_reason}
                  </div>
                )}

                {applymentStatus.reject_reason && (
                  <div className="rounded-md border border-rose-200 bg-rose-50 p-3 text-sm text-rose-700">
                    <span className="font-medium">拒绝原因：</span>
                    {applymentStatus.reject_reason}
                  </div>
                )}

                {(applymentStatus.status === "finish" ||
                  applymentStatus.status === "active") && (
                  <div className="flex items-center gap-2 text-sm text-emerald-600">
                    <CheckCircle2 className="h-4 w-4 shrink-0" />
                    <span>收付通已开通，可正常收款和提现。</span>
                  </div>
                )}

                {applymentStatus.can_submit &&
                  applymentStatus.status !== "finish" &&
                  applymentStatus.status !== "active" &&
                  !showBindForm && (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setShowBindForm(true)}
                  >
                    重新提交银行账户信息
                  </Button>
                  )}

                {showBindForm && (
                  <ApplymentBankForm
                    apiBasePath="/merchant/applyment"
                    defaultAccountType="ACCOUNT_TYPE_BUSINESS"
                    submitting={submittingBind}
                    submitLabel="重新提交银行账户信息"
                    onSubmit={handleSubmitBindBank}
                    onCancel={() => setShowBindForm(false)}
                  />
                )}
                    </>
                  );
                })()}
              </div>
            )}
          </CardContent>
        </Card>

        {/* ── Balance & withdraw (hidden when not configured) ── */}
        {notConfigured ? (
          <div className="flex items-center gap-2 rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-700">
            <XCircle className="h-4 w-4 shrink-0" />
            收付通账户尚未激活，完成进件签约后余额数据将自动显示。
          </div>
        ) : (
          <>
            <div className="grid gap-4 md:grid-cols-3">
              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm font-medium text-muted-foreground">
                    可用余额
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  {loadingBalance ? (
                    <Skeleton className="h-8 w-28" />
                  ) : (
                    <div className="text-2xl font-bold">
                      ¥{formatAmount(balance?.available_amount)}
                    </div>
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm font-medium text-muted-foreground">
                    可提现余额
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  {loadingBalance ? (
                    <Skeleton className="h-8 w-28" />
                  ) : (
                    <div className="text-2xl font-bold">
                      ¥{formatAmount(balance?.withdrawable_amount)}
                    </div>
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm font-medium text-muted-foreground">
                    处理中金额
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  {loadingBalance ? (
                    <Skeleton className="h-8 w-28" />
                  ) : (
                    <div className="text-2xl font-bold">
                      ¥{formatAmount(balance?.pending_amount)}
                    </div>
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
                    <p className="text-xs text-muted-foreground">
                      最小提现金额 ¥1.00
                    </p>
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
                    子商户号：{balance?.sub_mch_id || "-"}
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
                          <TableCell>
                            <Skeleton className="h-4 w-32" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="h-4 w-16" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="h-4 w-16" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="h-4 w-36" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="h-4 w-28" />
                          </TableCell>
                        </TableRow>
                      ))
                    ) : withdrawals.length === 0 ? (
                      <TableRow>
                        <TableCell
                          colSpan={5}
                          className="text-center text-sm text-muted-foreground"
                        >
                          暂无提现记录
                        </TableCell>
                      </TableRow>
                    ) : (
                      withdrawals.map((item) => {
                        const meta = withdrawStatusMeta[item.status] ?? {
                          label: item.status,
                          className:
                            "bg-slate-100 text-slate-700 border-slate-200",
                        };
                        return (
                          <TableRow key={item.id}>
                            <TableCell className="font-mono text-xs">
                              {item.out_request_no || "-"}
                            </TableCell>
                            <TableCell>
                              ¥{formatAmount(item.amount)}
                            </TableCell>
                            <TableCell>
                              <Badge className={meta.className}>
                                {meta.label}
                              </Badge>
                            </TableCell>
                            <TableCell className="max-w-75 truncate text-muted-foreground">
                              {item.reason || "-"}
                            </TableCell>
                            <TableCell className="text-xs text-muted-foreground">
                              {item.created_at
                                ? new Date(item.created_at).toLocaleString(
                                    "zh-CN"
                                  )
                                : "-"}
                            </TableCell>
                          </TableRow>
                        );
                      })
                    )}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </>
        )}
      </PageContent>
    </PageShell>
  );
}
