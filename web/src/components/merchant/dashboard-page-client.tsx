"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { 
  AlertCircle,
  Armchair,
  CheckCircle2,
  ChevronRight,
  Filter,
  Home,
  Loader2,
  MoreVertical,
  Plus,
  QrCode,
  RefreshCw,
  Search,
  Users,
  XCircle,
  Clock,
  Calendar,
  Phone,
  Sparkles,
  ArrowRightLeft,
} from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { apiGet, apiPatch, apiPost, formatAmount } from "@/lib/api";
import { useMerchantSession } from "@/components/providers/merchant-session-provider";
import { PageShell, PageHeader, PageContent } from "@/components/merchant/layout/page-shell";
import type { OrderResponse } from "@/types/order";
import { cn } from "@/lib/utils";

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

const TABLE_STATUS_MAP: Record<string, { label: string, variant: "default" | "secondary" | "destructive" | "outline", icon: any, colorClass: string }> = {
  available: { label: "空闲", variant: "secondary", icon: CheckCircle2, colorClass: "bg-emerald-500" },
  occupied: { label: "就餐中", variant: "default", icon: Armchair, colorClass: "bg-primary" },
  reserved: { label: "已预订", variant: "outline", icon: AlertCircle, colorClass: "bg-amber-500" },
  cleaning: { label: "清洁中", variant: "outline", icon: RefreshCw, colorClass: "bg-slate-400" },
  disabled: { label: "停用", variant: "destructive", icon: XCircle, colorClass: "bg-muted-foreground" },
};

