"use client";

import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { apiGet, apiPost } from "@/lib/api";
import type {
  KitchenOrderResponse,
  KitchenOrdersResponse,
  KitchenStats,
} from "@/types/kitchen";

const ORDER_TYPE_MAP: Record<string, string> = {
  takeout: "外卖",
  dine_in: "堂食",
  takeaway: "自取",
};

function formatTime(dateStr?: string) {
  if (!dateStr) return "";
  const d = new Date(dateStr);
  const h = String(d.getHours()).padStart(2, "0");
  const m = String(d.getMinutes()).padStart(2, "0");
  return `${h}:${m}`;
}

type Props = {
  initialData: KitchenOrdersResponse;
};

type KitchenOrderView = KitchenOrderResponse & {
  order_type_text: string;
  created_time: string;
  paid_time: string;
};

type ColumnKey = "newOrders" | "preparingOrders" | "readyOrders";

const COLUMNS: Array<{
  key: ColumnKey;
  title: string;
  color: string;
  status: "new" | "preparing" | "ready";
}> = [
  { key: "newOrders", title: "新订单", color: "bg-rose-500", status: "new" },
  { key: "preparingOrders", title: "制作中", color: "bg-amber-500", status: "preparing" },
  { key: "readyOrders", title: "待取餐", color: "bg-emerald-500", status: "ready" },
];

