import { apiGet } from "@/lib/api";
import type { KitchenOrdersResponse } from "@/types/kitchen";
import { KdsPageClient } from "@/components/merchant/kds-page-client";

const fallbackOrders: KitchenOrdersResponse = {
  new_orders: [],
  preparing_orders: [],
  ready_orders: [],
  stats: {
    new_count: 0,
    preparing_count: 0,
    ready_count: 0,
    completed_today_count: 0,
    avg_prepare_time: 15,
  },
};

export default async function KitchenPage() {
  const data = await apiGet<KitchenOrdersResponse>("/kitchen/orders").catch(
    () => fallbackOrders
  );

  return <KdsPageClient initialData={data} />;
}
