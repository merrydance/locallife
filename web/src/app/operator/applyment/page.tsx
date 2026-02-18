"use client";

import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { PageContent, PageHeader, PageShell } from "@/components/merchant/layout/page-shell";
import { apiGet, apiPost } from "@/lib/api";
import type {
  OperatorApplicationStatusResponse,
  OperatorApplymentStatusResponse,
  OperatorBindBankRequest,
  OperatorBindBankResponse,
} from "@/types/operator-console";

const INITIAL_FORM: OperatorBindBankRequest = {
  account_type: "ACCOUNT_TYPE_PRIVATE",
  account_bank: "",
  bank_address_code: "",
  bank_name: "",
  account_number: "",
  account_name: "",
  contact_phone: "",
  contact_email: "",
};

function isNotFoundError(message: string) {
  return message.includes("Request failed: 404") || message.includes("服务未找到") || message.includes("未找到");
}

function mapApplicationStatusToApplymentStatus(applicationStatus: string): string {
  switch (applicationStatus) {
    case "pending_bindbank":
    case "approved":
      return "pending";
    case "bindbank_submitted":
      return "submitted";
    case "bindbank_rejected":
      return "rejected";
    case "active":
      return "pending";
    default:
      return "pending";
  }
}

function mapApplymentStatusDesc(applymentStatus: string): string {
  switch (applymentStatus) {
    case "pending":
      return "待提交";
    case "submitted":
      return "已提交，等待审核";
    case "auditing":
      return "审核中";
    case "rejected":
      return "审核被拒绝";
    case "frozen":
      return "已冻结";
    case "to_be_signed":
      return "待签约，请点击签约链接完成签约";
    case "signing":
      return "签约中";
    case "rejected_sign":
      return "签约失败";
    case "finish":
      return "开户成功";
    default:
      return "未知状态";
  }
}

function mapApplicationToApplymentStatus(
  application: OperatorApplicationStatusResponse,
): OperatorApplymentStatusResponse {
  const status = mapApplicationStatusToApplymentStatus(application.status);
  return {
    status,
    status_desc: mapApplymentStatusDesc(status),
    reject_reason: application.reject_reason,
    created_at: application.created_at,
    updated_at: application.updated_at,
  };
}

