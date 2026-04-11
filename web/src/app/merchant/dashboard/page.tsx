import { DashboardPageClient } from "@/components/merchant/dashboard-page-client";
import { apiGet, formatAmount, formatDate } from "@/lib/api";
import type { OrderResponse } from "@/types/order";

type MerchantInfo = {
  id: number;
  name: string;
  is_open?: boolean;
};

type MerchantStatus = {
  is_open: boolean;
};

type StatsOverview = {
  total_orders: number;
  total_revenue: number;
  total_days: number;
  total_commission: number;
  avg_daily_sales: number;
};

type TableItem = {
  id: number;
  table_no: string;
  table_type?: string;
  status: string;
  capacity?: number;
  current_reservation_id?: number;
};

const fallbackMerchant: MerchantInfo = {
  id: 0,
  name: "商户工作台",
  is_open: false,
};

const fallbackStatus: MerchantStatus = {
  is_open: false,
};

const fallbackStats: StatsOverview = {
  total_orders: 0,
  total_revenue: 0,
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
  if (!data) return [] as OrderResponse[];
  if (Array.isArray(data)) return data;
  if (data.items && Array.isArray(data.items)) return data.items;
  if (data.orders && Array.isArray(data.orders)) return data.orders;
  return [] as OrderResponse[];
}

function normalizeTables(
  data:
    | TableItem[]
    | { items?: TableItem[]; tables?: TableItem[] }
    | undefined
) {
  if (!data) return [] as TableItem[];
  if (Array.isArray(data)) return data;
  if (data.items && Array.isArray(data.items)) return data.items;
  if (data.tables && Array.isArray(data.tables)) return data.tables;
  return [] as TableItem[];
}

function formatChineseDate(date: Date) {
  const weekDays = [
    "星期日",
    "星期一",
    "星期二",
    "星期三",
    "星期四",
    "星期五",
    "星期六",
  ];
  return `${date.getFullYear()}年${date.getMonth() + 1}月${date.getDate()}日 ${
    weekDays[date.getDay()]
  }`;
}

export default async function DashboardPage() {
  const today = formatDate(new Date());

  const [merchantInfo, merchantStatus, stats, ordersRaw, tablesRaw] =
    await Promise.all([
      apiGet<MerchantInfo>("/merchants/me").catch(() => fallbackMerchant),
      apiGet<MerchantStatus>("/merchants/me/status").catch(
        () => fallbackStatus
      ),
      apiGet<StatsOverview>("/merchant/stats/overview", {
        start_date: today,
        end_date: today,
      }).catch(() => fallbackStats),
      apiGet<OrderResponse[] | { items?: OrderResponse[]; orders?: OrderResponse[] }>(
        "/merchant/orders",
        { page_id: 1, page_size: 50 }
      ).catch(() => []),
      apiGet<TableItem[] | { items?: TableItem[]; tables?: TableItem[] }>(
        "/tables"
      ).catch(() => []),
    ]);

  const list = normalizeOrders(ordersRaw).filter((order) =>
    ["paid", "preparing", "ready"].includes(order.status)
  );

  const orders = list.map((order) => {
    const createdTime = order.created_at
      ? new Date(order.created_at)
      : null;
    const timeText = createdTime
      ? `${String(createdTime.getHours()).padStart(2, "0")}:${String(
          createdTime.getMinutes()
        ).padStart(2, "0")}`
      : "";
    const summary = order.items?.slice(0, 2).map((i) => i.name).join("、");
    return {
      id: order.id,
      order_no: order.order_no,
      status: order.status,
      status_text: order.status,
      order_type: order.order_type,
      order_type_text: order.order_type,
      total_amount: order.total_amount,
      amount_display: formatAmount(order.total_amount),
      items_summary: summary || "订单商品",
      table_no: order.table_id ? String(order.table_id) : undefined,
      created_at: order.created_at,
      created_time: timeText,
    };
  });

  const statusCounts = {
    paid: orders.filter((o) => o.status === "paid").length,
    preparing: orders.filter((o) => o.status === "preparing").length,
    ready: orders.filter((o) => o.status === "ready").length,
  };

  const tables = normalizeTables(tablesRaw);
  const tableStats = {
    total: tables.length,
    available: tables.filter((t) => t.status === "available").length,
    occupied: tables.filter((t) => t.status === "occupied").length,
  };

  const grouped = new Map<string, TableItem[]>();
  tables.forEach((table) => {
    const type = table.table_type || "table";
    if (!grouped.has(type)) grouped.set(type, []);
    grouped.get(type)!.push(table);
  });

  const tableGroups = [] as Array<{ name: string; type: string; tables: TableItem[] }>;
  if (grouped.has("table")) {
    tableGroups.push({ name: "散台", type: "table", tables: grouped.get("table")! });
  }
  if (grouped.has("room")) {
    tableGroups.push({ name: "包间", type: "room", tables: grouped.get("room")! });
  }
  grouped.forEach((value, key) => {
    if (key !== "table" && key !== "room") {
      tableGroups.push({ name: "其他", type: key, tables: value });
    }
  });

  return (
    <DashboardPageClient
      merchantName={merchantInfo.name}
      isOpen={merchantStatus.is_open ?? merchantInfo.is_open ?? false}
      currentDate={formatChineseDate(new Date())}
      wsConnected={false}
      revenue={stats.total_revenue}
      todayOrders={stats.total_orders}
      orders={orders}
      statusCounts={statusCounts}
      tableGroups={tableGroups}
      tableStats={tableStats}
    />
  );
}
