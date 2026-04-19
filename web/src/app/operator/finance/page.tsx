"use client";

import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { PageContent, PageHeader, PageShell } from "@/components/merchant/layout/page-shell";
import { apiGet, apiPost, formatAmount, getRecentRange } from "@/lib/api";
import { formatProfitConfigStatus, formatWithdrawalStatus } from "@/lib/operator-display";
import type {
  OperatorAccountBalanceResponse,
  OperatorApplicationStatusResponse,
  OperatorCommissionResponse,
  OperatorProfitSharingConfigListResponse,
  OperatorWithdrawalItem,
  OperatorWithdrawalsResponse,
} from "@/types/operator-console";
import type { OperatorFinanceOverviewResponse } from "@/types/operator-stats";

export default function OperatorFinancePage() {
  const [overview, setOverview] = useState<OperatorFinanceOverviewResponse | null>(null);
  const [balance, setBalance] = useState<OperatorAccountBalanceResponse | null>(null);
  const [commission, setCommission] = useState<OperatorCommissionResponse | null>(null);
  const [configs, setConfigs] = useState<OperatorProfitSharingConfigListResponse | null>(null);
  const [withdrawals, setWithdrawals] = useState<OperatorWithdrawalItem[]>([]);
  const [selectedWithdrawalId, setSelectedWithdrawalId] = useState("");
  const [selectedWithdrawal, setSelectedWithdrawal] = useState<OperatorWithdrawalItem | null>(null);
  const [financeLocked, setFinanceLocked] = useState(false);
  const [financeNotice, setFinanceNotice] = useState<string | null>(null);
  const [financeError, setFinanceError] = useState<string | null>(null);

  const [withdrawAmount, setWithdrawAmount] = useState("");
  const [withdrawRemark, setWithdrawRemark] = useState("");
  const [success, setSuccess] = useState<string | null>(null);

  const load = () => {
    const range = getRecentRange(30);
    Promise.allSettled([
      apiGet<OperatorFinanceOverviewResponse>("/operators/me/finance/overview"),
      apiGet<OperatorCommissionResponse>("/operators/me/commission", { ...range, page: 1, limit: 20 }),
      apiGet<OperatorProfitSharingConfigListResponse>("/operators/me/profit-sharing/configs", {
        page: 1,
        limit: 20,
      }),
      apiGet<OperatorApplicationStatusResponse>("/operator/application"),
    ])
      .then(async ([overviewResult, commissionResult, configResult, operatorApplicationResult]) => {
        const overviewRes = overviewResult.status === "fulfilled" ? overviewResult.value : null;
        const commissionRes = commissionResult.status === "fulfilled" ? commissionResult.value : null;
        const configRes = configResult.status === "fulfilled" ? configResult.value : null;
        const operatorApplication =
          operatorApplicationResult.status === "fulfilled" ? operatorApplicationResult.value : null;

        const shouldLoadPaymentFinance = operatorApplication?.status === "active";

        let balanceRes: OperatorAccountBalanceResponse | null = null;
        let withdrawalsRes: OperatorWithdrawalsResponse | null = null;
        let nextFinanceLocked = !shouldLoadPaymentFinance;
        let nextFinanceNotice: string | null = null;
        let nextFinanceError: string | null = null;

        if (shouldLoadPaymentFinance) {
          const [balanceResult, withdrawalsResult] = await Promise.allSettled([
            apiGet<OperatorAccountBalanceResponse>("/operators/me/finance/account/balance"),
            apiGet<OperatorWithdrawalsResponse>("/operators/me/finance/withdrawals", { page: 1, limit: 20 }),
          ]);

          if (balanceResult.status === "fulfilled") {
            balanceRes = balanceResult.value;
            if ((balanceRes.account_status ?? "active") !== "active") {
              nextFinanceLocked = true;
              nextFinanceNotice =
                balanceRes.status_desc || "当前运营账号支付配置未生效，请联系平台处理。";
            }
          }
          if (withdrawalsResult.status === "fulfilled") {
            withdrawalsRes = withdrawalsResult.value;
            if (!nextFinanceNotice && (withdrawalsRes.account_status ?? "active") !== "active") {
              nextFinanceLocked = true;
              nextFinanceNotice =
                withdrawalsRes.status_desc || "当前运营账号支付配置未生效，请联系平台处理。";
            }
          }

          if (balanceResult.status === "rejected" || withdrawalsResult.status === "rejected") {
            const balanceMessage =
              balanceResult.status === "rejected" && balanceResult.reason instanceof Error
                ? balanceResult.reason.message
                : "";
            const withdrawalsMessage =
              withdrawalsResult.status === "rejected" && withdrawalsResult.reason instanceof Error
                ? withdrawalsResult.reason.message
                : "";
            const blockedByPaymentConfig =
              balanceMessage.includes("operator payment config is not active") ||
              withdrawalsMessage.includes("operator payment config is not active");
            if (blockedByPaymentConfig) {
              nextFinanceLocked = true;
              nextFinanceNotice = "当前运营账号支付配置未生效，请联系平台处理。";
            } else {
              nextFinanceLocked = true;
              nextFinanceError = "财务数据加载失败，请稍后重试。";
            }
          }
        }

        setOverview(overviewRes);
        setBalance(balanceRes);
        setCommission(commissionRes);
        setWithdrawals(withdrawalsRes?.withdrawals ?? []);
        setConfigs(configRes);
        setFinanceLocked(nextFinanceLocked || !(balanceRes?.sub_mch_id || nextFinanceNotice));
        setFinanceNotice(nextFinanceNotice);
        setFinanceError(nextFinanceError);
      });
  };

  useEffect(() => {
    load();
  }, []);

  const withdraw = async () => {
    setSuccess(null);
    await apiPost("/operators/me/finance/withdraw", {
      amount: Number(withdrawAmount),
      remark: withdrawRemark || undefined,
    });
    setSuccess("提现申请已提交");
    setWithdrawAmount("");
    setWithdrawRemark("");
    load();
  };

  const queryWithdrawal = async () => {
    if (!selectedWithdrawalId) return;
    const res = await apiGet<{ withdrawal: OperatorWithdrawalItem }>(
      `/operators/me/finance/withdrawals/${selectedWithdrawalId}`
    );
    setSelectedWithdrawal(res.withdrawal);
  };

  return (
    <PageShell>
      <PageHeader
        title="财务管理"
        description="平台佣金明细、分账规则与提现"
        actions={<Badge variant="secondary">运营商</Badge>}
      />
      <PageContent className="space-y-4">
        {success && (
          <div className="rounded-lg border border-emerald-300 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
            {success}
          </div>
        )}
        {financeError && (
          <div className="rounded-lg border border-rose-300 bg-rose-50 px-4 py-3 text-sm text-rose-700">
            {financeError}
          </div>
        )}
        <Tabs defaultValue="overview" className="space-y-4">
          <TabsList className="grid w-full grid-cols-3 lg:w-auto lg:inline-grid">
            <TabsTrigger value="overview">概览</TabsTrigger>
            <TabsTrigger value="finance">账户与提现</TabsTrigger>
            <TabsTrigger value="config">分账规则</TabsTrigger>
          </TabsList>

          <TabsContent value="overview" className="space-y-4 m-0">
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm text-muted-foreground">当月运营商收入</CardTitle>
                </CardHeader>
                <CardContent className="text-2xl font-semibold">
                  ¥{formatAmount(overview?.current_month.operator_income ?? 0)}
                </CardContent>
              </Card>
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm text-muted-foreground">当月平台佣金</CardTitle>
                </CardHeader>
                <CardContent className="text-2xl font-semibold">
                  ¥{formatAmount(overview?.current_month.total_commission ?? 0)}
                </CardContent>
              </Card>
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm text-muted-foreground">累计运营商收入</CardTitle>
                </CardHeader>
                <CardContent className="text-2xl font-semibold">
                  ¥{formatAmount(overview?.total.operator_income ?? 0)}
                </CardContent>
              </Card>
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm text-muted-foreground">可提现余额</CardTitle>
                </CardHeader>
                <CardContent className="text-2xl font-semibold">
                  ¥{formatAmount(balance?.withdrawable_amount ?? 0)}
                </CardContent>
              </Card>
            </div>

            <Card>
              <CardHeader>
                <CardTitle>平台佣金明细（近30天）</CardTitle>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>日期</TableHead>
                      <TableHead>订单数</TableHead>
                      <TableHead>GMV</TableHead>
                      <TableHead>平台佣金率</TableHead>
                      <TableHead>平台佣金</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(commission?.items ?? []).map((item) => (
                      <TableRow key={item.date}>
                        <TableCell>{item.date}</TableCell>
                        <TableCell>{item.order_count}</TableCell>
                        <TableCell>¥{formatAmount(item.total_gmv)}</TableCell>
                        <TableCell>{item.commission_rate}</TableCell>
                        <TableCell>¥{formatAmount(item.commission)}</TableCell>
                      </TableRow>
                    ))}
                    {(commission?.items?.length ?? 0) === 0 && (
                      <TableRow>
                        <TableCell colSpan={5} className="text-muted-foreground">
                          暂无可展示数据
                        </TableCell>
                      </TableRow>
                    )}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="finance" className="space-y-4 m-0">
            {financeNotice && (
              <div className="rounded-lg border px-4 py-3 text-sm">
                {financeNotice}
              </div>
            )}

            <Card>
              <CardHeader>
                <CardTitle>收付通账户</CardTitle>
              </CardHeader>
              <CardContent className="grid gap-2 text-sm md:grid-cols-2">
                <div>子商户号：{balance?.sub_mch_id || "-"}</div>
                <div>可用余额：¥{formatAmount(balance?.available_amount ?? 0)}</div>
                <div>在途金额：¥{formatAmount(balance?.pending_amount ?? 0)}</div>
                <div>可提现：¥{formatAmount(balance?.withdrawable_amount ?? 0)}</div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>提现</CardTitle>
              </CardHeader>
              <CardContent className="flex flex-col gap-3 md:flex-row md:items-center">
                <Input
                  value={withdrawAmount}
                  onChange={(e) => setWithdrawAmount(e.target.value)}
                  placeholder="提现金额（分，最小100）"
                  className="w-full md:w-64"
                />
                <Input
                  value={withdrawRemark}
                  onChange={(e) => setWithdrawRemark(e.target.value)}
                  placeholder="备注（可选）"
                  className="w-full md:w-64"
                />
                <Button onClick={withdraw} disabled={financeLocked}>
                  提交提现
                </Button>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>提现记录</CardTitle>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>ID</TableHead>
                      <TableHead>金额</TableHead>
                      <TableHead>状态</TableHead>
                      <TableHead>渠道</TableHead>
                      <TableHead>申请时间</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {withdrawals.map((item) => (
                      <TableRow key={item.id}>
                        <TableCell>{item.id}</TableCell>
                        <TableCell>¥{formatAmount(item.amount)}</TableCell>
                        <TableCell>{formatWithdrawalStatus(item.status)}</TableCell>
                        <TableCell>{item.channel}</TableCell>
                        <TableCell>{new Date(item.created_at).toLocaleString("zh-CN")}</TableCell>
                      </TableRow>
                    ))}
                    {withdrawals.length === 0 && (
                      <TableRow>
                        <TableCell colSpan={5} className="text-muted-foreground">
                          暂无记录
                        </TableCell>
                      </TableRow>
                    )}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>单笔提现查询</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="flex flex-col gap-3 md:flex-row md:items-center">
                  <Input
                    value={selectedWithdrawalId}
                    onChange={(e) => setSelectedWithdrawalId(e.target.value)}
                    placeholder="提现记录ID"
                    className="w-full md:w-56"
                  />
                  <Button variant="outline" onClick={queryWithdrawal}>
                    查询并同步状态
                  </Button>
                </div>
                {selectedWithdrawal && (
                  <div className="grid gap-2 rounded-lg border p-4 text-sm md:grid-cols-2">
                    <div>ID：{selectedWithdrawal.id}</div>
                    <div>状态：{formatWithdrawalStatus(selectedWithdrawal.status)}</div>
                    <div>金额：¥{formatAmount(selectedWithdrawal.amount)}</div>
                    <div>外部单号：{selectedWithdrawal.out_request_no || "-"}</div>
                    <div>微信提现单号：{selectedWithdrawal.withdraw_id || "-"}</div>
                    <div>失败原因：{selectedWithdrawal.reason || "-"}</div>
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="config" className="space-y-4 m-0">
            <Card>
              <CardHeader>
                <CardTitle>分账规则配置</CardTitle>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>ID</TableHead>
                      <TableHead>状态</TableHead>
                      <TableHead>来源</TableHead>
                      <TableHead>平台费率</TableHead>
                      <TableHead>运营商费率</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(configs?.items ?? []).map((item) => (
                      <TableRow key={item.id}>
                        <TableCell>{item.id}</TableCell>
                        <TableCell>{formatProfitConfigStatus(item.status)}</TableCell>
                        <TableCell>{item.order_source}</TableCell>
                        <TableCell>{item.platform_rate}</TableCell>
                        <TableCell>{item.operator_rate}</TableCell>
                      </TableRow>
                    ))}
                    {(configs?.items?.length ?? 0) === 0 && (
                      <TableRow>
                        <TableCell colSpan={5} className="text-muted-foreground">
                          暂无可展示数据
                        </TableCell>
                      </TableRow>
                    )}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </PageContent>
    </PageShell>
  );
}
