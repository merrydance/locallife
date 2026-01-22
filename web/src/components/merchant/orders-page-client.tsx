"use client";

import { useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Search, RefreshCw, Download, Filter, Printer, MoreVertical, Store } from "lucide-react";
import { toast } from "sonner";
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
  SheetFooter,
} from "@/components/ui/sheet";
import { Checkbox } from "@/components/ui/checkbox";
import { Separator } from "@/components/ui/separator";
import { PageShell, PageHeader, PageContent } from "@/components/merchant/layout/page-shell";
import { apiGet, apiPost, formatAmount } from "@/lib/api";
import type { OrderResponse } from "@/types/order";

const STATUS_LABELS: Record<string, string> = {
  pending: "待支付",
  paid: "待接单",
  preparing: "制作中",
  ready: "待出餐",
  courier_accepted: "骑手已接单",
  picked: "已取餐",
  delivering: "配送中",
  rider_delivered: "骑手送达",
  user_delivered: "用户确认送达",
  completed: "已完成",
  cancelled: "已取消",
};

const TYPE_LABELS: Record<string, string> = {
  takeout: "外卖",
  dine_in: "堂食",
  takeaway: "自取",
  reservation: "预订",
};

const STATUS_TABS = [
  { label: "全部", value: "" },
  { label: "待接单", value: "paid" },
  { label: "制作中", value: "preparing" },
  { label: "待出餐", value: "ready" },
  { label: "已完成", value: "completed" },
  { label: "已取消", value: "cancelled" },
];

const TYPE_OPTIONS = [
  { label: "全部类型", value: "" },
  { label: "外卖", value: "takeout" },
  { label: "堂食", value: "dine_in" },
  { label: "自取", value: "takeaway" },
  { label: "预订", value: "reservation" },
];

type Props = {
  initialOrders: OrderResponse[];
  todayOrders: number;
  statusCounts: {
    paid: number;
    preparing: number;
    ready: number;
    completed: number;
  };
  todayRevenue: number;
  page: number;
  pageSize: number;
  status: string;
  orderType: string;
  keyword: string;
};

function buildItemsSummary(order: OrderResponse) {
  if (!order.items?.length) {
    return { summary: "无商品信息", count: 0 };
  }
  const summary = order.items.slice(0, 2).map((item) => item.name).join("、");
  const suffix = order.items.length > 2 ? "..." : "";
  const count = order.items.reduce((sum, item) => sum + item.quantity, 0);
  return { summary: `${summary}${suffix}`, count };
}

