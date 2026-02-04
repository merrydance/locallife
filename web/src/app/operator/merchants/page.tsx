"use client";

import { useEffect, useState } from "react";
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
import type { OperatorMerchantRankingRow } from "@/types/operator-stats";

export default function OperatorMerchantsPage() {
  const [rows, setRows] = useState<OperatorMerchantRankingRow[]>([]);
  const [loadState, setLoadState] = useState<"loading" | "loaded" | "error">("loading");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const range = getRecentRange(30);
    apiGet<OperatorMerchantRankingRow[]>("/operator/merchants/ranking", {
      ...range,
      page: 1,
      limit: 20,
    })
      .then((data) => {
        setRows(data ?? []);
        setLoadState("loaded");
      })
      .catch((err: unknown) => {
        const message = err instanceof Error ? err.message : "加载失败";
        setError(message);
        setLoadState("error");
      });
  }, []);

  const loading = loadState === "loading";

  return (
    <PageShell>
      <PageHeader
        title="商户排行"
        description="区域内商户销售排行"
        actions={<Badge variant="secondary">近 30 天</Badge>}
      />
      <PageContent>
        {error && (
          <div className="mb-4 rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
            {error}
          </div>
        )}
        <Card>
          <CardHeader>
            <CardTitle>商户销售排行</CardTitle>
            <CardDescription>按 GMV 与订单量综合排序</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>商户</TableHead>
                  <TableHead>订单数</TableHead>
                  <TableHead>GMV</TableHead>
                  <TableHead>平台佣金</TableHead>
                  <TableHead>平均客单</TableHead>
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
                      暂无排行数据
                    </TableCell>
                  </TableRow>
                )}
                {rows.map((row) => (
                  <TableRow key={row.merchant_id}>
                    <TableCell className="font-medium">{row.merchant_name}</TableCell>
                    <TableCell>{row.order_count}</TableCell>
                    <TableCell>¥{formatAmount(row.total_sales)}</TableCell>
                    <TableCell>¥{formatAmount(row.total_commission)}</TableCell>
                    <TableCell>¥{formatAmount(row.avg_order_amount)}</TableCell>
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