export function KdsPageClient({ initialData }: Props) {
  const [data, setData] = useState(initialData);
  const connected = true;
  const [loading, setLoading] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [voiceEnabled, setVoiceEnabled] = useState(true);
  const [currentTime, setCurrentTime] = useState("");
  const [detailOrder, setDetailOrder] = useState<KitchenOrderView | null>(null);

  const stats = data.stats || ({} as KitchenStats);

  const processed = useMemo(() => {
    const mapOrder = (order: KitchenOrderResponse): KitchenOrderView => ({
      ...order,
      order_type_text: ORDER_TYPE_MAP[order.order_type] || order.order_type,
      created_time: formatTime(order.created_at),
      paid_time: formatTime(order.paid_at),
    });

    return {
      newOrders: (data.new_orders || []).map(mapOrder),
      preparingOrders: (data.preparing_orders || []).map(mapOrder),
      readyOrders: (data.ready_orders || []).map(mapOrder),
    };
  }, [data]);

  useEffect(() => {
    const timer = setInterval(() => {
      const now = new Date();
      const h = String(now.getHours()).padStart(2, "0");
      const m = String(now.getMinutes()).padStart(2, "0");
      const s = String(now.getSeconds()).padStart(2, "0");
      setCurrentTime(`${h}:${m}:${s}`);
    }, 1000);

    return () => clearInterval(timer);
  }, []);

  useEffect(() => {
    if (!autoRefresh) return;
    const timer = setInterval(() => {
      refresh();
    }, 10000);

    return () => clearInterval(timer);
  }, [autoRefresh]);

  const refresh = async () => {
    setLoading(true);
    try {
      const next = await apiGet<KitchenOrdersResponse>("/kitchen/orders");
      setData(next);
    } catch {
      toast.error("加载失败，请稍后重试");
    } finally {
      setLoading(false);
    }
  };

  const toggleVoice = () => {
    setVoiceEnabled((prev) => !prev);
  };

  const toggleRefresh = () => {
    setAutoRefresh((prev) => !prev);
  };

  const runAction = async (orderId: number, action: "preparing" | "ready") => {
    try {
      await apiPost(`/kitchen/orders/${orderId}/${action}`);
      refresh();
    } catch {
      toast.error("操作失败，请稍后重试");
    }
  };

  return (
    <div className="min-h-screen bg-background text-foreground">
      <header className="page-header">
        <div className="flex items-center gap-4">
          <h1 className="text-xl font-semibold">🍳 厨房显示系统</h1>
          <div
            className={`flex items-center gap-2 rounded-full px-3 py-1 text-sm ${
              connected ? "bg-emerald-500/10 text-emerald-700" : "bg-rose-500/10 text-rose-700"
            }`}
          >
            <span className={`h-2 w-2 rounded-full ${connected ? "bg-emerald-500" : "bg-rose-500"}`} />
            {connected ? "已连接" : "离线"}
          </div>
        </div>
        <div className="flex items-center gap-6">
          <div className="flex items-center gap-2 text-sm">
            <span className="rounded-full bg-muted px-3 py-1 text-muted-foreground">
              待制作 {stats.new_count ?? 0}
            </span>
            <span className="rounded-full bg-muted px-3 py-1 text-muted-foreground">
              制作中 {stats.preparing_count ?? 0}
            </span>
            <span className="rounded-full bg-muted px-3 py-1 text-muted-foreground">
              待取餐 {stats.ready_count ?? 0}
            </span>
            <span className="rounded-full bg-muted px-3 py-1 text-muted-foreground">
              今日完成 {stats.completed_today_count ?? 0}
            </span>
          </div>
          <div className="text-base font-semibold">{currentTime}</div>
          <div className="flex items-center gap-2">
            <Button size="sm" variant={voiceEnabled ? "default" : "outline"} onClick={toggleVoice}>
              🔊
            </Button>
            <Button size="sm" variant={autoRefresh ? "default" : "outline"} onClick={toggleRefresh}>
              🔄
            </Button>
            <Button size="sm" variant="outline" onClick={refresh}>
              刷新
            </Button>
            <Button size="sm" variant="destructive" onClick={() => window.location.href = "/merchant/dashboard"}>
              退出
            </Button>
          </div>
        </div>
      </header>

      {loading ? (
        <div className="page-content flex items-center justify-center text-muted-foreground">
          加载中...
        </div>
      ) : (
        <div className="page-content grid gap-4 lg:grid-cols-3">
          {COLUMNS.map((column) => (
            <div key={column.key} className="panel">
              <div className="flex items-center justify-between border-b px-4 py-3">
                <div className="flex items-center gap-2">
                  <span className={`h-2 w-2 rounded-full ${column.color}`} />
                  <span className="font-semibold">{column.title}</span>
                </div>
                <Badge variant="outline" className="border-border text-foreground">
                  {processed[column.key].length}
                </Badge>
              </div>
              <div className="max-h-[70vh] space-y-4 overflow-y-auto p-4">
                {processed[column.key].length === 0 ? (
                  <div className="text-center text-sm text-muted-foreground">暂无{column.title}</div>
                ) : (
                  processed[column.key].map((order) => (
                    <div
                      key={order.id}
                      className={`rounded-lg border border-border bg-card p-4 ${
                        order.is_urged ? "ring-2 ring-rose-500" : ""
                      }`}
                      onClick={() => setDetailOrder(order)}
                    >
                      <div className="flex items-center justify-between">
                        <div className="text-lg font-semibold">#{order.order_no}</div>
                        <span className="rounded-full bg-muted px-2 py-1 text-xs text-muted-foreground">
                          {order.order_type_text}
                        </span>
                      </div>
                      {order.table_no ? (
                        <div className="mt-2 text-sm text-muted-foreground">
                          桌号 {order.table_no}
                        </div>
                      ) : null}
                      <div className="mt-3 space-y-1 text-sm">
                        {order.items.map((item) => (
                          <div key={item.id} className="flex items-center justify-between">
                            <span>{item.name}</span>
                            <span>×{item.quantity}</span>
                          </div>
                        ))}
                      </div>
                      {order.notes ? (
                        <div className="mt-3 text-xs text-muted-foreground">📝 {order.notes}</div>
                      ) : null}
                      <div className="mt-3 flex items-center justify-between text-xs text-muted-foreground">
                        <span>{order.paid_time || order.created_time}</span>
                        <span>等待 {order.waiting_minutes} 分</span>
                      </div>
                      {column.status === "new" ? (
                        <Button
                          size="sm"
                          className="mt-3 w-full"
                          onClick={(event) => {
                            event.stopPropagation();
                            runAction(order.id, "preparing");
                          }}
                        >
                          开始制作
                        </Button>
                      ) : null}
                      {column.status === "preparing" ? (
                        <Button
                          size="sm"
                          variant="outline"
                          className="mt-3 w-full"
                          onClick={(event) => {
                            event.stopPropagation();
                            runAction(order.id, "ready");
                          }}
                        >
                          ✓ 制作完成
                        </Button>
                      ) : null}
                    </div>
                  ))
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      {detailOrder ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
          <div className="w-full max-w-xl rounded-xl bg-card p-6 text-foreground">
            <div className="flex items-center justify-between">
              <div className="text-lg font-semibold">订单详情</div>
              <button onClick={() => setDetailOrder(null)}>✕</button>
            </div>
            <div className="mt-4 space-y-4 text-sm">
              <div className="grid gap-2">
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">订单号</span>
                  <span>{detailOrder.order_no}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">订单类型</span>
                  <span>{detailOrder.order_type_text}</span>
                </div>
                {detailOrder.table_no ? (
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground">桌号</span>
                    <span>{detailOrder.table_no}</span>
                  </div>
                ) : null}
                {detailOrder.pickup_number ? (
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground">取餐号</span>
                    <span>{detailOrder.pickup_number}</span>
                  </div>
                ) : null}
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">下单时间</span>
                  <span>{detailOrder.created_time}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">等待时间</span>
                  <span>{detailOrder.waiting_minutes} 分钟</span>
                </div>
              </div>

              <div>
                <div className="text-base font-semibold">商品明细</div>
                <div className="mt-3 space-y-2">
                  {detailOrder.items.map((item) => (
                    <div key={item.id} className="space-y-1">
                      <div className="flex items-center justify-between">
                        <span>{item.name}</span>
                        <span>×{item.quantity}</span>
                      </div>
                      {item.customizations?.length ? (
                        <div className="flex flex-wrap gap-2 text-xs text-muted-foreground">
                          {item.customizations.map((custom) => (
                            <span
                              key={`${custom.name}-${custom.value}`}
                              className="rounded-full bg-muted px-2 py-0.5"
                            >
                              {custom.name}:{custom.value}
                            </span>
                          ))}
                        </div>
                      ) : null}
                    </div>
                  ))}
                </div>
              </div>

              {detailOrder.notes ? (
                <div>
                  <div className="text-base font-semibold">订单备注</div>
                  <div className="mt-2 rounded-md bg-muted/40 p-3 text-sm">
                    {detailOrder.notes}
                  </div>
                </div>
              ) : null}
            </div>
          </div>
        </div>
      ) : null}

    </div>
  );
}