export function OrdersPageClient({
  initialOrders,
  todayOrders,
  statusCounts,
  todayRevenue,
  page,
  pageSize,
  status,
  orderType,
  keyword,
}: Props) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [searchValue, setSearchValue] = useState(keyword);
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [detailOrder, setDetailOrder] = useState<OrderResponse | null>(null);
  const [loadingDetail, setLoadingDetail] = useState(false);

  const filteredOrders = useMemo(() => {
    let list = [...initialOrders];
    if (orderType) {
      list = list.filter((order) => order.order_type === orderType);
    }
    if (searchValue) {
      const text = searchValue.trim();
      if (text) {
        list = list.filter(
          (order) =>
            order.order_no.includes(text) ||
            (order.notes || "").includes(text)
        );
      }
    }
    return list;
  }, [initialOrders, orderType, searchValue]);

  const selectedCount = selectedIds.size;
  const allSelected = selectedCount > 0 && selectedCount === filteredOrders.length;
  const canBatchAccept = filteredOrders.some(
    (order) => selectedIds.has(order.id) && order.status === "paid"
  );

  const updateQuery = (next: Record<string, string | number | undefined>) => {
    const params = new URLSearchParams(searchParams?.toString());
    Object.entries(next).forEach(([key, value]) => {
      if (value === undefined || value === "") {
        params.delete(key);
      } else {
        params.set(key, String(value));
      }
    });
    router.push(`/merchant/orders?${params.toString()}`);
  };

  const toggleSelectAll = () => {
    if (allSelected) {
      setSelectedIds(new Set());
      return;
    }
    setSelectedIds(new Set(filteredOrders.map((order) => order.id)));
  };

  const toggleSelect = (orderId: number) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(orderId)) {
        next.delete(orderId);
      } else {
        next.add(orderId);
      }
      return next;
    });
  };

  const runAction = async (orderId: number, action: string, payload?: Record<string, unknown>) => {
    try {
      await apiPost(`/merchant/orders/${orderId}/${action}`, payload);
      window.location.reload();
    } catch {
      toast.error("操作失败，请稍后重试");
    }
  };

  const openDetail = async (orderId: number) => {
    setLoadingDetail(true);
    setDetailOrder(null);
    try {
      const data = await apiGet<OrderResponse>(`/merchant/orders/${orderId}`);
      setDetailOrder(data);
    } catch {
      toast.error("加载失败，请稍后重试");
    } finally {
      setLoadingDetail(false);
    }
  };

  const handleSearch = () => {
    updateQuery({ keyword: searchValue, page: 1 });
  };

  const handlePageChange = (nextPage: number) => {
    updateQuery({ page: nextPage });
  };

  const handlePageSize = (nextSize: number) => {
    updateQuery({ page_size: nextSize, page: 1 });
  };

  const hasNext = initialOrders.length === pageSize;

  return (
    <PageShell>
      <PageHeader
        title="订单管理"
        description="管理全渠道订单流转与履约状态"
        actions={
          <>
            <Button variant="outline" size="sm" onClick={() => window.location.reload()}>
              <RefreshCw className="size-4 mr-2" />
              刷新
            </Button>
            <Button variant="outline" size="sm" onClick={() => toast.info("导出功能开发中")}>
              <Download className="size-4 mr-2" />
              导出
            </Button>
          </>
        }
      />

      <PageContent className="space-y-6">
            <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
            <Card
              className="cursor-pointer border-l-4 border-l-amber-400"
              onClick={() => updateQuery({ status: "paid", page: 1 })}
            >
              <CardHeader className="pb-2">
                <CardDescription>待接单</CardDescription>
                <CardTitle className="text-2xl">{statusCounts.paid}</CardTitle>
              </CardHeader>
              <CardContent className="text-xs text-muted-foreground">
                {statusCounts.paid > 0 ? "需立即处理" : "-"}
              </CardContent>
            </Card>
            <Card
              className="cursor-pointer border-l-4 border-l-blue-400"
              onClick={() => updateQuery({ status: "preparing", page: 1 })}
            >
              <CardHeader className="pb-2">
                <CardDescription>制作中</CardDescription>
                <CardTitle className="text-2xl">{statusCounts.preparing}</CardTitle>
              </CardHeader>
            </Card>
            <Card
              className="cursor-pointer border-l-4 border-l-emerald-400"
              onClick={() => updateQuery({ status: "ready", page: 1 })}
            >
              <CardHeader className="pb-2">
                <CardDescription>待出餐</CardDescription>
                <CardTitle className="text-2xl">{statusCounts.ready}</CardTitle>
              </CardHeader>
            </Card>
            <Card
              className="cursor-pointer border-l-4 border-l-slate-400"
              onClick={() => updateQuery({ status: "completed", page: 1 })}
            >
              <CardHeader className="pb-2">
                <CardDescription>今日完成</CardDescription>
                <CardTitle className="text-2xl">{statusCounts.completed}</CardTitle>
              </CardHeader>
            </Card>
            <Card className="border-l-4 border-l-primary">
              <CardHeader className="pb-2">
                <CardDescription>今日营业额</CardDescription>
                <CardTitle className="text-2xl">¥{formatAmount(todayRevenue)}</CardTitle>
              </CardHeader>
              <CardContent className="text-xs text-muted-foreground">
                今日订单 {todayOrders}
              </CardContent>
            </Card>
          </section>

          <section className="flex flex-wrap items-center justify-between gap-4 rounded-xl border bg-white p-4 shadow-sm">
            <div className="flex flex-wrap items-center gap-2">
              {STATUS_TABS.map((tab) => {
                const active = status === tab.value;
                return (
                  <Button
                    key={tab.value || "all"}
                    variant={active ? "default" : "secondary"}
                    size="sm"
                    onClick={() => updateQuery({ status: tab.value, page: 1 })}
                    className="h-8 rounded-full px-4"
                  >
                    {tab.label}
                    {tab.value === "paid" && statusCounts.paid > 0 && (
                      <Badge variant="secondary" className="ml-1.5 h-4 min-w-4 bg-white/20 px-1 text-[10px] text-white">
                        {statusCounts.paid}
                      </Badge>
                    )}
                  </Button>
                );
              })}
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Select
                value={orderType}
                onValueChange={(value) => updateQuery({ order_type: value, page: 1 })}
              >
                <SelectTrigger className="h-8 w-[120px] rounded-full">
                  <SelectValue placeholder="所有类型" />
                </SelectTrigger>
                <SelectContent>
                  {TYPE_OPTIONS.map((option) => (
                    <SelectItem key={option.value} value={option.value || "all"}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              
              <div className="relative flex items-center">
                <Search className="absolute left-3 size-4 text-muted-foreground" />
                <input
                  className="h-8 w-48 rounded-lg border border-slate-200 bg-slate-50 pl-9 pr-3 text-sm focus:outline-none focus:ring-2 focus:ring-primary/20"
                  placeholder="搜索订单/备注"
                  value={searchValue}
                  onChange={(e) => setSearchValue(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && handleSearch()}
                />
              </div>
              <Button size="sm" variant="ghost" className="h-8 w-8 rounded-full p-0" onClick={handleSearch}>
                <Filter className="size-4" />
              </Button>
            </div>
          </section>

          <Card>
            <CardHeader>
              <CardTitle>订单列表</CardTitle>
              <CardDescription>对应 /v1/merchant/orders</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="rounded-md border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-12">
                        <Checkbox
                          checked={allSelected}
                          onCheckedChange={toggleSelectAll}
                        />
                      </TableHead>
                      <TableHead>订单号</TableHead>
                      <TableHead>类型</TableHead>
                      <TableHead>商品</TableHead>
                      <TableHead>金额</TableHead>
                      <TableHead>状态</TableHead>
                      <TableHead>下单时间</TableHead>
                      <TableHead>操作</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {filteredOrders.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={8} className="text-center text-sm">
                          暂无订单数据
                        </TableCell>
                      </TableRow>
                    ) : (
                      filteredOrders.map((order) => {
                        const { summary, count } = buildItemsSummary(order);
                        const createdAt = new Date(order.created_at);
                        const createdDate = `${createdAt.getMonth() + 1}-${createdAt
                          .getDate()
                          .toString()
                          .padStart(2, "0")}`;
                        const createdTime = createdAt.toTimeString().slice(0, 8);
                        return (
                          <TableRow key={order.id}>
                            <TableCell>
                              <Checkbox
                                checked={selectedIds.has(order.id)}
                                onCheckedChange={() => toggleSelect(order.id)}
                              />
                            </TableCell>
                            <TableCell>
                              <div className="font-medium">#{order.order_no}</div>
                              {order.notes ? (
                                <div className="text-xs text-muted-foreground">
                                  {order.notes}
                                </div>
                              ) : null}
                            </TableCell>
                            <TableCell>
                              <span className={`rounded-full px-2 py-1 text-xs bg-muted`}>
                                {TYPE_LABELS[order.order_type] || order.order_type}
                              </span>
                            </TableCell>
                            <TableCell>
                              <div className="text-sm">{summary}</div>
                              <div className="text-xs text-muted-foreground">
                                共{count}件
                              </div>
                            </TableCell>
                            <TableCell>
                              <div className="font-semibold">
                                ¥{formatAmount(order.total_amount)}
                              </div>
                              {order.discount_amount > 0 ? (
                                <div className="text-xs text-rose-500">
                                  -¥{formatAmount(order.discount_amount)}
                                </div>
                              ) : null}
                            </TableCell>
                            <TableCell>
                              <Badge variant="outline">
                                {STATUS_LABELS[order.status] || order.status}
                              </Badge>
                            </TableCell>
                            <TableCell>
                              <div className="text-sm">{createdDate}</div>
                              <div className="text-xs text-muted-foreground">
                                {createdTime}
                              </div>
                            </TableCell>
                            <TableCell>
                              <div className="flex flex-wrap gap-2">
                                {order.status === "paid" ? (
                                  <>
                                    <Button
                                      size="sm"
                                      onClick={() => runAction(order.id, "accept")}
                                    >
                                      接单
                                    </Button>
                                    <Button
                                      size="sm"
                                      variant="destructive"
                                      onClick={() =>
                                        runAction(order.id, "reject", { reason: "商户拒单" })
                                      }
                                    >
                                      拒单
                                    </Button>
                                  </>
                                ) : null}
                                {order.status === "preparing" ? (
                                  <Button
                                    size="sm"
                                    variant="outline"
                                    onClick={() => runAction(order.id, "ready")}
                                  >
                                    出餐
                                  </Button>
                                ) : null}
                                {order.status === "ready" || order.status === "delivering" ? (
                                  <Button
                                    size="sm"
                                    variant="outline"
                                    onClick={() => runAction(order.id, "complete")}
                                  >
                                    完成
                                  </Button>
                                ) : null}
                                <Button size="sm" variant="outline" onClick={() => openDetail(order.id)}>
                                  详情
                                </Button>
                                <Button
                                  size="sm"
                                  variant="ghost"
                                  onClick={() => toast.info("打印功能开发中")}
                                >
                                  打印
                                </Button>
                              </div>
                            </TableCell>
                          </TableRow>
                        );
                      })
                    )}
                  </TableBody>
                </Table>
              </div>

              <div className="mt-4 flex flex-wrap items-center justify-between gap-3 text-sm">
                <div>共 {filteredOrders.length} 条记录</div>
                <div className="flex items-center gap-2">
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={page <= 1}
                    onClick={() => handlePageChange(page - 1)}
                  >
                    上一页
                  </Button>
                  <span>{page}</span>
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={!hasNext}
                    onClick={() => handlePageChange(page + 1)}
                  >
                    下一页
                  </Button>
                </div>
                <div className="flex items-center gap-2">
                  <span>每页</span>
                  <select
                    className="h-8 rounded-md border border-input bg-background px-2"
                    value={pageSize}
                    onChange={(event) => handlePageSize(Number(event.target.value))}
                  >
                    {[20, 50, 100].map((size) => (
                      <option key={size} value={size}>
                        {size}
                      </option>
                    ))}
                  </select>
                  <span>条</span>
                </div>
              </div>
            </CardContent>
          </Card>

            {selectedCount > 0 ? (
              <div className="flex flex-wrap items-center gap-3 rounded-lg border bg-card p-4">
                <span>已选择 {selectedCount} 项</span>
                {canBatchAccept ? (
                  <Button size="sm" onClick={() => toast.info("批量接单开发中")}>
                    批量接单
                  </Button>
                ) : null}
                <Button size="sm" variant="outline" onClick={() => toast.info("批量打印开发中")}>
                  批量打印
                </Button>
                <Button size="sm" variant="ghost" onClick={() => setSelectedIds(new Set())}>
                  取消选择
                </Button>
              </div>
            ) : null}
      </PageContent>

      <Sheet open={!!detailOrder} onOpenChange={(open) => !open && setDetailOrder(null)}>
        <SheetContent className="flex flex-col sm:max-w-md md:max-w-lg">
          <SheetHeader className="border-b pb-4">
            <SheetTitle>订单详情</SheetTitle>
            <SheetDescription>对应接口 /v1/merchant/orders/{detailOrder?.id}</SheetDescription>
          </SheetHeader>
          
          <div className="flex-1 overflow-y-auto py-6 space-y-8">
            {loadingDetail ? (
              <div className="flex h-40 flex-col items-center justify-center gap-2 text-muted-foreground">
                <RefreshCw className="size-6 animate-spin" />
                <span>正在加载数据...</span>
              </div>
            ) : detailOrder ? (
              <>
                <section>
                  <h3 className="mb-3 font-semibold">基本信息</h3>
                  <div className="rounded-lg border bg-muted/30 p-4 space-y-3">
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">订单号</span>
                      <span className="font-medium">#{detailOrder.order_no}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">订单类型</span>
                      <Badge variant="secondary">{TYPE_LABELS[detailOrder.order_type]}</Badge>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">状态</span>
                      <Badge>{STATUS_LABELS[detailOrder.status]}</Badge>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">下单时间</span>
                      <span>{detailOrder.created_at}</span>
                    </div>
                    {detailOrder.table_id && (
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">桌台</span>
                        <span>{detailOrder.table_id}</span>
                      </div>
                    )}
                  </div>
                </section>

                <section>
                  <h3 className="mb-3 font-semibold">商品明细</h3>
                  <div className="space-y-4">
                    {detailOrder.items.map((item) => (
                      <div key={item.id} className="flex gap-4">
                        <div className="size-16 shrink-0 rounded-lg bg-muted flex items-center justify-center text-muted-foreground">
                          <Store className="size-8 opacity-20" />
                        </div>
                        <div className="flex flex-1 flex-col justify-between">
                          <div className="flex justify-between gap-2">
                            <span className="font-medium leading-none">{item.name}</span>
                            <span className="text-sm whitespace-nowrap">¥{formatAmount(item.subtotal)}</span>
                          </div>
                          <div className="text-xs text-muted-foreground">
                            {item.customizations?.map(c => `${c.name}:${c.value}`).join("、") || "默认口味"}
                          </div>
                          <div className="text-xs font-medium">x{item.quantity}</div>
                        </div>
                      </div>
                    ))}
                  </div>
                </section>

                <section>
                  <Separator className="my-6" />
                  <div className="space-y-3">
                    <div className="flex justify-between text-sm">
                      <span className="text-muted-foreground">商品小计</span>
                      <span>¥{formatAmount(detailOrder.subtotal)}</span>
                    </div>
                    {detailOrder.delivery_fee > 0 && (
                      <div className="flex justify-between text-sm">
                        <span className="text-muted-foreground">配送费</span>
                        <span>¥{formatAmount(detailOrder.delivery_fee)}</span>
                      </div>
                    )}
                    {detailOrder.discount_amount > 0 && (
                      <div className="flex justify-between text-sm text-destructive">
                        <span>优惠金额</span>
                        <span>-¥{formatAmount(detailOrder.discount_amount)}</span>
                      </div>
                    )}
                    <div className="flex justify-between border-t pt-3 text-lg font-bold">
                      <span>实付金额</span>
                      <span className="text-primary">¥{formatAmount(detailOrder.total_amount)}</span>
                    </div>
                  </div>
                </section>

                {detailOrder.notes && (
                  <section>
                    <h3 className="mb-2 font-semibold">订单备注</h3>
                    <div className="rounded-lg bg-amber-50 p-3 text-sm text-amber-900 dark:bg-amber-900/10 dark:text-amber-400">
                      {detailOrder.notes}
                    </div>
                  </section>
                )}
              </>
            ) : null}
          </div>

          <SheetFooter className="border-t pt-4 sm:flex-col sm:space-x-0 gap-2">
            {detailOrder && (
              <>
                <div className="flex gap-2 w-full">
                  <Button variant="outline" className="flex-1" onClick={() => toast.info("打印功能开发中")}>
                    <Printer className="size-4 mr-2" />
                    打印小票
                  </Button>
                  <Button variant="outline" className="flex-1">
                    <MoreVertical className="size-4 mr-2" />
                    更多
                  </Button>
                </div>
                {detailOrder.status === "paid" && (
                  <div className="flex gap-2 w-full">
                    <Button variant="destructive" className="flex-1" onClick={() => runAction(detailOrder.id, "reject")}>
                      拒单
                    </Button>
                    <Button className="flex-1" onClick={() => runAction(detailOrder.id, "accept")}>
                      接单
                    </Button>
                  </div>
                )}
                {detailOrder.status === "preparing" && (
                  <Button className="w-full" onClick={() => runAction(detailOrder.id, "ready")}>
                    出餐完成
                  </Button>
                )}
              </>
            )}
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </PageShell>
  );
}
