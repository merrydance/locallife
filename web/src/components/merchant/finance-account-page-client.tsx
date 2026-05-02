"use client";

import { useCallback, useEffect, useState } from "react";
import {
  RefreshCw,
  AlertTriangle,
  Building2,
  CreditCard,
  ExternalLink,
  CheckCircle2,
  Clock,
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
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { ApplymentBankForm } from "@/components/applyment/applyment-bank-form";
import { getUserFacingErrorMessage } from "@/lib/user-facing-errors";
import type { ApplymentBindBankPayload } from "@/types/applyment-bank";

/* ─────────────────────────── Interfaces ─────────────────────────── */

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
  /* Applyment: undefined=loading, null=404 (not applied), object=has record */
  const [loadingApplyment, setLoadingApplyment] = useState(true);
  const [applymentStatus, setApplymentStatus] = useState<
    ApplymentStatusResponse | null | undefined
  >(undefined);
  const [applymentErrorMessage, setApplymentErrorMessage] = useState("");

  /* Bind bank form */
  const [showBindForm, setShowBindForm] = useState(false);
  const [submittingBind, setSubmittingBind] = useState(false);

  /* ── Loaders ── */

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
      setApplymentErrorMessage("");
    } catch (error: unknown) {
      console.error("Failed to load merchant applyment status", error);
      setApplymentErrorMessage(
        getUserFacingErrorMessage(
          error,
          "查询普通服务商进件状态失败，请稍后重试；如持续失败请联系平台管理员处理。",
        ),
      );
    } finally {
      setLoadingApplyment(false);
    }
  }, []);

  const refreshAll = useCallback(async () => {
    await loadApplymentStatus();
  }, [loadApplymentStatus]);

  useEffect(() => {
    refreshAll();
  }, [refreshAll]);

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
      console.error("Failed to submit merchant applyment bank account", error);
      toast.error(
        getUserFacingErrorMessage(
          error,
          "提交银行账户信息失败，请核对账户信息后重试；如持续失败请联系平台管理员处理。",
        ),
      );
    } finally {
      setSubmittingBind(false);
    }
  };

  /* ─────────────────────────── Render ─────────────────────────── */

  return (
    <PageShell>
      <PageHeader
        title="结算账户"
        description="普通服务商进件、结算账户与资金操作指引"
        actions={
          <Button variant="outline" onClick={refreshAll}>
            <RefreshCw className="mr-2 h-4 w-4" /> 刷新
          </Button>
        }
      />

      <PageContent>
        {/* ── 普通服务商进件状态 ── */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <Building2 className="h-4 w-4" /> 普通服务商进件状态
            </CardTitle>
            <CardDescription>
              微信支付特约商户进件；收款由本平台处理，资金操作在微信支付商户平台/商家助手处理
            </CardDescription>
          </CardHeader>
          <CardContent>
            {loadingApplyment ? (
              <div className="space-y-2">
                <Skeleton className="h-5 w-40" />
                <Skeleton className="h-4 w-64" />
              </div>
            ) : applymentStatus === undefined ? (
              <div className="space-y-3 rounded-lg border border-amber-200 bg-amber-50/70 p-4 text-sm text-amber-800">
                <div className="flex items-start gap-2">
                  <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                  <span>
                    {applymentErrorMessage ||
                      "暂未获取普通服务商进件状态，请重新查询；如持续失败请联系平台管理员处理。"}
                  </span>
                </div>
                <Button size="sm" variant="outline" onClick={refreshAll}>
                  重新查询进件状态
                </Button>
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
                {applymentErrorMessage && (
                  <div className="flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
                    <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                    <span>{applymentErrorMessage}</span>
                  </div>
                )}
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
                    <span>普通服务商特约商户已开通，可正常收款；余额、提现和注销提现请前往微信支付商户平台/商家助手处理。</span>
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

        <Card>
          <CardHeader>
            <CardTitle className="text-base">资金操作指引</CardTitle>
            <CardDescription>
              普通服务商模式不再通过平台发起商户余额查询、提现、注销提现、补差或垫付回补
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3 text-sm text-muted-foreground">
            <p>
              请在微信支付商户平台或微信支付商家助手查看余额、发起提现和处理注销提现。本平台仅保留普通服务商进件、开户意愿状态、结算账户查询与修改。
            </p>
            <p>
              如遇支付、退款或分账被限制，请联系平台管理员在“平台财务 - 普通服务商商户管控诊断”中查询受限能力和解脱路径。
            </p>
          </CardContent>
        </Card>
      </PageContent>
    </PageShell>
  );
}
