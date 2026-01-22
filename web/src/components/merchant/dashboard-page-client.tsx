"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { apiPatch, apiPost, formatAmount } from "@/lib/api";
import { useMerchantSession } from "@/components/providers/merchant-session-provider";
import { PageShell, PageHeader, PageContent } from "@/components/merchant/layout/page-shell";
import type { OrderResponse } from "@/types/order";

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
  const session = useMerchantSession();
  const [orderTab, setOrderTab] = useState<OrderTab>("all");
  const [activeTable, setActiveTable] = useState<DashboardTable | null>(null);
  const [loadingStatus, setLoadingStatus] = useState(false);
  const [loadingTableStatus, setLoadingTableStatus] = useState<string | null>(null);
  const reloadGuardRef = useRef(0);

  const [ordersState, setOrdersState] = useState<DashboardOrder[]>(orders);
  const [statusCountsState, setStatusCountsState] = useState(statusCounts);
  const [tableGroupsState, setTableGroupsState] = useState(tableGroups);
  const [tableStatsState, setTableStatsState] = useState(tableStats);
  const [revenueState, setRevenueState] = useState(revenue);
  const [todayOrdersState, setTodayOrdersState] = useState(todayOrders);

  const effectiveIsOpen = session?.isReady ? session.isOpen : isOpen;
  const effectiveWsConnected = session?.isReady ? session.wsConnected : wsConnected;

  const filteredOrders = useMemo(() => {
    if (orderTab === "all") return ordersState;
    return ordersState.filter((order) => order.status === orderTab);
  }, [ordersState, orderTab]);

  const mapOrderSnapshot = useCallback(
    (payload?: Partial<OrderResponse>, fallback?: DashboardOrder) => {
      if (!payload?.id || !payload.order_no) return null;
      const createdTime = payload.created_at
        ? new Date(payload.created_at)
        : fallback?.created_at
        ? new Date(fallback.created_at)
        : null;
      const timeText = createdTime
        ? `${String(createdTime.getHours()).padStart(2, "0")}:${String(
            createdTime.getMinutes()
          ).padStart(2, "0")}`
        : fallback?.created_time || "";
      const summary = payload.items?.length
        ? payload.items.slice(0, 2).map((i) => i.name).join("、")
        : fallback?.items_summary;
      const totalAmount =
        typeof payload.total_amount === "number"
          ? payload.total_amount
          : fallback?.total_amount ?? 0;

      return {
        id: payload.id,
        order_no: payload.order_no,
        status: payload.status || fallback?.status || "paid",
        status_text: payload.status || fallback?.status_text || "paid",
        order_type: payload.order_type || fallback?.order_type || "takeout",
        order_type_text:
          payload.order_type || fallback?.order_type_text || "takeout",
        total_amount: totalAmount,
        amount_display: formatAmount(totalAmount),
        items_summary: summary || "订单商品",
        table_no: payload.table_id
          ? String(payload.table_id)
          : fallback?.table_no,
        created_at: payload.created_at || fallback?.created_at,
        created_time: timeText,
      } satisfies DashboardOrder;
    },
    []
  );

  const recomputeCounts = useCallback((nextOrders: DashboardOrder[]) => {
    setStatusCountsState({
      paid: nextOrders.filter((o) => o.status === "paid").length,
      preparing: nextOrders.filter((o) => o.status === "preparing").length,
      ready: nextOrders.filter((o) => o.status === "ready").length,
    });
  }, []);

  const applyOrderSnapshot = useCallback(
    (payload?: Partial<OrderResponse>, options?: { isNew?: boolean }) => {
      if (!payload) return;
      setOrdersState((prev) => {
        const existing = prev.find((order) => order.id === payload.id);
        const mapped = mapOrderSnapshot(payload, existing || undefined);
        if (!mapped) return prev;

        const shouldInclude = ["paid", "preparing", "ready"].includes(
          mapped.status
        );
        let next = prev.filter((order) => order.id !== mapped.id);
        if (shouldInclude) {
          next = [mapped, ...next];
        }

        if (options?.isNew && !existing && shouldInclude) {
          setTodayOrdersState((value) => value + 1);
          setRevenueState((value) => value + mapped.total_amount);
        }

        recomputeCounts(next);
        return next;
      });
    },
    [mapOrderSnapshot, recomputeCounts]
  );

  const applyTableSnapshot = useCallback(
    (payload?: { id?: number; status?: string; table_no?: string }) => {
      if (!payload?.id) return;
      setTableGroupsState((prev) => {
        let updated = false;
        const next = prev.map((group) => ({
          ...group,
          tables: group.tables.map((table) => {
            if (table.id !== payload.id) return table;
            updated = true;
            return {
              ...table,
              status: payload.status ?? table.status,
              table_no: payload.table_no ?? table.table_no,
            };
          }),
        }));

        if (updated) {
          const allTables = next.flatMap((group) => group.tables);
          setTableStatsState({
            total: allTables.length,
            available: allTables.filter((t) => t.status === "available").length,
            occupied: allTables.filter((t) => t.status === "occupied").length,
          });
        }

        return next;
      });
    },
    []
  );

  const toggleStatus = async () => {
    if (session && !session.isAuthenticated) {
      toast.error("未登录，无法切换营业状态");
      return;
    }
    setLoadingStatus(true);
    try {
      if (session?.setOpen) {
        await session.setOpen(!effectiveIsOpen);
      } else {
        await apiPatch("/merchants/me/status", { is_open: !effectiveIsOpen });
      }
    } catch {
      toast.error("更新营业状态失败，请确认已登录");
    } finally {
      setLoadingStatus(false);
    }
  };

  useEffect(() => {
    const handler = (event: Event) => {
      const customEvent = event as CustomEvent;
      const detail = customEvent.detail as { type?: string } | string | undefined;
      const messageType = typeof detail === "string" ? detail : detail?.type;

      if (
        messageType === "new_order" ||
        messageType === "order_update" ||
        messageType === "table_status_change"
      ) {
        const now = Date.now();
        if (now - reloadGuardRef.current < 3000) return;
        reloadGuardRef.current = now;
        if (messageType === "table_status_change") {
          applyTableSnapshot((detail as { data?: Record<string, unknown> })?.data);
        } else {
          applyOrderSnapshot((detail as { data?: Partial<OrderResponse> })?.data, {
            isNew: messageType === "new_order",
          });
        }
      }
    };

    window.addEventListener("merchant-realtime", handler as EventListener);
    return () => {
      window.removeEventListener("merchant-realtime", handler as EventListener);
    };
  }, [applyOrderSnapshot, applyTableSnapshot]);

  const updateTableStatus = async (status: string) => {
    if (!activeTable) return;
    setLoadingTableStatus(status);
    try {
      const updated = await apiPatch<DashboardTable>(
        `/tables/${activeTable.id}/status`,
        { status }
      );
      applyTableSnapshot(updated);
    } catch {
      toast.error("更新桌台状态失败，请稍后重试");
    } finally {
      setLoadingTableStatus(null);
      setActiveTable(null);
    }
  };

  return (
    <PageShell>
      <PageHeader
        title={merchantName || "商户工作台"}
        description={currentDate}
        actions={
          <>
            <button
              className={`flex items-center gap-2 rounded-full px-3 py-1 text-xs transition-colors ${
                effectiveIsOpen
                  ? "bg-emerald-500/15 text-emerald-700"
                  : "bg-rose-500/15 text-rose-700"
              }`}
              onClick={toggleStatus}
              disabled={loadingStatus || (session ? !session.isAuthenticated : false)}
            >
              <span
                className={`h-2 w-2 rounded-full ${
                  effectiveIsOpen ? "bg-emerald-500" : "bg-rose-500"
                }`}
              />
              {effectiveIsOpen ? "营业中" : "已打烊"}
            </button>
            {effectiveWsConnected ? (
              <div className="flex items-center gap-2 rounded-full bg-emerald-500/15 px-3 py-1 text-xs text-emerald-700">
                <span className="h-2 w-2 animate-pulse rounded-full bg-emerald-500" />
                实时
              </div>
            ) : null}
          </>
        }
      />

      <PageContent className="grid grid-cols-1 lg:grid-cols-[1.1fr_1.8fr_1.1fr] gap-6">
            <section className="flex flex-col overflow-hidden rounded-xl bg-white border shadow-sm">
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
                      ? statusCountsState.paid
                      : tab.key === "preparing"
                      ? statusCountsState.preparing
                      : tab.key === "ready"
                      ? statusCountsState.ready
                      : ordersState.length;
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
                                const updated = await apiPost<OrderResponse>(
                                  `/merchant/orders/${order.id}/accept`
                                );
                                applyOrderSnapshot(updated);
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
                                const updated = await apiPost<OrderResponse>(
                                  `/merchant/orders/${order.id}/reject`,
                                  { reason }
                                );
                                applyOrderSnapshot(updated);
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
                              const updated = await apiPost<OrderResponse>(
                                `/merchant/orders/${order.id}/ready`
                              );
                              applyOrderSnapshot(updated);
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

            <section className="flex flex-col overflow-hidden rounded-xl bg-white border shadow-sm">
              <div className="flex items-center justify-between border-b px-5 py-4">
                <div className="text-base font-semibold">🪑 桌台状态</div>
                <div className="flex gap-4 text-xs text-muted-foreground">
                  <span>空闲 {tableStatsState.available}</span>
                  <span>就餐 {tableStatsState.occupied}</span>
                  <span>共 {tableStatsState.total}</span>
                </div>
              </div>
              <div className="flex-1 space-y-6 overflow-y-auto p-4">
                {tableGroupsState.length === 0 ? (
                  <div className="py-10 text-center text-sm text-muted-foreground">
                    暂无桌台数据
                  </div>
                ) : (
                  tableGroupsState.map((group) => (
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

            <section className="flex flex-col overflow-hidden rounded-xl bg-white border shadow-sm">
              <div className="bg-[linear-gradient(135deg,#667eea_0%,#764ba2_100%)] p-6 text-white">
                <div className="text-sm opacity-90">今日营业</div>
                <div className="mt-2 text-xs opacity-80">营业额</div>
                <div className="mt-1 text-3xl font-bold">¥{formatAmount(revenueState)}</div>
                <div className="mt-3 text-xs opacity-80">订单 {todayOrdersState} 单</div>
              </div>
              <div className="grid gap-4 p-5 text-sm">
                <div className="rounded-lg border bg-muted/30 p-3">
                  <div className="text-xs text-muted-foreground">待接单</div>
                  <div className="mt-1 text-lg font-semibold text-rose-500">
                    {statusCountsState.paid}
                  </div>
                </div>
                <div className="rounded-lg border bg-muted/30 p-3">
                  <div className="text-xs text-muted-foreground">制作中</div>
                  <div className="mt-1 text-lg font-semibold text-amber-500">
                    {statusCountsState.preparing}
                  </div>
                </div>
                <div className="rounded-lg border bg-muted/30 p-3">
                  <div className="text-xs text-muted-foreground">待取餐</div>
                  <div className="mt-1 text-lg font-semibold text-emerald-500">
                    {statusCountsState.ready}
                  </div>
                </div>
                <div className="rounded-lg border bg-muted/30 p-3">
                  <div className="text-xs text-muted-foreground">空闲桌台</div>
                  <div className="mt-1 text-lg font-semibold text-slate-700">
                    {tableStatsState.available}
                  </div>
                </div>
                <div className="rounded-lg border bg-muted/30 p-3">
                  <div className="text-xs text-muted-foreground">就餐桌台</div>
                  <div className="mt-1 text-lg font-semibold text-slate-700">
                    {tableStatsState.occupied}
                  </div>
                </div>
                <div className="rounded-lg border bg-muted/30 p-3">
                  <div className="text-xs text-muted-foreground">桌台总数</div>
                  <div className="mt-1 text-lg font-semibold text-slate-700">
                    {tableStatsState.total}
                  </div>
                </div>
              </div>
            </section>
        </PageContent>

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
    </PageShell>
  );
}
