"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { ArrowLeft, RefreshCw } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
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
import { apiGet, formatDate } from "@/lib/api";

type ReservationDishReference = {
  reservation_id: number;
  reservation_time: string;
  table_no?: string;
  contact_name?: string;
  status: string;
  quantity: number;
};

type ReservationDishSummaryItem = {
  type: "dish" | "combo";
  dish_id?: number;
  combo_id?: number;
  name: string;
  total_quantity: number;
  reservation_count: number;
  references: ReservationDishReference[];
};

type ReservationDishSummaryResponse = {
  date: string;
  items: ReservationDishSummaryItem[];
};

export function ReservationDishPrepPageClient() {
  const [date, setDate] = useState(formatDate(new Date()));
  const [loading, setLoading] = useState(true);
  const [items, setItems] = useState<ReservationDishSummaryItem[]>([]);

  const loadSummary = useCallback(async () => {
    setLoading(true);
    try {
      const data = await apiGet<ReservationDishSummaryResponse>(
        "/reservations/merchant/dishes",
        { date }
      );
      setItems(data.items || []);
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "加载备菜清单失败";
      toast.error(message);
    } finally {
      setLoading(false);
    }
  }, [date]);

  useEffect(() => {
    loadSummary();
  }, [loadSummary]);

  const summary = useMemo(() => {
    const totalSku = items.length;
    const totalQuantity = items.reduce((sum, item) => sum + item.total_quantity, 0);
    const totalReservations = items.reduce((sum, item) => sum + item.reservation_count, 0);
    return { totalSku, totalQuantity, totalReservations };
  }, [items]);

  return (
    <PageShell>
      <PageHeader
        title="预订备菜清单"
        description="按天汇总预订菜品数量，便于后厨统一备菜。"
        actions={
          <div className="flex items-center gap-2">
            <Button variant="outline" asChild>
              <Link href="/merchant/reservations">
                <ArrowLeft className="mr-2 h-4 w-4" />
                返回预订管理
              </Link>
            </Button>
            <Button variant="outline" onClick={loadSummary} disabled={loading}>
              <RefreshCw className="mr-2 h-4 w-4" />
              刷新
            </Button>
          </div>
        }
      />
      <PageContent>
        <Card>
          <CardHeader>
            <CardTitle>查询条件</CardTitle>
            <CardDescription>选择需要备菜的业务日期</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex flex-wrap items-center gap-3">
              <Input
                type="date"
                className="w-48"
                value={date}
                onChange={(event) => setDate(event.target.value)}
              />
              <Button onClick={loadSummary} disabled={loading || !date}>
                查询
              </Button>
            </div>
          </CardContent>
        </Card>

        <div className="mt-4 grid gap-4 md:grid-cols-3">
          <Card>
            <CardContent className="pt-6">
              <div className="text-sm text-muted-foreground">菜品/套餐种类</div>
              <div className="mt-1 text-2xl font-bold">{summary.totalSku}</div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="text-sm text-muted-foreground">总备菜数量</div>
              <div className="mt-1 text-2xl font-bold">{summary.totalQuantity}</div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="text-sm text-muted-foreground">覆盖预订次数</div>
              <div className="mt-1 text-2xl font-bold">{summary.totalReservations}</div>
            </CardContent>
          </Card>
        </div>

        <Card className="mt-4">
          <CardHeader>
            <CardTitle>{date} 备菜明细</CardTitle>
            <CardDescription>按总数量倒序展示，支持快速分配备菜。</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>类型</TableHead>
                  <TableHead>名称</TableHead>
                  <TableHead>总数量</TableHead>
                  <TableHead>涉及预订</TableHead>
                  <TableHead>预订分布</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading ? (
                  <TableRow>
                    <TableCell colSpan={5} className="text-center text-muted-foreground">
                      加载中...
                    </TableCell>
                  </TableRow>
                ) : items.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={5} className="text-center text-muted-foreground">
                      当日暂无可备菜预订
                    </TableCell>
                  </TableRow>
                ) : (
                  items.map((item) => (
                    <TableRow key={`${item.type}-${item.dish_id ?? item.combo_id ?? item.name}`}>
                      <TableCell>{item.type === "combo" ? "套餐" : "菜品"}</TableCell>
                      <TableCell className="font-medium">{item.name}</TableCell>
                      <TableCell>{item.total_quantity}</TableCell>
                      <TableCell>{item.reservation_count}</TableCell>
                      <TableCell>
                        <div className="max-w-130 text-xs text-muted-foreground">
                          {item.references
                            .map((ref) => {
                              const tableNo = ref.table_no ? `·${ref.table_no}` : "";
                              const contactName = ref.contact_name ? `·${ref.contact_name}` : "";
                              return `${ref.reservation_time}${tableNo}${contactName} x${ref.quantity}`;
                            })
                            .join("；")}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </PageContent>
    </PageShell>
  );
}
