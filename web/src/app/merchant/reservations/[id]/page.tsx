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
import { ReservationActions } from "@/components/merchant/reservation-actions";
import { apiGet, apiPut } from "@/lib/api";

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

type Table = {
  id: number;
  table_no: string;
  table_type: string;
};

const fallbackDetail: Reservation = {
  id: 0,
  table_id: undefined,
  table_no: "",
  table_type: "",
  reservation_date: "",
  reservation_time: "",
  guest_count: 0,
  contact_name: "",
  contact_phone: "",
  payment_mode: "",
  deposit_amount: 0,
  prepaid_amount: 0,
  status: "",
  notes: "",
  created_at: "",
};

const fallbackTables: Table[] = [];

function normalizeTables(value: Table[] | undefined) {
  return Array.isArray(value) ? value : fallbackTables;
}

export default async function ReservationDetailPage({
  params,
}: {
  params: { id: string };
}) {
  const [detail, tables] = await Promise.all([
    apiGet<Reservation>(`/reservations/${params.id}`).catch(
      () => fallbackDetail
    ),
    apiGet<Table[]>("/tables").catch(() => fallbackTables),
  ]);

  const tableOptions = normalizeTables(tables);

  async function updateReservation(formData: FormData) {
    "use server";
    const payload = {
      table_id: Number(formData.get("table_id")) || undefined,
      date: formData.get("reservation_date")?.toString() || detail.reservation_date,
      time: formData.get("reservation_time")?.toString() || detail.reservation_time,
      guest_count: Number(formData.get("guest_count")) || detail.guest_count,
      contact_name: formData.get("contact_name")?.toString() || detail.contact_name,
      contact_phone: formData.get("contact_phone")?.toString() || detail.contact_phone,
      notes: formData.get("notes")?.toString() || detail.notes,
    };

    await apiPut(`/reservations/${params.id}/update`, payload);
  }

  return (
    <div className="page-content">
      <div className="flex items-center justify-between">
        <div className="space-y-1">
          <Link href="/merchant/reservations" className="text-sm text-muted-foreground">
            ← 返回预订列表
          </Link>
          <h1 className="text-xl font-semibold">预订详情</h1>
          <p className="text-sm text-muted-foreground">预订号 #{detail.id}</p>
        </div>
        <Badge variant="outline">{detail.status || "-"}</Badge>
      </div>

      <section className="grid gap-4 lg:grid-cols-[2fr_1fr]">
        <Card className="panel">
          <CardHeader>
            <CardTitle>预订信息</CardTitle>
            <CardDescription>对应 /v1/reservations/:id</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4 text-sm">
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">联系人</span>
                <span className="font-medium">{detail.contact_name}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">联系电话</span>
                <span className="font-medium">{detail.contact_phone}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">桌台</span>
                <span className="font-medium">{detail.table_no || "-"}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">用餐时间</span>
                <span className="font-medium">
                  {detail.reservation_date} {detail.reservation_time}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">人数</span>
                <span className="font-medium">{detail.guest_count}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">支付方式</span>
                <span className="font-medium">{detail.payment_mode || "-"}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">预付</span>
                <span className="font-medium">¥{detail.prepaid_amount ?? 0}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">定金</span>
                <span className="font-medium">¥{detail.deposit_amount ?? 0}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">创建时间</span>
                <span className="font-medium">{detail.created_at}</span>
              </div>
            </div>
            <div>
              <p className="text-muted-foreground">备注</p>
              <p className="mt-1 rounded-md border bg-muted/40 p-2">
                {detail.notes || "-"}
              </p>
            </div>
          </CardContent>
        </Card>

        <Card className="panel">
          <CardHeader>
            <CardTitle>预订操作</CardTitle>
            <CardDescription>确认 / 到店 / 起菜 / 完成 / 爽约</CardDescription>
          </CardHeader>
          <CardContent>
            <ReservationActions reservationId={detail.id} />
          </CardContent>
        </Card>
      </section>

      <section className="grid gap-4 lg:grid-cols-[2fr_1fr]">
        <Card className="panel">
          <CardHeader>
            <CardTitle>代客修改</CardTitle>
            <CardDescription>对应 /v1/reservations/:id/update</CardDescription>
          </CardHeader>
          <CardContent>
            <form action={updateReservation} className="grid gap-4 md:grid-cols-2">
              <label className="space-y-1 text-sm">
                <span className="text-muted-foreground">桌台</span>
                <select
                  name="table_id"
                  defaultValue={detail.table_id ?? ""}
                  className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm"
                >
                  <option value="">未分配</option>
                  {tableOptions.map((table) => (
                    <option key={table.id} value={table.id}>
                      {table.table_no} - {table.table_type}
                    </option>
                  ))}
                </select>
              </label>
              <label className="space-y-1 text-sm">
                <span className="text-muted-foreground">日期</span>
                <Input
                  type="date"
                  name="reservation_date"
                  defaultValue={detail.reservation_date}
                />
              </label>
              <label className="space-y-1 text-sm">
                <span className="text-muted-foreground">时间</span>
                <Input
                  type="time"
                  name="reservation_time"
                  defaultValue={detail.reservation_time}
                />
              </label>
              <label className="space-y-1 text-sm">
                <span className="text-muted-foreground">人数</span>
                <Input
                  type="number"
                  name="guest_count"
                  min={1}
                  defaultValue={detail.guest_count}
                />
              </label>
              <label className="space-y-1 text-sm">
                <span className="text-muted-foreground">联系人</span>
                <Input
                  name="contact_name"
                  defaultValue={detail.contact_name}
                />
              </label>
              <label className="space-y-1 text-sm">
                <span className="text-muted-foreground">联系电话</span>
                <Input
                  name="contact_phone"
                  defaultValue={detail.contact_phone}
                />
              </label>
              <label className="space-y-1 text-sm md:col-span-2">
                <span className="text-muted-foreground">备注</span>
                <textarea
                  name="notes"
                  defaultValue={detail.notes || ""}
                  className="min-h-20 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                />
              </label>
              <div className="md:col-span-2 flex justify-end gap-2">
                <Button type="submit">保存修改</Button>
              </div>
            </form>
          </CardContent>
        </Card>

        <Card className="panel">
          <CardHeader>
            <CardTitle>当日桌台</CardTitle>
            <CardDescription>对应 /v1/tables</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>桌台</TableHead>
                  <TableHead>类型</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {tableOptions.map((table) => (
                  <TableRow key={table.id}>
                    <TableCell className="font-medium">{table.table_no}</TableCell>
                    <TableCell>{table.table_type}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
