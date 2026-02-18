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
import type { OperatorRiderRankingRow } from "@/types/operator-stats";

export default function OperatorRidersPage() {
  const [rows, setRows] = useState<OperatorRiderRankingRow[]>([]);
  const [loadState, setLoadState] = useState<"loading" | "loaded" | "error">("loading");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const range = getRecentRange(30);
    apiGet<OperatorRiderRankingRow[]>("/operator/riders/ranking", {
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

  useEffect(() => {
    if (error) {
      toast.error(error);
    }
  }, [error]);

  const loading = loadState === "loading";

  return (
    <PageShell>
      <PageHeader
        title="骑手排行"
        description="区域内骑手履约与收入排行"
        actions={<Badge variant="secondary">近 30 天</Badge>}
      />
      <PageContent className="space-y-4">
        <Card>
          <CardHeader>
            <CardTitle>骑手绩效排行</CardTitle>
            <CardDescription>以配送完成率与收入综合排序</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>骑手</TableHead>
                  <TableHead>配送单量</TableHead>
                  <TableHead>完成率</TableHead>
                  <TableHead>平均时长</TableHead>
                  <TableHead>收入</TableHead>
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
                  <TableRow key={row.rider_id}>
                    <TableCell className="font-medium">{row.rider_name}</TableCell>
                    <TableCell>{row.delivery_count}</TableCell>
                    <TableCell>{row.completion_rate.toFixed(1)}%</TableCell>
                    <TableCell>{Math.round(row.avg_delivery_time_seconds / 60)} 分钟</TableCell>
                    <TableCell>¥{formatAmount(row.total_earnings)}</TableCell>
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
