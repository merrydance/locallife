import { OrdersPageClient } from "@/components/merchant/orders-page-client";
import { apiGet, formatDate } from "@/lib/api";
import type { OrderResponse, OrderStatsResponse } from "@/types/order";

type StatsOverview = {
  total_orders: number;
  total_sales: number;
  total_days: number;
  total_commission: number;
  avg_daily_sales: number;
};

const fallbackOrders: OrderResponse[] = [];
const fallbackStats: OrderStatsResponse = {
  pending_count: 0,
  paid_count: 0,
  preparing_count: 0,
  ready_count: 0,
  delivering_count: 0,
  completed_count: 0,
  cancelled_count: 0,
};

const fallbackOverview: StatsOverview = {
  total_orders: 0,
  total_sales: 0,
  total_days: 0,
  total_commission: 0,
  avg_daily_sales: 0,
};

function normalizeOrders(
  data:
    | OrderResponse[]
    | { items?: OrderResponse[]; orders?: OrderResponse[] }
    | undefined
) {
  if (!data) return fallbackOrders;
  if (Array.isArray(data)) return data;
  if (data.items && Array.isArray(data.items)) return data.items;
  if (data.orders && Array.isArray(data.orders)) return data.orders;
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

  const [stats, overview, orders] = await Promise.all([
    apiGet<OrderStatsResponse>("/merchant/orders/stats", {
      start_date: today,
      end_date: today,
    }).catch(() => fallbackStats),
    apiGet<StatsOverview>("/merchant/stats/overview", {
      start_date: today,
      end_date: today,
    }).catch(() => fallbackOverview),
    apiGet<OrderResponse[] | { items?: OrderResponse[]; orders?: OrderResponse[] }>("/merchant/orders", {
      page_id: page,
      page_size: pageSize,
      status: status || undefined,
    }).catch(() => fallbackOrders),
  ]);

  const list = normalizeOrders(orders);

  const statusCounts = {
    paid: stats.paid_count || 0,
    preparing: stats.preparing_count || 0,
    ready: stats.ready_count || 0,
    completed: stats.completed_count || 0,
  };

  const todayRevenue = overview.total_sales;
  const todayOrders = overview.total_orders;

  return (
    <OrdersPageClient
      initialOrders={list}
      todayOrders={todayOrders}
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