export default function OperatorApplymentPage() {
  const [form, setForm] = useState<OperatorBindBankRequest>(INITIAL_FORM);
  const [status, setStatus] = useState<OperatorApplymentStatusResponse | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const loadStatus = useCallback(async () => {
    try {
      const res = await apiGet<OperatorApplymentStatusResponse>("/operator/applyment/status");
      setStatus(res);
      setError(null);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : "获取开户状态失败";
      if (isNotFoundError(message)) {
        try {
          const application = await apiGet<OperatorApplicationStatusResponse>("/operator/application");
          setStatus(mapApplicationToApplymentStatus(application));
          setError(null);
          return;
        } catch (fallbackErr: unknown) {
          const fallbackMessage = fallbackErr instanceof Error ? fallbackErr.message : "获取开户状态失败";
          setError(fallbackMessage);
          return;
        }
      }
      setError(message);
    }
  }, []);

  useEffect(() => {
    loadStatus();
  }, [loadStatus]);

  useEffect(() => {
    if (error) {
      toast.error(error);
    }
  }, [error]);

  useEffect(() => {
    if (success) {
      toast.success(success);
    }
  }, [success]);

  const statusCode = status?.status || "pending";
  const isOpened = statusCode === "finish" && Boolean(status?.sub_mch_id);
  const canSubmitOpenInfo = statusCode === "pending" || statusCode === "rejected" || statusCode === "rejected_sign";
  const isInReview = statusCode === "submitted" || statusCode === "auditing" || statusCode === "to_be_signed" || statusCode === "signing";

  const onSubmit = async () => {
    setSubmitting(true);
    setSuccess(null);
    setError(null);
    try {
      const payload: OperatorBindBankRequest = {
        ...form,
        bank_name: form.bank_name || undefined,
        contact_email: form.contact_email || undefined,
      };
      const res = await apiPost<OperatorBindBankResponse>("/operator/applyment/bindbank", payload);
      setSuccess(res.message || "开户申请已提交，请等待微信审核");
      await loadStatus();
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : "提交开户申请失败";
      setError(message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <PageShell>
      <PageHeader
        title="微信支付开户"
        description="完成运营商微信支付商户进件，开户成功后才可正常经营与提现"
        actions={<Badge variant="secondary">运营商</Badge>}
      />
      <PageContent className="space-y-4">
        <Card>
          <CardHeader>
            <CardTitle>开户状态</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            <div>状态：{status?.status_desc || status?.status || "未提交"}</div>
            <div>申请单号：{status?.applyment_id || "-"}</div>
            <div>子商户号：{status?.sub_mch_id || "-"}</div>
            <div>拒绝原因：{status?.reject_reason || "-"}</div>
            {status?.sign_url && (
              <a className="text-primary underline" href={status.sign_url} target="_blank" rel="noreferrer">
                前往微信签约
              </a>
            )}
          </CardContent>
        </Card>

        {isOpened && (
          <Card>
            <CardHeader>
              <CardTitle>已开通微信支付商户</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              <div>当前状态：开户成功，可正常经营与提现。</div>
              <div>微信二级商户号：{status?.sub_mch_id || "-"}</div>
              <div>微信申请单号：{status?.applyment_id || "-"}</div>
            </CardContent>
          </Card>
        )}

        {isInReview && !isOpened && (
          <Card>
            <CardHeader>
              <CardTitle>开户审核中</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              <div>微信支付开户信息已提交，请等待审核结果。</div>
              <div>审核期间无需重复填写开户信息。</div>
            </CardContent>
          </Card>
        )}

        {canSubmitOpenInfo && !isOpened && (
          <Card>
            <CardHeader>
              <CardTitle>提交开户信息</CardTitle>
            </CardHeader>
            <CardContent className="grid gap-4">
              <div className="rounded-lg border px-3 py-2 text-sm">
                当前尚未开通微信支付商户，请提交以下必要信息完成开户。
              </div>
            <div className="grid gap-2 md:w-72">
              <Label>账户类型</Label>
              <Select
                value={form.account_type}
                onValueChange={(value: "ACCOUNT_TYPE_BUSINESS" | "ACCOUNT_TYPE_PRIVATE") =>
                  setForm((prev) => ({ ...prev, account_type: value }))
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="ACCOUNT_TYPE_PRIVATE">对私账户</SelectItem>
                  <SelectItem value="ACCOUNT_TYPE_BUSINESS">对公账户</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="grid gap-2 md:max-w-xl">
              <Label>开户银行</Label>
              <Input
                value={form.account_bank}
                onChange={(e) => setForm((prev) => ({ ...prev, account_bank: e.target.value }))}
                placeholder="例如：中国工商银行"
              />
            </div>

            <div className="grid gap-2 md:max-w-xl">
              <Label>开户银行地区编码</Label>
              <Input
                value={form.bank_address_code}
                onChange={(e) => setForm((prev) => ({ ...prev, bank_address_code: e.target.value }))}
                placeholder="例如：440300"
              />
            </div>

            <div className="grid gap-2 md:max-w-xl">
              <Label>开户支行全称（可选）</Label>
              <Input
                value={form.bank_name || ""}
                onChange={(e) => setForm((prev) => ({ ...prev, bank_name: e.target.value }))}
                placeholder="例如：中国工商银行深圳南山支行"
              />
            </div>

            <div className="grid gap-2 md:max-w-xl">
              <Label>银行账号</Label>
              <Input
                value={form.account_number}
                onChange={(e) => setForm((prev) => ({ ...prev, account_number: e.target.value }))}
                placeholder="请输入收款银行卡号"
              />
            </div>

            <div className="grid gap-2 md:max-w-xl">
              <Label>开户名称</Label>
              <Input
                value={form.account_name}
                onChange={(e) => setForm((prev) => ({ ...prev, account_name: e.target.value }))}
                placeholder="对私填姓名；对公填公司名称"
              />
            </div>

            <div className="grid gap-2 md:max-w-xl">
              <Label>联系手机号</Label>
              <Input
                value={form.contact_phone}
                onChange={(e) => setForm((prev) => ({ ...prev, contact_phone: e.target.value }))}
                placeholder="请输入用于微信审核联系的手机号"
              />
            </div>

            <div className="grid gap-2 md:max-w-xl">
              <Label>联系邮箱（可选）</Label>
              <Input
                value={form.contact_email || ""}
                onChange={(e) => setForm((prev) => ({ ...prev, contact_email: e.target.value }))}
                placeholder="例如：ops@example.com"
              />
            </div>

            <div>
              <Button onClick={onSubmit} disabled={submitting}>
                {submitting ? "提交中..." : "提交微信支付开户"}
              </Button>
            </div>
            </CardContent>
          </Card>
        )}
      </PageContent>
    </PageShell>
  );
}
