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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Skeleton } from "@/components/ui/skeleton";
import { Separator } from "@/components/ui/separator";

/* ─────────────────────────── Interfaces ─────────────────────────── */

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

interface ApplymentStatusResponse {
  status: string;
  status_desc: string;
  sign_url?: string;
  sub_mch_id?: string;
  reject_reason?: string;
}

interface BindBankFormData {
  account_type: "ACCOUNT_TYPE_BUSINESS" | "ACCOUNT_TYPE_PRIVATE";
  account_bank: string;
  bank_address_code: string;
  bank_name: string;
  account_number: string;
  account_name: string;
  contact_phone: string;
  contact_email: string;
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
    label: "进件审核中",
    className: "bg-blue-100 text-blue-700 border-blue-200",
  },
  auditing: {
    label: "审核中",
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
};

function is404(error: unknown): boolean {
  return error instanceof Error && error.message.includes("404");
}

const emptyForm: BindBankFormData = {
  account_type: "ACCOUNT_TYPE_BUSINESS",
  account_bank: "",
  bank_address_code: "",
  bank_name: "",
  account_number: "",
  account_name: "",
  contact_phone: "",
  contact_email: "",
};

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
  const [bindForm, setBindForm] = useState<BindBankFormData>(emptyForm);

  const pageSize = 20;

  /* ── Loaders ── */

  const loadBalance = useCallback(async () => {
    setLoadingBalance(true);
    try {
      const data = await apiGet<MerchantAccountBalance>(
        "/merchant/finance/account/balance"
      );
      setBalance(data);
      setNotConfigured(false);
    } catch (error: unknown) {
      if (is404(error)) {
        setNotConfigured(true);
      } else {
        toast.error(
          error instanceof Error ? error.message : "加载账户余额失败"
        );
      }
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
      if (!is404(error)) {
        toast.error(
          error instanceof Error ? error.message : "加载提现记录失败"
        );
      }
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
      setApplymentStatus(data);
      setShowBindForm(false);
    } catch (error: unknown) {
      if (is404(error)) {
        setApplymentStatus(null);
      } else {
        toast.error(
          error instanceof Error ? error.message : "查询进件状态失败"
        );
        setApplymentStatus(null);
      }
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

  const bindFormValid =
    bindForm.account_bank.trim() &&
    bindForm.bank_address_code.trim() &&
    bindForm.account_number.trim() &&
    bindForm.account_name.trim() &&
    bindForm.contact_phone.trim();

  const handleSubmitBindBank = async () => {
    if (!bindFormValid || submittingBind) return;
    setSubmittingBind(true);
    try {
      await apiPost("/merchant/applyment/bindbank", {
        account_type: bindForm.account_type,
        account_bank: bindForm.account_bank.trim(),
        bank_address_code: bindForm.bank_address_code.trim(),
        bank_name: bindForm.bank_name.trim() || undefined,
        account_number: bindForm.account_number.trim(),
        account_name: bindForm.account_name.trim(),
        contact_phone: bindForm.contact_phone.trim(),
        contact_email: bindForm.contact_email.trim() || undefined,
      });
      toast.success("银行账户信息已提交，请等待微信审核");
      setBindForm(emptyForm);
      await loadApplymentStatus();
    } catch (error: unknown) {
      toast.error(
        error instanceof Error ? error.message : "提交银行账户信息失败"
      );
    } finally {
      setSubmittingBind(false);
    }
  };

  const updateBindForm = (field: keyof BindBankFormData, value: string) => {
    setBindForm((prev) => ({ ...prev, [field]: value }));
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
            ) : applymentStatus === null ? (
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
                  <BindBankForm
                    form={bindForm}
                    valid={!!bindFormValid}
                    submitting={submittingBind}
                    onChange={updateBindForm}
                    onSubmit={handleSubmitBindBank}
                    onCancel={() => setShowBindForm(false)}
                  />
                )}
              </div>
            ) : (
              <div className="space-y-3">
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
                </div>

                {applymentStatus.sign_url && (
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

                {applymentStatus.status === "rejected" && !showBindForm && (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setShowBindForm(true)}
                  >
                    重新提交银行账户信息
                  </Button>
                )}

                {showBindForm && (
                  <div className="pt-2">
                    <Separator className="mb-4" />
                    <BindBankForm
                      form={bindForm}
                      valid={!!bindFormValid}
                      submitting={submittingBind}
                      onChange={updateBindForm}
                      onSubmit={handleSubmitBindBank}
                      onCancel={() => setShowBindForm(false)}
                    />
                  </div>
                )}
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

/* ─────────────────────────── BindBankForm ─────────────────────────── */

interface BindBankFormProps {
  form: BindBankFormData;
  valid: boolean;
  submitting: boolean;
  onChange: (field: keyof BindBankFormData, value: string) => void;
  onSubmit: () => void;
  onCancel: () => void;
}

function BindBankForm({
  form,
  valid,
  submitting,
  onChange,
  onSubmit,
  onCancel,
}: BindBankFormProps) {
  return (
    <div className="space-y-5 rounded-lg border bg-muted/30 p-5">
      <div className="text-sm font-medium">填写银行结算账户</div>

      <div className="space-y-2">
        <Label>账户类型</Label>
        <Select
          value={form.account_type}
          onValueChange={(val) =>
            onChange(
              "account_type",
              val as "ACCOUNT_TYPE_BUSINESS" | "ACCOUNT_TYPE_PRIVATE"
            )
          }
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="ACCOUNT_TYPE_BUSINESS">
              对公账户（企业银行账户）
            </SelectItem>
            <SelectItem value="ACCOUNT_TYPE_PRIVATE">
              对私账户（个人银行卡）
            </SelectItem>
          </SelectContent>
        </Select>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="bb-account-name">
            开户名称 <span className="text-destructive">*</span>
          </Label>
          <Input
            id="bb-account-name"
            placeholder={
              form.account_type === "ACCOUNT_TYPE_BUSINESS"
                ? "企业/机构全称"
                : "持卡人姓名"
            }
            value={form.account_name}
            onChange={(e) => onChange("account_name", e.target.value)}
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="bb-account-number">
            银行账号 <span className="text-destructive">*</span>
          </Label>
          <Input
            id="bb-account-number"
            placeholder="银行账户号码"
            value={form.account_number}
            onChange={(e) => onChange("account_number", e.target.value)}
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="bb-account-bank">
            开户银行 <span className="text-destructive">*</span>
          </Label>
          <Input
            id="bb-account-bank"
            placeholder="如：中国工商银行"
            value={form.account_bank}
            onChange={(e) => onChange("account_bank", e.target.value)}
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="bb-bank-addr">
            开户行省市编码 <span className="text-destructive">*</span>
          </Label>
          <Input
            id="bb-bank-addr"
            placeholder="如：110000（北京市）"
            value={form.bank_address_code}
            onChange={(e) => onChange("bank_address_code", e.target.value)}
          />
          <p className="text-xs text-muted-foreground">
            参考微信支付省市编码表
          </p>
        </div>

        <div className="space-y-2">
          <Label htmlFor="bb-bank-name">开户支行全称</Label>
          <Input
            id="bb-bank-name"
            placeholder="如：工商银行北京朝阳支行（选填）"
            value={form.bank_name}
            onChange={(e) => onChange("bank_name", e.target.value)}
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="bb-phone">
            联系手机 <span className="text-destructive">*</span>
          </Label>
          <Input
            id="bb-phone"
            type="tel"
            placeholder="11位手机号"
            value={form.contact_phone}
            onChange={(e) => onChange("contact_phone", e.target.value)}
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="bb-email">联系邮箱</Label>
          <Input
            id="bb-email"
            type="email"
            placeholder="选填"
            value={form.contact_email}
            onChange={(e) => onChange("contact_email", e.target.value)}
          />
        </div>
      </div>

      <div className="flex gap-2 pt-1">
        <Button onClick={onSubmit} disabled={!valid || submitting}>
          {submitting ? "提交中..." : "提交银行账户信息"}
        </Button>
        <Button variant="outline" onClick={onCancel} disabled={submitting}>
          取消
        </Button>
      </div>
    </div>
  );
}
