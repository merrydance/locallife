"use client";

import { useEffect, useState } from "react";
import { RefreshCw } from "lucide-react";
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
import {
  PageContent,
  PageHeader,
  PageShell,
} from "@/components/merchant/layout/page-shell";
import { apiGet } from "@/lib/api";
import type { BillReconciliationReport } from "@/types/platform-stats";

function billTypeLabel(t: string) {
  switch (t) {
    case "trade":
      return "小程序交易";
    case "ecommerce_trade":
      return "合单交易";
    case "refund":
      return "退款";
    default:
      return t;
  }
}

function formatBillReconciliationStatus(status: string): string {
  switch (status) {
    case "pending":
      return "待对账";
    case "running":
      return "对账中";
    case "completed":
      return "已完成";
    case "failed":
      return "失败";
    default:
      return status;
  }
}

function StatusBadge({
  status,
  mismatchCount,
}: {
  status: string;
  mismatchCount: number;
}) {
  if (status === "completed" && mismatchCount === 0) {
    return (
      <Badge
        variant="secondary"
        className="bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300"
      >
        已完成
      </Badge>
    );
  }
  if (status === "completed" && mismatchCount > 0) {
    return (
      <Badge variant="destructive">有差异 {mismatchCount} 笔</Badge>
    );
  }
  return (
    <Badge variant={status === "failed" ? "destructive" : "outline"}>
      {formatBillReconciliationStatus(status)}
    </Badge>
  );
}

export default function BillReconciliationPage() {
  const [reports, setReports] = useState<BillReconciliationReport[]>([]);
  const [loadState, setLoadState] = useState<"loading" | "loaded" | "error">(
    "loading"
  );
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);

  useEffect(() => {
    apiGet<BillReconciliationReport[]>("/platform/stats/bill-reconciliation", {
      page_id: 1,
      page_size: 90,
    })
      .then((data) => {
        setReports(data ?? []);
        setLoadState("loaded");
      })
      .catch((err: unknown) => {
        setErrorMsg(err instanceof Error ? err.message : "加载失败");
        setLoadState("error");
      });
  }, [refreshKey]);

  return (
    <PageShell>
      <PageHeader
        title="账单对账"
        description="每日自动拉取微信支付账单与本地记录比对，10:00 运行"
        actions={
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              setLoadState("loading");
              setRefreshKey((k) => k + 1);
            }}
            disabled={loadState === "loading"}
          >
            <RefreshCw
              className={`mr-2 h-4 w-4 ${loadState === "loading" ? "animate-spin" : ""}`}
            />
            刷新
          </Button>
        }
      />
      <PageContent>
        <Card>
          <CardHeader>
            <CardTitle>对账报告</CardTitle>
            <CardDescription>
              每天 10:00 自动运行，对账 T-1 账单（微信账单 T+1 9:00 后可用）
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>日期</TableHead>
                  <TableHead>账单类型</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead className="text-right">微信笔数</TableHead>
                  <TableHead className="text-right">本地笔数</TableHead>
                  <TableHead className="text-right">差异笔数</TableHead>
                  <TableHead>错误信息</TableHead>
                  <TableHead>运行时间</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loadState === "loading" && (
                  <TableRow>
                    <TableCell
                      colSpan={8}
                      className="text-sm text-muted-foreground"
                    >
                      加载中...
                    </TableCell>
                  </TableRow>
                )}
                {loadState === "error" && (
                  <TableRow>
                    <TableCell colSpan={8} className="text-sm text-destructive">
                      {errorMsg}
                    </TableCell>
                  </TableRow>
                )}
                {loadState === "loaded" && reports.length === 0 && (
                  <TableRow>
                    <TableCell
                      colSpan={8}
                      className="text-sm text-muted-foreground"
                    >
                      暂无对账数据（首次对账将在明日 10:00 运行）
                    </TableCell>
                  </TableRow>
                )}
                {reports.map((r) => (
                  <TableRow key={r.id}>
                    <TableCell className="font-medium">{r.bill_date}</TableCell>
                    <TableCell>{billTypeLabel(r.bill_type)}</TableCell>
                    <TableCell>
                      <StatusBadge
                        status={r.status}
                        mismatchCount={r.mismatch_count}
                      />
                    </TableCell>
                    <TableCell className="text-right">{r.wxpay_count}</TableCell>
                    <TableCell className="text-right">{r.local_count}</TableCell>
                    <TableCell className="text-right font-medium">
                      {r.mismatch_count > 0 ? (
                        <span className="text-destructive">
                          {r.mismatch_count}
                        </span>
                      ) : (
                        r.mismatch_count
                      )}
                    </TableCell>
                    <TableCell
                      className="max-w-48 truncate text-sm text-muted-foreground"
                      title={r.error_message ?? ""}
                    >
                      {r.error_message ?? "—"}
                    </TableCell>
                    <TableCell className="whitespace-nowrap text-sm text-muted-foreground">
                      {r.created_at}
                    </TableCell>
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
