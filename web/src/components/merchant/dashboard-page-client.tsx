"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { apiPatch, apiPost, formatAmount } from "@/lib/api";

const ORDER_STATUS_LABELS: Record<string, string> = {
  paid: "待接单",
  preparing: "制作中",
  ready: "待取餐",
};

const ORDER_TYPE_LABELS: Record<string, string> = {
  takeout: "外卖",
  dine_in: "堂食",
  takeaway: "自取",
  reservation: "预订",
};

const TABLE_STATUS_LABELS: Record<string, string> = {
  available: "空闲",
  occupied: "就餐中",
  reserved: "已预订",
  cleaning: "清洁中",
  disabled: "停用",
};

const TABLE_STATUS_CLASS: Record<string, string> = {
  available: "from-emerald-50 to-emerald-100 border-emerald-200",
  occupied: "from-amber-50 to-amber-100 border-amber-200",
  reserved: "from-blue-50 to-blue-100 border-blue-200",
  cleaning: "from-slate-50 to-slate-100 border-slate-200",
  disabled: "from-gray-50 to-gray-100 border-gray-200 opacity-60",
};

export type DashboardOrder = {
  id: number;
  order_no: string;
  status: "paid" | "preparing" | "ready" | string;
  status_text: string;
  order_type: string;
  order_type_text: string;
  total_amount: number;
  amount_display: string;
  items_summary: string;
  table_no?: string;
  created_at?: string;
  created_time: string;
};

export type DashboardTable = {
  id: number;
  table_no: string;
  table_type?: string;
  status: string;
  capacity?: number;
  current_reservation_id?: number;
};

export type DashboardTableGroup = {
  name: string;
  type: string;
  tables: DashboardTable[];
};

type Props = {
  merchantName: string;
  isOpen: boolean;
  currentDate: string;
  wsConnected?: boolean;
  revenue: number;
  todayOrders: number;
  orders: DashboardOrder[];
  statusCounts: {
    paid: number;
    preparing: number;
    ready: number;
  };
  tableGroups: DashboardTableGroup[];
  tableStats: {
    total: number;
    available: number;
    occupied: number;
  };
};

const ORDER_TABS = [
  { key: "all", label: "全部" },
  { key: "paid", label: "待接单" },
  { key: "preparing", label: "制作中" },
  { key: "ready", label: "待取餐" },
] as const;

type OrderTab = (typeof ORDER_TABS)[number]["key"];