const TABLE_TYPE_MAP: Record<string, { label: string, icon: any }> = {
  table: { label: "大厅桌台", icon: Armchair },
  room: { label: "包间", icon: Home },
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
  current_reservation?: {
    id: number;
    contact_name: string;
    contact_phone: string;
    guest_count: number;
    reservation_time: string;
    notes?: string;
  };
  todayReservation?: {
    id: number;
    contact_name: string;
    contact_phone: string;
    reservation_time: string;
    guest_count: number;
  };
  tags?: Array<{ id: number; name: string }>;
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
  const [loadingTables, setLoadingTables] = useState(false);
  const reloadGuardRef = useRef(0);
  const [ordersState, setOrdersState] = useState<DashboardOrder[]>(orders);
  const [statusCountsState, setStatusCountsState] = useState(statusCounts);
  const [tableGroupsState, setTableGroupsState] = useState(tableGroups);
  const [tableStatsState, setTableStatsState] = useState(tableStats);
  const [revenueState, setRevenueState] = useState(revenue);
  const [todayOrdersState, setTodayOrdersState] = useState(todayOrders);

  const loadTables = useCallback(async () => {
    setLoadingTables(true);
    try {
      const response = await apiGet<{ tables: DashboardTable[] }>("/tables");
      const tables = response.tables || [];
      
      // Update stats
      setTableStatsState({
        total: tables.length,
        available: tables.filter((t) => t.status === "available").length,
        occupied: tables.filter((t) => t.status === "occupied").length,
      });

      const resResp = await apiGet<{ reservations: any[] }>("/reservations/merchant/today");
      const dayReservations = resResp.reservations || [];
      const nowTimeStr = new Date().toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' });

      // Group tables
      const grouped = new Map<string, DashboardTable[]>();
      tables.forEach((table) => {
        const type = table.table_type || "table";
        if (!grouped.has(type)) grouped.set(type, []);
        
        // Find nearest upcoming reservation
        const tableDayRes = dayReservations
          .filter(r => r.table_id === table.id)
          .sort((a, b) => a.reservation_time.localeCompare(b.reservation_time));
        
        const nextRes = tableDayRes.find(r => r.reservation_time >= nowTimeStr) || tableDayRes[0];
        
        grouped.get(type)!.push({
          ...table,
          todayReservation: nextRes
        });
      });

      const nextGroups = [] as DashboardTableGroup[];
      if (grouped.has("table")) {
        nextGroups.push({ name: "散台", type: "table", tables: grouped.get("table")! });
      }
      if (grouped.has("room")) {
        nextGroups.push({ name: "包间", type: "room", tables: grouped.get("room")! });
      }
      grouped.forEach((value, key) => {
        if (key !== "table" && key !== "room") {
          nextGroups.push({ name: "其他", type: key, tables: value });
        }
      });
      
      setTableGroupsState(nextGroups);
    } catch (error) {
      console.error("Failed to load tables", error);
    } finally {
      setLoadingTables(false);
    }
  }, []);

  useEffect(() => {
    // Initial client-side fetch to ensure data is visible and up to date
    loadTables();
  }, [loadTables]);

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
      // 注意：商户只能将桌台设为空闲（清台），不能开台
      // 开台操作只能由用户扫码完成
      const updated = await apiPatch<DashboardTable>(
        `/tables/${activeTable.id}/status`,
        { status }
      );
      applyTableSnapshot(updated);
      toast.success(`桌台状态已更新为 ${TABLE_STATUS_MAP[status]?.label || status}`);
    } catch (error: any) {
      toast.error(error.message || "操作失败，请重试");
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

      <PageContent className="grid grid-cols-1 lg:grid-cols-[0.9fr_2.2fr_0.9fr] gap-6">
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
                      className={`flex-1 flex flex-col items-center justify-center py-2 px-1 rounded-xl transition-all border ${
                        active
                          ? "bg-primary/5 border-primary text-primary font-bold shadow-xs"
                          : "border-transparent text-muted-foreground hover:bg-muted"
                      }`}
                    >
                      <span className="text-[10px] opacity-70 mb-0.5">{tab.label}</span>
                      <span className={cn(
                        "text-sm",
                        active ? "text-primary" : "text-slate-900"
                      )}>
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
                <div className="flex items-center gap-3">
                  <div className="text-base font-semibold">桌台状态</div>
                  <div className="flex gap-2">
                    <Badge variant="outline" className="text-[10px] font-normal border-slate-200">
                      空闲 {tableStatsState.available}
                    </Badge>
                    <Badge variant="outline" className="text-[10px] font-normal border-slate-200">
                      就餐 {tableStatsState.occupied}
                    </Badge>
                  </div>
                </div>
                <Button variant="ghost" size="icon" className="h-8 w-8 text-slate-400" onClick={loadTables} disabled={loadingTables}>
                  <RefreshCw className={cn("h-4 w-4", loadingTables && "animate-spin")} />
                </Button>
              </div>
              <div className="flex-1 space-y-6 overflow-y-auto p-4 bg-slate-50/50">
                {loadingTables && tableGroupsState.length === 0 ? (
                  <div className="flex flex-col items-center justify-center py-20 gap-3 text-muted-foreground">
                    <Loader2 className="h-8 w-8 animate-spin" />
                    <p className="text-sm">正在加载桌台...</p>
                  </div>
                ) : tableGroupsState.length === 0 ? (
                  <div className="py-10 text-center text-sm text-muted-foreground flex flex-col items-center gap-3">
                    <div className="h-12 w-12 rounded-full bg-muted flex items-center justify-center">
                      <Armchair className="h-6 w-6 opacity-20" />
                    </div>
                    暂无桌台数据
                  </div>
                ) : (
                  tableGroupsState.map((group) => (
                    <div key={group.type} className="space-y-3">
                      <div className="flex items-center gap-2">
                        <div className="w-1 h-4 bg-primary rounded-full" />
                        <span className="text-xs font-bold text-slate-500 uppercase tracking-wider">
                          {group.name} ({group.tables.length})
                        </span>
                      </div>
                      <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-2 2xl:grid-cols-3 gap-3">
                        {group.tables.map((table) => {
                          const statusInfo = TABLE_STATUS_MAP[table.status] || TABLE_STATUS_MAP.available;
                          const typeInfo = TABLE_TYPE_MAP[table.table_type || "table"] || TABLE_TYPE_MAP.table;
                          const StatusIcon = statusInfo.icon;
                          const TypeIcon = typeInfo.icon;
                          const isOccupied = table.status === 'occupied';

                          return (
                            <div
                              key={table.id}
                              className={cn(
                                "group bg-white rounded-xl border shadow-sm transition-all hover:shadow-md cursor-pointer overflow-hidden relative flex flex-col",
                                table.status === 'disabled' && "opacity-60 grayscale-[0.5]",
                                isOccupied ? "border-primary/30" : "border-slate-200 hover:border-primary/30"
                              )}
                              onClick={() => setActiveTable(table)}
                            >
                              {/* Top status bar */}
                              <div className={cn("absolute top-0 left-0 right-0 h-1", statusInfo.colorClass)} />
                              
                              <div className="p-3 pt-4 flex-1">
                                <div className="flex items-start justify-between mb-2">
                                  <div className="space-y-0.5">
                                    <div className="flex items-center gap-2">
                                      <span className="text-xl font-black text-slate-900 tracking-tighter">
                                        {table.table_no}
                                      </span>
                                      <Badge 
                                        variant={statusInfo.variant} 
                                        className={cn(
                                          "text-[9px] px-1.5 h-4 font-bold border-none",
                                          table.status === 'available' ? "bg-emerald-500 text-white" :
                                          table.status === 'occupied' ? "bg-primary text-white" :
                                          table.status === 'reserved' ? "bg-amber-500 text-white" :
                                          "bg-slate-400 text-white"
                                        )}
                                      >
                                        {statusInfo.label}
                                      </Badge>
                                    </div>
                                    <div className="flex items-center text-[10px] text-slate-400 gap-2">
                                      <span className="flex items-center gap-1">
                                        <TypeIcon className="h-2.5 w-2.5" />
                                        {typeInfo.label}
                                      </span>
                                      <span>•</span>
                                      <span className="flex items-center gap-1">
                                        <Users className="h-2.5 w-2.5" />
                                        {table.capacity}人
                                      </span>
                                    </div>
                                  </div>
                                </div>

                                {table.current_reservation ? (
                                  <div className="mt-2 p-2 bg-amber-50/50 rounded-lg border border-amber-100">
                                    <div className="flex items-center justify-between text-[10px] font-bold text-amber-700">
                                      <span className="flex items-center gap-1">
                                        <AlertCircle className="h-2.5 w-2.5" />
                                        近期预约
                                      </span>
                                      <span>{table.current_reservation.reservation_time}</span>
                                    </div>
                                    <div className="text-[10px] text-amber-600/80 truncate mt-0.5">
                                      {table.current_reservation.contact_name} ({table.current_reservation.guest_count}人)
                                    </div>
                                  </div>
                                ) : table.todayReservation ? (
                                  <div className="mt-2 p-2 bg-slate-50 rounded-lg border border-slate-100 group-hover:border-primary/20 transition-colors">
                                    <div className="flex items-center justify-between text-[10px] font-bold text-slate-500 group-hover:text-primary/70">
                                      <span className="flex items-center gap-1">
                                        <Calendar className="h-2.5 w-2.5" />
                                        今日预约
                                      </span>
                                      <span>{table.todayReservation.reservation_time}</span>
                                    </div>
                                    <div className="flex items-center justify-between mt-0.5">
                                      <div className="text-[10px] text-slate-600 font-bold truncate">
                                        {table.todayReservation.contact_name} · {table.todayReservation.guest_count}人
                                      </div>
                                      <div className="text-[9px] text-slate-400 font-medium">
                                        {table.todayReservation.contact_phone}
                                      </div>
                                    </div>
                                  </div>
                                ) : (
                                  <div className="mt-2 h-[38px] flex items-center justify-center border border-dashed border-slate-200 rounded-lg opacity-40">
                                     <span className="text-[9px] text-slate-400">今日暂无预订</span>
                                  </div>
                                )}
                              </div>

                              {/* Actions Footer */}
                              <div className="p-2 bg-slate-50 border-t flex gap-1.5">
                                {isOccupied && (
                                  <>
                                    <Button 
                                      variant="outline" 
                                      size="sm" 
                                      className="flex-1 h-7 text-[10px] font-bold rounded-md border-primary/20 text-primary hover:bg-primary/5"
                                      onClick={(e) => { e.stopPropagation(); setActiveTable(table); }}
                                    >
                                      清台
                                    </Button>
                                    <Button 
                                      variant="ghost" 
                                      size="sm" 
                                      className="h-7 px-2 text-[10px] font-medium text-amber-600 hover:bg-amber-50 rounded-md"
                                      onClick={(e) => { e.stopPropagation(); window.location.href = `/merchant/dinein?transfer=${table.id}`; }}
                                    >
                                      <ArrowRightLeft className="size-3 mr-0.5" /> 换桌
                                    </Button>
                                  </>
                                )}
                                <Button 
                                  variant="ghost" 
                                  size="sm" 
                                  className={cn(
                                    "h-7 px-2 text-[10px] font-medium rounded-md",
                                    isOccupied 
                                      ? "text-slate-400 hover:bg-slate-100" 
                                      : "flex-1 text-slate-500 hover:bg-slate-100"
                                  )}
                                  onClick={(e) => { 
                                    e.stopPropagation(); 
                                    apiPatch(`/tables/${table.id}/status`, { status: 'available' })
                                      .then(() => {
                                        toast.success(`桌台 ${table.table_no} 已完成清扫`);
                                        loadTables();
                                      })
                                      .catch((err: any) => toast.error(err.message || "操作失败"));
                                  }}
                                >
                                  <Sparkles className="size-3 mr-0.5" /> 清扫
                                </Button>
                              </div>
                            </div>
                          );
                        })}
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
              <div>当前状态：{TABLE_STATUS_MAP[activeTable.status]?.label || activeTable.status}</div>
              {activeTable.capacity ? (
                <div className="mt-1">容量：{activeTable.capacity}人</div>
              ) : null}
              {activeTable.current_reservation_id ? (
                <div className="mt-1">预订号：#{activeTable.current_reservation_id}</div>
              ) : null}
            </div>
            <div className="flex flex-col gap-2 px-4 py-4">
              {activeTable.status === "occupied" ? (
                <Button
                  className="w-full"
                  onClick={() => updateTableStatus("available")}
                  disabled={loadingTableStatus !== null}
                >
                  清台结账
                </Button>
              ) : activeTable.status !== "available" ? (
                <Button
                  className="w-full"
                  variant="secondary"
                  onClick={() => updateTableStatus("available")}
                  disabled={loadingTableStatus !== null}
                >
                  <Sparkles className="size-4 mr-2" /> 完成清扫
                </Button>
              ) : (
                <div className="text-center text-sm text-muted-foreground py-2">
                  桌台空闲，等待客人扫码入座
                </div>
              )}
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
