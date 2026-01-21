import { OrdersPageClient } from "@/components/merchant/orders-page-client";
import { apiGet, formatDate } from "@/lib/api";
import type { OrderResponse, OrderStatsResponse } from "@/types/order";

const fallbackOrders: OrderResponse[] = [];
const fallbackStats: OrderStatsResponse = {
  total_orders: 0,
  total_revenue: 0,
  avg_order_value: 0,
  completed_orders: 0,
  cancelled_orders: 0,
  completion_rate: 0,
};

function normalizeOrders(data: OrderResponse[] | { items?: OrderResponse[] } | undefined) {
  if (!data) return fallbackOrders;
  if (Array.isArray(data)) return data;
  if (data.items && Array.isArray(data.items)) return data.items;
  return fallbackOrders;
}

export default async function OrdersPage({
  searchParams,
}: {
  searchParams?: Record<string, string | string[] | undefined>;
}) {
  const status = (searchParams?.status as string) || "";
  const orderType = (searchParams?.order_type as string) || "";
  const keyword = (searchParams?.keyword as string) || "";
  const page = Number(searchParams?.page || 1);
  const pageSize = Number(searchParams?.page_size || 20);

  const today = formatDate(new Date());

  const [stats, orders, paidOrders, preparingOrders, readyOrders, completedOrders] =
    await Promise.all([
      apiGet<OrderStatsResponse>("/merchant/orders/stats", {
        start_date: today,
        end_date: today,
      }).catch(() => fallbackStats),
      apiGet<OrderResponse[] | { items?: OrderResponse[] }>("/merchant/orders", {
        page_id: page,
        page_size: pageSize,
        status: status || undefined,
      }).catch(() => fallbackOrders),
      apiGet<OrderResponse[]>("/merchant/orders", {
        page_id: 1,
        page_size: 50,
        status: "paid",
      }).catch(() => []),
      apiGet<OrderResponse[]>("/merchant/orders", {
        page_id: 1,
        page_size: 50,
        status: "preparing",
      }).catch(() => []),
      apiGet<OrderResponse[]>("/merchant/orders", {
        page_id: 1,
        page_size: 50,
        status: "ready",
      }).catch(() => []),
      apiGet<OrderResponse[]>("/merchant/orders", {
        page_id: 1,
        page_size: 50,
        status: "completed",
      }).catch(() => []),
    ]);

  const list = normalizeOrders(orders);

  const statusCounts = {
    paid: paidOrders.length,
    preparing: preparingOrders.length,
    ready: readyOrders.length,
    completed: completedOrders.length,
  };

  const todayRevenue = stats.total_revenue;

  return (
    <OrdersPageClient
      initialOrders={list}
      stats={stats}
      statusCounts={statusCounts}
      todayRevenue={todayRevenue}
      page={page}
      pageSize={pageSize}
      status={status}
      orderType={orderType}
      keyword={keyword}
    />
  );
}