export function DashboardPageClient({
  merchantName,
  isOpen,
  currentDate,
  wsConnected,
  revenue,
  todayOrders,
  orders,
  statusCounts,
  tableGroups,
  tableStats,
}: Props) {
  const [orderTab, setOrderTab] = useState<OrderTab>("all");
  const [activeTable, setActiveTable] = useState<DashboardTable | null>(null);
  const [loadingStatus, setLoadingStatus] = useState(false);
  const [loadingTableStatus, setLoadingTableStatus] = useState<string | null>(null);

  const filteredOrders = useMemo(() => {
    if (orderTab === "all") return orders;
    return orders.filter((order) => order.status === orderTab);
  }, [orders, orderTab]);

  const toggleStatus = async () => {
    setLoadingStatus(true);
    try {
      await apiPatch("/merchants/me/status", { is_open: !isOpen });

      window.location.reload();
    } catch {
      window.alert("更新营业状态失败，请稍后重试");
    } finally {
      setLoadingStatus(false);
    }
  };

  const updateTableStatus = async (status: string) => {
    if (!activeTable) return;
    setLoadingTableStatus(status);
    try {
      await apiPatch(`/tables/${activeTable.id}/status`, { status });

      window.location.reload();
    } catch {
      window.alert("更新桌台状态失败，请稍后重试");
    } finally {
      setLoadingTableStatus(null);
      setActiveTable(null);
    }
  };

  return (
    <div className="min-h-screen bg-background">
      <div className="block min-[1600px]:hidden">
        <div className="flex min-h-screen items-center justify-center bg-[linear-gradient(135deg,#667eea_0%,#764ba2_100%)] px-6">
          <div className="w-full max-w-md rounded-2xl bg-white p-10 text-center shadow-2xl">
            <div className="text-5xl">🏪</div>
            <div className="mt-4 text-lg font-semibold text-slate-900">
              请使用电脑端访问工作台
            </div>
            <div className="mt-2 text-sm text-slate-500">
              工作台为大屏优化设计，建议使用 1600px 以上显示器
            </div>
            <Link
              href="/merchant/orders"
              className="mt-6 inline-flex items-center justify-center rounded-lg bg-[linear-gradient(135deg,#667eea_0%,#764ba2_100%)] px-6 py-3 text-sm font-medium text-white"
            >
              进入订单管理
            </Link>
          </div>
        </div>
      </div>

      <div className="hidden min-[1600px]:block">
        <div className="flex min-h-screen flex-col">
          <header className="flex h-16 items-center justify-between bg-[linear-gradient(135deg,#667eea_0%,#764ba2_100%)] px-8 text-white shadow-lg">
            <div className="flex items-center gap-4">
              <div className="text-lg font-semibold">
                {merchantName || "商户工作台"}
              </div>
              <button
                className={`flex items-center gap-2 rounded-full px-3 py-1 text-xs transition-colors ${
                  isOpen
                    ? "bg-emerald-500/20"
                    : "bg-rose-500/20"
                }`}
                onClick={toggleStatus}
                disabled={loadingStatus}
              >
                <span
                  className={`h-2 w-2 rounded-full ${
                    isOpen ? "bg-emerald-400" : "bg-rose-400"
                  }`}
                />
                {isOpen ? "营业中" : "已打烊"}
              </button>
              {wsConnected ? (
                <div className="flex items-center gap-2 rounded-full bg-emerald-500/20 px-3 py-1 text-xs">
                  <span className="h-2 w-2 animate-pulse rounded-full bg-emerald-300" />
                  实时
                </div>
              ) : null}
            </div>
            <div className="text-sm opacity-90">{currentDate}</div>
            <div />
          </header>

          <main className="grid flex-1 grid-cols-[1fr_2fr_1fr] gap-5 bg-muted/50 p-5">
            <section className="flex flex-col overflow-hidden rounded-xl bg-card shadow-sm">
              <div className="flex items-center justify-between border-b px-5 py-4">
                <div className="text-base font-semibold">📋 订单流</div>
                <Link href="/merchant/orders" className="text-sm text-primary">
                  查看全部 →
                </Link>
              </div>
              <div className="flex gap-2 border-b bg-muted/30 px-4 py-3">
                {ORDER_TABS.map((tab) => {
                  const active = orderTab === tab.key;
                  const count =
                    tab.key === "paid"
                      ? statusCounts.paid
                      : tab.key === "preparing"
                      ? statusCounts.preparing
                      : tab.key === "ready"
                      ? statusCounts.ready
                      : orders.length;
                  return (
                    <button
                      key={tab.key}
                      onClick={() => setOrderTab(tab.key)}
                      className={`flex items-center gap-2 rounded-full px-3 py-1 text-xs transition-colors ${
                        active
                          ? "bg-[linear-gradient(135deg,#667eea_0%,#764ba2_100%)] text-white"
                          : "text-muted-foreground hover:bg-muted"
                      }`}
                    >
                      {tab.label}
                      <span
                        className={`rounded-full px-2 text-[10px] ${
                          active ? "bg-white/30" : "bg-black/10"
                        }`}
                      >
                        {count}
                      </span>
                    </button>
                  );
                })}
              </div>
              <div className="flex-1 space-y-3 overflow-y-auto p-4">
                {filteredOrders.length === 0 ? (
                  <div className="py-10 text-center text-sm text-muted-foreground">
                    暂无订单
                  </div>
                ) : (
                  filteredOrders.map((order) => (
                    <div
                      key={order.id}
                      className={`rounded-lg border-l-4 bg-muted/40 p-4 shadow-sm transition hover:translate-x-1 hover:shadow-md ${
                        order.status === "preparing"
                          ? "border-amber-400"
                          : order.status === "ready"
                          ? "border-emerald-400"
                          : "border-rose-400"
                      }`}
                    >
                      <div className="flex items-center gap-2 text-sm font-semibold">
                        <span>#{order.order_no}</span>
                        <span className="rounded bg-primary/10 px-2 py-0.5 text-xs text-primary">
                          {ORDER_TYPE_LABELS[order.order_type] || order.order_type_text}
                        </span>
                        <span
                          className={`ml-auto rounded px-2 py-0.5 text-xs ${
                            order.status === "preparing"
                              ? "bg-amber-50 text-amber-600"
                              : order.status === "ready"
                              ? "bg-emerald-50 text-emerald-600"
                              : "bg-rose-50 text-rose-600"
                          }`}
                        >
                          {ORDER_STATUS_LABELS[order.status] || order.status_text}
                        </span>
                      </div>
                      <div className="mt-2 truncate text-xs text-muted-foreground">
                        {order.items_summary}
                      </div>
                      <div className="mt-2 flex items-center justify-between text-sm">
                        <span className="font-semibold text-rose-500">
                          ¥{order.amount_display}
                        </span>
                        <span className="text-xs text-muted-foreground">
                          {order.created_time}
                        </span>
                      </div>
                      <div className="mt-3 flex gap-2">
                        {order.status === "paid" ? (
                          <>
                            <Button
                              size="sm"
                              className="flex-1"
                              onClick={async () => {
                                await apiPost(`/merchant/orders/${order.id}/accept`);
                                window.location.reload();
                              }}
                            >
                              接单
                            </Button>
                            <Button
                              size="sm"
                              variant="outline"
                              className="flex-1"
                              onClick={async () => {
                                const reason = window.prompt("拒单原因");
                                if (!reason) return;
                                await apiPost(`/merchant/orders/${order.id}/reject`, {
                                  reason,
                                });
                                window.location.reload();
                              }}
                            >
                              拒绝
                            </Button>
                          </>
                        ) : null}
                        {order.status === "preparing" ? (
                          <Button
                            size="sm"
                            className="flex-1"
                            onClick={async () => {
                              await apiPost(`/merchant/orders/${order.id}/ready`);
                              window.location.reload();
                            }}
                          >
                            已出餐
                          </Button>
                        ) : null}
                      </div>
                    </div>
                  ))
                )}
              </div>
            </section>

            <section className="flex flex-col overflow-hidden rounded-xl bg-card shadow-sm">
              <div className="flex items-center justify-between border-b px-5 py-4">
                <div className="text-base font-semibold">🪑 桌台状态</div>
                <div className="flex gap-4 text-xs text-muted-foreground">
                  <span>空闲 {tableStats.available}</span>
                  <span>就餐 {tableStats.occupied}</span>
                  <span>共 {tableStats.total}</span>
                </div>
              </div>
              <div className="flex-1 space-y-6 overflow-y-auto p-4">
                {tableGroups.length === 0 ? (
                  <div className="py-10 text-center text-sm text-muted-foreground">
                    暂无桌台数据
                  </div>
                ) : (
                  tableGroups.map((group) => (
                    <div key={group.type}>
                      <div className="mb-3 border-l-4 border-primary pl-3 text-sm font-medium text-slate-700">
                        {group.name}
                      </div>
                      <div className="grid grid-cols-[repeat(auto-fill,minmax(110px,1fr))] gap-3">
                        {group.tables.map((table) => (
                          <button
                            key={table.id}
                            className={`rounded-xl border bg-linear-to-br p-3 text-center text-xs transition hover:-translate-y-0.5 hover:shadow-lg ${
                              TABLE_STATUS_CLASS[table.status] || "from-slate-50 to-slate-100"
                            }`}
                            onClick={() => setActiveTable(table)}
                          >
                            <div className="text-base font-bold text-slate-900">
                              {table.table_no}
                            </div>
                            <div className="text-[11px] text-slate-500">
                              {TABLE_STATUS_LABELS[table.status] || table.status}
                            </div>
                            {table.capacity ? (
                              <div className="text-[11px] text-slate-400">
                                {table.capacity}人座
                              </div>
                            ) : null}
                          </button>
                        ))}
                      </div>
                    </div>
                  ))
                )}
              </div>
            </section>

            <section className="flex flex-col overflow-hidden rounded-xl bg-card shadow-sm">
              <div className="bg-[linear-gradient(135deg,#667eea_0%,#764ba2_100%)] p-6 text-white">
                <div className="text-sm opacity-90">今日营业</div>
                <div className="mt-2 text-xs opacity-80">营业额</div>
                <div className="mt-1 text-3xl font-bold">¥{formatAmount(revenue)}</div>
                <div className="mt-3 text-xs opacity-80">订单 {todayOrders} 单</div>
              </div>
              <div className="grid gap-4 p-5 text-sm">
                <div className="rounded-lg border bg-muted/30 p-3">
                  <div className="text-xs text-muted-foreground">待接单</div>
                  <div className="mt-1 text-lg font-semibold text-rose-500">
                    {statusCounts.paid}
                  </div>
                </div>
                <div className="rounded-lg border bg-muted/30 p-3">
                  <div className="text-xs text-muted-foreground">制作中</div>
                  <div className="mt-1 text-lg font-semibold text-amber-500">
                    {statusCounts.preparing}
                  </div>
                </div>
                <div className="rounded-lg border bg-muted/30 p-3">
                  <div className="text-xs text-muted-foreground">待取餐</div>
                  <div className="mt-1 text-lg font-semibold text-emerald-500">
                    {statusCounts.ready}
                  </div>
                </div>
                <div className="rounded-lg border bg-muted/30 p-3">
                  <div className="text-xs text-muted-foreground">空闲桌台</div>
                  <div className="mt-1 text-lg font-semibold text-slate-700">
                    {tableStats.available}
                  </div>
                </div>
                <div className="rounded-lg border bg-muted/30 p-3">
                  <div className="text-xs text-muted-foreground">就餐桌台</div>
                  <div className="mt-1 text-lg font-semibold text-slate-700">
                    {tableStats.occupied}
                  </div>
                </div>
                <div className="rounded-lg border bg-muted/30 p-3">
                  <div className="text-xs text-muted-foreground">桌台总数</div>
                  <div className="mt-1 text-lg font-semibold text-slate-700">
                    {tableStats.total}
                  </div>
                </div>
              </div>
            </section>
          </main>
        </div>
      </div>

      {activeTable ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="w-80 overflow-hidden rounded-xl bg-white shadow-2xl">
            <div className="flex items-center justify-between bg-[linear-gradient(135deg,#667eea_0%,#764ba2_100%)] px-5 py-4 text-white">
              <div className="text-lg font-semibold">{activeTable.table_no}</div>
              <button className="text-xl" onClick={() => setActiveTable(null)}>
                ×
              </button>
            </div>
            <div className="border-b px-5 py-4 text-sm text-slate-600">
              <div>当前状态：{TABLE_STATUS_LABELS[activeTable.status] || activeTable.status}</div>
              {activeTable.capacity ? (
                <div className="mt-1">容量：{activeTable.capacity}人</div>
              ) : null}
              {activeTable.current_reservation_id ? (
                <div className="mt-1">预订号：#{activeTable.current_reservation_id}</div>
              ) : null}
            </div>
            <div className="flex flex-col gap-2 px-4 py-4">
              {activeTable.status !== "available" ? (
                <Button
                  className="w-full"
                  onClick={() => updateTableStatus("available")}
                  disabled={loadingTableStatus !== null}
                >
                  设为空闲（离场）
                </Button>
              ) : null}
              {activeTable.status !== "occupied" ? (
                <Button
                  className="w-full"
                  variant="outline"
                  onClick={() => updateTableStatus("occupied")}
                  disabled={loadingTableStatus !== null}
                >
                  设为就餐中
                </Button>
              ) : null}
              <Button asChild variant="outline" className="w-full">
                <Link
                  href={`/merchant/reservations?tableId=${activeTable.id}&openAdd=true`}
                >
                  添加预订
                </Link>
              </Button>
            </div>
            {loadingTableStatus ? (
              <div className="pb-4 text-center text-xs text-muted-foreground">
                正在更新...
              </div>
            ) : null}
          </div>
        </div>
      ) : null}
    </div>
  );
}
