import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { apiGet, formatAmount, formatDate } from "@/lib/api";

type Reservation = {
  id: number;
  table_id?: number;
  table_no?: string;
  table_type?: string;
  reservation_date: string;
  reservation_time: string;
  guest_count: number;
  contact_name: string;
  contact_phone: string;
  payment_mode?: string;
  deposit_amount?: number;
  prepaid_amount?: number;
  status: string;
  notes?: string;
  created_at: string;
};

type ReservationStats = {
  pending_count: number;
  paid_count: number;
  confirmed_count: number;
  completed_count: number;
  cancelled_count: number;
  expired_count: number;
  no_show_count: number;
};

const fallbackReservations: Reservation[] = [];

const fallbackStats: ReservationStats = {
  pending_count: 0,
  paid_count: 0,
  confirmed_count: 0,
  completed_count: 0,
  cancelled_count: 0,
  expired_count: 0,
  no_show_count: 0,
};

const statusLabels: Record<string, string> = {
  pending: "待确认",
  paid: "已支付",
  confirmed: "已确认",
  checked_in: "已到店",
  completed: "已完成",
  cancelled: "已取消",
  expired: "已过期",
  no_show: "爽约",
};

const statusTabs = [
  { label: "全部", value: "" },
  { label: "待确认", value: "pending" },
  { label: "已支付", value: "paid" },
  { label: "已确认", value: "confirmed" },
  { label: "已完成", value: "completed" },
  { label: "爽约", value: "no_show" },
];

function normalizeReservations(
  value:
    | Reservation[]
    | { items?: Reservation[]; reservations?: Reservation[] }
    | undefined
) {
  if (!value) return fallbackReservations;
  if (Array.isArray(value)) return value;
  if (value.items && Array.isArray(value.items)) return value.items;
  if (value.reservations && Array.isArray(value.reservations)) {
    return value.reservations;
  }
  return fallbackReservations;
}

export default async function ReservationsPage({
  searchParams,
}: {
  searchParams?: { status?: string; date?: string };
}) {
  const status = searchParams?.status || "";
  const date = searchParams?.date || formatDate(new Date());

  const [stats, reservations] = await Promise.all([
    apiGet<ReservationStats>("/reservations/merchant/stats").catch(
      () => fallbackStats
    ),
    apiGet<Reservation[] | { items?: Reservation[]; reservations?: Reservation[] }>(
      "/reservations/merchant",
      {
        page_id: 1,
        page_size: 20,
        status: status || undefined,
        date,
      }
    ).catch(() => fallbackReservations),
  ]);

  const list = normalizeReservations(reservations);

  return (
    <>
      <header className="page-header flex-col items-start gap-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <h1 className="text-xl font-semibold">预订管理</h1>
            <Badge variant="secondary">{date}</Badge>
          </div>
          <div className="flex items-center gap-3">
            <Input placeholder="搜索姓名/手机号" className="w-60" />
            <Button variant="outline">创建预订</Button>
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          {statusTabs.map((tab) => {
            const active = status === tab.value;
            const href = tab.value
              ? `/merchant/reservations?status=${tab.value}&date=${date}`
              : `/merchant/reservations?date=${date}`;
            return (
              <Link
                key={tab.value || "all"}
                href={href}
                className={`rounded-md border px-3 py-1.5 text-sm transition-colors ${
                  active
                    ? "border-primary bg-accent text-accent-foreground"
                    : "text-muted-foreground hover:bg-muted"
                }`}
              >
                {tab.label}
              </Link>
            );
          })}
        </div>
      </header>

      <main className="page-content">
        <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <Card className="panel">
            <CardHeader className="pb-2">
              <CardDescription>待确认</CardDescription>
              <CardTitle className="text-2xl">{stats.pending_count}</CardTitle>
            </CardHeader>
          </Card>
          <Card className="panel">
            <CardHeader className="pb-2">
              <CardDescription>已确认</CardDescription>
              <CardTitle className="text-2xl">{stats.confirmed_count}</CardTitle>
            </CardHeader>
          </Card>
          <Card className="panel">
            <CardHeader className="pb-2">
              <CardDescription>已完成</CardDescription>
              <CardTitle className="text-2xl">{stats.completed_count}</CardTitle>
            </CardHeader>
          </Card>
          <Card className="panel">
            <CardHeader className="pb-2">
              <CardDescription>爽约</CardDescription>
              <CardTitle className="text-2xl">{stats.no_show_count}</CardTitle>
            </CardHeader>
          </Card>
        </section>

        <Card className="panel">
          <CardHeader>
            <CardTitle>预订列表</CardTitle>
            <CardDescription>对应 /v1/reservations/merchant</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>联系人</TableHead>
                  <TableHead>桌台</TableHead>
                  <TableHead>时间</TableHead>
                  <TableHead>人数</TableHead>
                  <TableHead>支付</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead className="text-right">创建时间</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {list.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell>
                      <div className="font-medium">{item.contact_name}</div>
                      <div className="text-xs text-muted-foreground">
                        {item.contact_phone}
                      </div>
                    </TableCell>
                    <TableCell>
                      {item.table_no || "-"}
                      <div className="text-xs text-muted-foreground">
                        {item.table_type || ""}
                      </div>
                    </TableCell>
                    <TableCell>
                      {item.reservation_date} {item.reservation_time}
                    </TableCell>
                    <TableCell>{item.guest_count} 人</TableCell>
                    <TableCell>
                      {item.payment_mode || "-"}
                      <div className="text-xs text-muted-foreground">
                        预付 ¥{formatAmount(item.prepaid_amount)} / 定金 ¥
                        {formatAmount(item.deposit_amount)}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline">
                        {statusLabels[item.status] || item.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <Link
                        href={`/merchant/reservations/${item.id}`}
                        className="text-primary hover:underline"
                      >
                        {item.created_at}
                      </Link>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </main>
    </>
  );
}
