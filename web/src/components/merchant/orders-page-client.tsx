"use client";

import { useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
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
      window.alert("操作失败，请稍后重试");
    }
  };

  const openDetail = async (orderId: number) => {
    setLoadingDetail(true);
    setDetailOrder(null);
    try {
      const data = await apiGet<OrderResponse>(`/merchant/orders/${orderId}`);
      setDetailOrder(data);
    } catch {
      window.alert("加载失败，请稍后重试");
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
    <div className="space-y-6">
      <header className="page-header">
        <div>
          <h1 className="text-xl font-semibold">订单管理</h1>
          <p className="text-sm text-muted-foreground">
            管理所有订单的接单、制作和完成流程
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => window.location.reload()}>
            刷新
          </Button>
          <Button variant="outline" onClick={() => window.alert("导出功能开发中")}>
            导出
          </Button>
        </div>
      </header>

      <div className="page-content">
        <div>
          <div className="space-y-6">
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

          <section className="rounded-lg border bg-card p-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="flex flex-wrap items-center gap-2">
                {STATUS_TABS.map((tab) => {
                  const active = status === tab.value;
                  return (
                    <button
                      key={tab.value || "all"}
                      onClick={() => updateQuery({ status: tab.value, page: 1 })}
                      className={`rounded-md px-3 py-1.5 text-sm transition-colors ${
                        active
                          ? "bg-primary text-primary-foreground"
                          : "bg-muted text-muted-foreground hover:bg-muted/70"
                      }`}
                    >
                      {tab.label}
                      {tab.value === "paid" && statusCounts.paid > 0 ? (
                        <span className="ml-1 rounded-full bg-white/20 px-2 text-xs">
                          {statusCounts.paid}
                        </span>
                      ) : null}
                    </button>
                  );
                })}
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <select
                  className="h-9 rounded-md border border-input bg-background px-3 text-sm"
                  value={orderType}
                  onChange={(event) =>
                    updateQuery({ order_type: event.target.value, page: 1 })
                  }
                >
                  {TYPE_OPTIONS.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
                <div className="flex items-center gap-2 rounded-md border border-input bg-background px-3">
                  <span className="text-sm">🔍</span>
                  <input
                    className="h-9 w-52 bg-transparent text-sm outline-none"
                    placeholder="搜索订单号/备注"
                    value={searchValue}
                    onChange={(event) => setSearchValue(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter") handleSearch();
                    }}
                  />
                </div>
                <Button size="sm" variant="outline" onClick={handleSearch}>
                  搜索
                </Button>
              </div>
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
                      <TableHead className="w-10">
                        <button
                          className={`h-4 w-4 rounded border ${
                            allSelected ? "bg-primary" : "bg-white"
                          }`}
                          onClick={toggleSelectAll}
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
                              <button
                                className={`h-4 w-4 rounded border ${
                                  selectedIds.has(order.id)
                                    ? "bg-primary"
                                    : "bg-white"
                                }`}
                                onClick={() => toggleSelect(order.id)}
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
                                  onClick={() => window.alert("打印功能开发中")}
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
                  <Button size="sm" onClick={() => window.alert("批量接单开发中")}>
                    批量接单
                  </Button>
                ) : null}
                <Button size="sm" variant="outline" onClick={() => window.alert("批量打印开发中")}>
                  批量打印
                </Button>
                <Button size="sm" variant="ghost" onClick={() => setSelectedIds(new Set())}>
                  取消选择
                </Button>
              </div>
            ) : null}
          </div>
        </div>
      </div>

      <div className={`fixed inset-0 z-40 ${detailOrder ? "" : "pointer-events-none"}`}>
        <div
          className={`absolute inset-0 bg-black/50 transition-opacity ${
            detailOrder ? "opacity-100" : "opacity-0"
          }`}
          onClick={() => setDetailOrder(null)}
        />
        <div
          className={`absolute right-0 top-0 h-full w-105 bg-background p-6 shadow-xl transition-transform ${
            detailOrder ? "translate-x-0" : "translate-x-full"
          }`}
        >
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold">订单详情</h2>
            <button onClick={() => setDetailOrder(null)}>✕</button>
          </div>
          {loadingDetail ? (
            <div className="mt-6 text-sm text-muted-foreground">加载中...</div>
          ) : detailOrder ? (
            <div className="mt-6 space-y-6 text-sm">
              <section className="space-y-3">
                <div className="text-base font-semibold">订单信息</div>
                <div className="grid gap-2">
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground">订单号</span>
                    <span>#{detailOrder.order_no}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground">订单类型</span>
                    <span>{TYPE_LABELS[detailOrder.order_type]}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground">订单状态</span>
                    <Badge variant="outline">
                      {STATUS_LABELS[detailOrder.status]}
                    </Badge>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground">下单时间</span>
                    <span>{detailOrder.created_at}</span>
                  </div>
                  {detailOrder.paid_at ? (
                    <div className="flex items-center justify-between">
                      <span className="text-muted-foreground">支付时间</span>
                      <span>{detailOrder.paid_at}</span>
                    </div>
                  ) : null}
                  {detailOrder.table_id ? (
                    <div className="flex items-center justify-between">
                      <span className="text-muted-foreground">桌台</span>
                      <span>{detailOrder.table_id}</span>
                    </div>
                  ) : null}
                </div>
              </section>

              <section className="space-y-3">
                <div className="text-base font-semibold">商品明细</div>
                <div className="space-y-3">
                  {detailOrder.items.map((item) => (
                    <div key={item.id} className="flex items-center gap-3">
                      <div className="h-12 w-12 rounded-md bg-muted" />
                      <div className="flex-1">
                        <div className="font-medium">{item.name}</div>
                        {item.customizations?.length ? (
                          <div className="text-xs text-muted-foreground">
                            {item.customizations
                              .map((custom) => `${custom.name}:${custom.value}`)
                              .join("、")}
                          </div>
                        ) : null}
                      </div>
                      <div className="text-xs text-muted-foreground">x{item.quantity}</div>
                      <div className="text-sm">¥{formatAmount(item.subtotal)}</div>
                    </div>
                  ))}
                </div>
              </section>

              <section className="space-y-2">
                <div className="text-base font-semibold">金额明细</div>
                <div className="flex items-center justify-between">
                  <span>商品小计</span>
                  <span>¥{formatAmount(detailOrder.subtotal)}</span>
                </div>
                {detailOrder.delivery_fee > 0 ? (
                  <div className="flex items-center justify-between">
                    <span>配送费</span>
                    <span>¥{formatAmount(detailOrder.delivery_fee)}</span>
                  </div>
                ) : null}
                {detailOrder.discount_amount > 0 ? (
                  <div className="flex items-center justify-between text-rose-500">
                    <span>优惠</span>
                    <span>-¥{formatAmount(detailOrder.discount_amount)}</span>
                  </div>
                ) : null}
                <div className="flex items-center justify-between text-base font-semibold">
                  <span>实付金额</span>
                  <span>¥{formatAmount(detailOrder.total_amount)}</span>
                </div>
              </section>

              {detailOrder.notes ? (
                <section className="space-y-2">
                  <div className="text-base font-semibold">订单备注</div>
                  <div className="rounded-md border bg-muted/40 p-2">
                    {detailOrder.notes}
                  </div>
                </section>
              ) : null}
            </div>
          ) : null}

          {detailOrder ? (
            <div className="mt-6 flex flex-wrap gap-2">
              <Button variant="outline" onClick={() => window.alert("打印功能开发中")}>
                打印小票
              </Button>
              {detailOrder.status === "paid" ? (
                <>
                  <Button
                    variant="destructive"
                    onClick={() =>
                      runAction(detailOrder.id, "reject", { reason: "商户拒单" })
                    }
                  >
                    拒单
                  </Button>
                  <Button onClick={() => runAction(detailOrder.id, "accept")}>接单</Button>
                </>
              ) : null}
              {detailOrder.status === "preparing" ? (
                <Button onClick={() => runAction(detailOrder.id, "ready")}>出餐完成</Button>
              ) : null}
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}
