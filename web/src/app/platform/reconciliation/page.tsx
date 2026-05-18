"use client";

import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  PageContent,
  PageHeader,
  PageShell,
} from "@/components/merchant/layout/page-shell";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiGet, formatAmount, getRecentRange } from "@/lib/api";
import type {
  PlatformBaofuDailyReconciliationRow,
  PlatformBaofuSettlementStatusResponse,
  PlatformProfitSharingReconciliationRow,
} from "@/types/platform-stats";

function formatReconciliationStatus(status: string) {
  switch (status) {
    case "pending":
      return "待分账";
    case "processing":
      return "分账中";
    case "success":
      return "已完成";
    case "failed":
      return "失败";
    case "reversed":
      return "已回退";
    default:
      return "未知状态";
  }
}

function formatBaofuChannel(channel: string) {
  return channel === "baofu_aggregate" ? "宝付聚合支付" : channel || "-";
}

function countBaofuAnomalies(row: PlatformBaofuDailyReconciliationRow) {
  return row.unapplied_fact_count + row.unknown_command_count + row.fee_ledger_mismatch_count;
}

export default function PlatformReconciliationPage() {
  const [rows, setRows] = useState<PlatformProfitSharingReconciliationRow[]>([]);
  const [baofuRows, setBaofuRows] = useState<PlatformBaofuDailyReconciliationRow[]>([]);
  const [settlementStatus, setSettlementStatus] = useState<PlatformBaofuSettlementStatusResponse | null>(null);
  const [loadState, setLoadState] = useState<"loading" | "loaded" | "error">("loading");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const range = getRecentRange(30);
    Promise.all([
      apiGet<PlatformProfitSharingReconciliationRow[]>(
        "/platform/stats/profit-sharing/reconciliation",
        range
      ),
      apiGet<PlatformBaofuDailyReconciliationRow[]>(
        "/platform/stats/baofu/reconciliation/daily",
        range
      ),
      apiGet<PlatformBaofuSettlementStatusResponse>(
        "/platform/finance/settlement-account/status"
      ),
    ])
      .then((data) => {
        setRows(data[0] ?? []);
        setBaofuRows(data[1] ?? []);
        setSettlementStatus(data[2] ?? null);
        setLoadState("loaded");
      })
      .catch((err: unknown) => {
        const message = err instanceof Error ? err.message : "加载失败";
        setError(message);
        setLoadState("error");
      })
  }, []);

  useEffect(() => {
    if (error) {
      toast.error(error);
    }
  }, [error]);

  const loading = loadState === "loading";
  const baofuAnomalyCount = baofuRows.reduce((sum, row) => sum + countBaofuAnomalies(row), 0);

  return (
    <PageShell>
      <PageHeader
        title="分账复核"
        description="异常差错与分账复核处理"
        actions={<Badge variant="secondary">对账中</Badge>}
      />
      <PageContent>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <Card>
            <CardHeader className="pb-2">
              <CardDescription>宝付支付手续费</CardDescription>
              <CardTitle>
                ¥{formatAmount(baofuRows.reduce((sum, row) => sum + row.payment_fee, 0))}
              </CardTitle>
            </CardHeader>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardDescription>提现处理中</CardDescription>
              <CardTitle>
                ¥{formatAmount(baofuRows.reduce((sum, row) => sum + row.withdraw_processing_amount, 0))}
              </CardTitle>
            </CardHeader>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardDescription>对账异常计数</CardDescription>
              <CardTitle>{baofuAnomalyCount}</CardTitle>
            </CardHeader>
          </Card>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>平台宝付佣金接收方</CardTitle>
            <CardDescription>平台 2% 佣金必须分账到平台名下宝付二级户，不使用平台收款商户号</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-3 text-sm md:grid-cols-3">
            <div>
              <div className="text-muted-foreground">结算账户状态</div>
              <div className="mt-1 font-medium">
                {settlementStatus?.settlement_account?.label || "待同步"}
              </div>
            </div>
            <div>
              <div className="text-muted-foreground">合同号</div>
              <div className="mt-1 font-mono">
                {settlementStatus?.masked_contract_no || "-"}
              </div>
            </div>
            <div>
              <div className="text-muted-foreground">分账接收方</div>
              <div className="mt-1 font-mono">
                {settlementStatus?.masked_sharing_mer_id || "-"}
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>分账对账汇总</CardTitle>
            <CardDescription>按分账状态聚合金额与订单数</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>状态</TableHead>
                  <TableHead>订单数</TableHead>
                  <TableHead>GMV</TableHead>
                  <TableHead>平台佣金</TableHead>
                  <TableHead>运营商佣金</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading && (
                  <TableRow>
                    <TableCell colSpan={5} className="text-sm text-muted-foreground">
                      加载中...
                    </TableCell>
                  </TableRow>
                )}
                {!loading && rows.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={5} className="text-sm text-muted-foreground">
                      暂无对账数据
                    </TableCell>
                  </TableRow>
                )}
                {rows.map((row) => (
                  <TableRow key={row.status}>
                    <TableCell className="font-medium">{formatReconciliationStatus(row.status)}</TableCell>
                    <TableCell>{row.total_orders}</TableCell>
                    <TableCell>¥{formatAmount(row.total_amount)}</TableCell>
                    <TableCell>¥{formatAmount(row.total_platform_commission)}</TableCell>
                    <TableCell>¥{formatAmount(row.total_operator_commission)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>宝付每日资金复核</CardTitle>
            <CardDescription>按日期聚合手续费、提现状态和异常计数；不展示上游账户号</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>日期</TableHead>
                  <TableHead>通道</TableHead>
                  <TableHead>支付金额</TableHead>
                  <TableHead>支付手续费</TableHead>
                  <TableHead>商户分账</TableHead>
                  <TableHead>骑手分账</TableHead>
                  <TableHead>提现成功</TableHead>
                  <TableHead>提现中</TableHead>
                  <TableHead>异常</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading && (
                  <TableRow>
                    <TableCell colSpan={9} className="text-sm text-muted-foreground">
                      加载中...
                    </TableCell>
                  </TableRow>
                )}
                {!loading && baofuRows.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={9} className="text-sm text-muted-foreground">
                      暂无宝付对账数据
                    </TableCell>
                  </TableRow>
                )}
                {baofuRows.map((row) => {
                  const anomalyCount = countBaofuAnomalies(row);
                  return (
                    <TableRow key={`${row.date}-${row.provider}-${row.channel}`}>
                      <TableCell>{row.date}</TableCell>
                      <TableCell>{formatBaofuChannel(row.channel)}</TableCell>
                      <TableCell>¥{formatAmount(row.paid_amount)}</TableCell>
                      <TableCell>¥{formatAmount(row.payment_fee)}</TableCell>
                      <TableCell>¥{formatAmount(row.merchant_amount)}</TableCell>
                      <TableCell>¥{formatAmount(row.rider_amount)}</TableCell>
                      <TableCell>¥{formatAmount(row.withdraw_succeeded_amount)}</TableCell>
                      <TableCell>¥{formatAmount(row.withdraw_processing_amount)}</TableCell>
                      <TableCell>
                        <Badge variant={anomalyCount > 0 ? "destructive" : "secondary"}>
                          {anomalyCount > 0 ? `${anomalyCount} 项` : "正常"}
                        </Badge>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </PageContent>
    </PageShell>
  );
}
