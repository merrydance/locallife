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
import { TableStatusActions } from "@/components/merchant/table-status-actions";
import { apiGet, formatAmount } from "@/lib/api";

type TableItem = {
  id: number;
  table_no: string;
  table_type: string;
  capacity: number;
  minimum_spend?: number;
  status: string;
  description?: string;
  qr_code_url?: string;
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

export default async function TablesPage() {
  const tables = await apiGet<
    TableItem[] | { items?: TableItem[]; tables?: TableItem[] }
  >("/tables").catch(() => fallbackTables);

  const list = normalizeTables(tables);

  return (
    <>
      <header className="page-header flex-col items-start gap-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <h1 className="text-xl font-semibold">桌台管理</h1>
            <Badge variant="secondary">共 {list.length} 台</Badge>
          </div>
          <div className="flex items-center gap-3">
            <Input placeholder="搜索桌台号" className="w-60" />
            <Button variant="outline">新增桌台</Button>
          </div>
        </div>
      </header>

      <main className="page-content">
        <Card className="panel">
          <CardHeader>
            <CardTitle>桌台列表</CardTitle>
            <CardDescription>对应 /v1/tables</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>桌台号</TableHead>
                  <TableHead>类型</TableHead>
                  <TableHead>容量</TableHead>
                  <TableHead>最低消费</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>操作</TableHead>
                  <TableHead className="text-right">详情</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {list.map((table) => (
                  <TableRow key={table.id}>
                    <TableCell className="font-medium">{table.table_no}</TableCell>
                    <TableCell>{table.table_type}</TableCell>
                    <TableCell>{table.capacity} 人</TableCell>
                    <TableCell>¥{formatAmount(table.minimum_spend)}</TableCell>
                    <TableCell>
                      <Badge variant="outline">
                        {statusLabels[table.status] || table.status}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <TableStatusActions
                        tableId={table.id}
                        currentStatus={table.status}
                      />
                    </TableCell>
                    <TableCell className="text-right">
                      <Link
                        href={`/merchant/tables/${table.id}`}
                        className="text-primary hover:underline"
                      >
                        查看
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
