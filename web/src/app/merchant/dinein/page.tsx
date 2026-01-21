import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { DiningSessionActions } from "@/components/merchant/dining-session-actions";
import { TableStatusActions } from "@/components/merchant/table-status-actions";
import { apiGet, formatAmount } from "@/lib/api";

type TableItem = {
  id: number;
  table_no: string;
  table_type: string;
  status: string;
  capacity: number;
  minimum_spend?: number;
  description?: string;
  current_reservation_id?: number;
};

const fallbackTables: TableItem[] = [];

const statusLabels: Record<string, string> = {
  available: "空闲",
  occupied: "占用",
  cleaning: "清洁中",
  disabled: "停用",
};

function normalizeTables(
  value: TableItem[] | { items?: TableItem[]; tables?: TableItem[] } | undefined
) {
  if (!value) return fallbackTables;
  if (Array.isArray(value)) return value;
  if (value.items && Array.isArray(value.items)) return value.items;
  if (value.tables && Array.isArray(value.tables)) return value.tables;
  return fallbackTables;
}

export default async function DineInPage() {
  const tables = await apiGet<
    TableItem[] | { items?: TableItem[]; tables?: TableItem[] }
  >("/tables", {
    table_type: "table",
  }).catch(() => fallbackTables);

  const list = normalizeTables(tables);

  return (
    <>
      <header className="page-header">
        <div className="space-y-1">
          <h1 className="text-xl font-semibold">堂食管理</h1>
          <p className="text-sm text-muted-foreground">对应 /v1/tables</p>
        </div>
        <Badge variant="secondary">桌台数 {list.length}</Badge>
      </header>

      <div className="page-content">

      <section className="grid gap-4 lg:grid-cols-[2fr_1fr]">
        <Card className="panel">
          <CardHeader>
            <CardTitle>桌台状态</CardTitle>
            <CardDescription>开台/结台/状态调整</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>桌台</TableHead>
                  <TableHead>容量</TableHead>
                  <TableHead>最低消费</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>预订</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {list.map((table) => (
                  <TableRow key={table.id}>
                    <TableCell className="font-medium">{table.table_no}</TableCell>
                    <TableCell>{table.capacity} 人</TableCell>
                    <TableCell>¥{formatAmount(table.minimum_spend)}</TableCell>
                    <TableCell>
                      <Badge variant="outline">
                        {statusLabels[table.status] || table.status}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {table.current_reservation_id
                        ? `#${table.current_reservation_id}`
                        : "-"}
                    </TableCell>
                    <TableCell className="text-right">
                      <TableStatusActions
                        tableId={table.id}
                        currentStatus={table.status}
                      />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card className="panel">
          <CardHeader>
            <CardTitle>用餐会话</CardTitle>
            <CardDescription>开台预检 / 开台 / 转台</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <DiningSessionActions />
            <div className="rounded-md border bg-muted/40 p-3 text-xs text-muted-foreground">
              对应 /v1/dining-sessions/precheck、/v1/dining-sessions/open、/v1/dining-sessions/:id/transfer-table
            </div>
          </CardContent>
        </Card>
      </section>

      <Card className="panel">
        <CardHeader>
          <CardTitle>快速操作</CardTitle>
          <CardDescription>常用堂食入口</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-2">
          <Button variant="outline">桌台二维码</Button>
          <Button variant="outline">预订入座</Button>
          <Button variant="outline">用餐会话</Button>
          <Button variant="outline">堂食订单</Button>
        </CardContent>
      </Card>
      </div>
    </>
  );
}
