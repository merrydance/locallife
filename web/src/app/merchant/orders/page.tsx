import { OrdersPageClient } from "@/components/merchant/orders-page-client";
import { apiGet, formatDate } from "@/lib/api";
import type { OrderResponse, OrderStatsResponse } from "@/types/order";

const fallbackStats: OrderStatsResponse = {
  pending_count: 0,
  paid_count: 0,
  preparing_count: 0,
  ready_count: 0,
  delivering_count: 0,
  completed_count: 0,
  cancelled_count: 0,
};

interface MerchantOrdersResponse {
  orders: OrderResponse[];
  total: number;
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

  const [stats, ordersRes, overview] = await Promise.all([
    apiGet<OrderStatsResponse>("/merchant/orders/stats", {
      start_date: today,
      end_date: today,
    }).catch(() => fallbackStats),
    apiGet<MerchantOrdersResponse>("/merchant/orders", {
      page_id: page,
      page_size: pageSize,
      status: status || undefined,
    ).catch(() => ({ orders: [], total: 0 })),
    apiGet<{ total_sales: number; total_orders: number }>("/merchant/stats/overview", {
      start_date: today,
      end_date: today,
    }).catch(() => ({ total_sales: 0, total_orders: 0 })),
  ]);

  const orders = ordersRes.orders || [];
  const totalCount = ordersRes.total || 0;

  return (
    <OrdersPageClient
      initialOrders={orders}
      totalCount={totalCount}
      statusCounts={stats}
      todayRevenue={overview.total_sales}
      todayOrders={overview.total_orders}
      page={page}
      pageSize={pageSize}
      status={status}
      orderType={orderType}
      keyword={keyword}
    />
  );
}
