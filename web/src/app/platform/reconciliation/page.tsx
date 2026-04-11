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
import type { PlatformProfitSharingReconciliationRow } from "@/types/platform-stats";

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

export default function PlatformReconciliationPage() {
  const [rows, setRows] = useState<PlatformProfitSharingReconciliationRow[]>([]);
  const [loadState, setLoadState] = useState<"loading" | "loaded" | "error">("loading");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const range = getRecentRange(30);
    apiGet<PlatformProfitSharingReconciliationRow[]>(
      "/platform/stats/profit-sharing/reconciliation",
      range
    )
      .then((data) => {
        setRows(data ?? []);
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

  return (
    <PageShell>
      <PageHeader
        title="分账复核"
        description="异常差错与分账复核处理"
        actions={<Badge variant="secondary">对账中</Badge>}
      />
      <PageContent>
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
      </PageContent>
    </PageShell>
  );
}
