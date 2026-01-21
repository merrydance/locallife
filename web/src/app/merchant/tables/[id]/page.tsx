import Image from "next/image";
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
import { TableDeleteButton } from "@/components/merchant/table-delete-button";
import { apiGet, apiPatch, formatAmount } from "@/lib/api";

type TableItem = {
  id: number;
  table_no: string;
  table_type: string;
  capacity: number;
  minimum_spend?: number;
  status: string;
  description?: string;
  qr_code_url?: string;
  tags?: { id: number; name: string }[];
};

type TableImage = {
  id: number;
  image_url: string;
  is_primary?: boolean;
  sort_order?: number;
};

const fallbackDetail: TableItem = {
  id: 0,
  table_no: "",
  table_type: "",
  capacity: 0,
  minimum_spend: 0,
  status: "",
  description: "",
  qr_code_url: "",
  tags: [],
};

const fallbackImages: TableImage[] = [];

function normalizeImages(
  value: TableImage[] | { items?: TableImage[]; images?: TableImage[] } | undefined
) {
  if (!value) return fallbackImages;
  if (Array.isArray(value)) return value;
  if (value.items && Array.isArray(value.items)) return value.items;
  if (value.images && Array.isArray(value.images)) return value.images;
  return fallbackImages;
}

export default async function TableDetailPage({
  params,
}: {
  params: { id: string };
}) {
  const [detail, images] = await Promise.all([
    apiGet<TableItem>(`/tables/${params.id}`).catch(() => fallbackDetail),
    apiGet<TableImage[] | { items?: TableImage[]; images?: TableImage[] }>(
      `/tables/${params.id}/images`
    ).catch(() => fallbackImages),
  ]);

  const list = normalizeImages(images);

  async function updateTable(formData: FormData) {
    "use server";
    const payload = {
      table_no: formData.get("table_no")?.toString() || detail.table_no,
      table_type: formData.get("table_type")?.toString() || detail.table_type,
      capacity: Number(formData.get("capacity")) || detail.capacity,
      minimum_spend:
        Number(formData.get("minimum_spend")) || detail.minimum_spend || 0,
      description: formData.get("description")?.toString() || detail.description,
      status: formData.get("status")?.toString() || detail.status,
    };

    await apiPatch(`/tables/${params.id}`, payload);
  }

  return (
    <div className="page-content">
      <div className="flex items-center justify-between">
        <div className="space-y-1">
          <Link href="/merchant/tables" className="text-sm text-muted-foreground">
            ← 返回桌台列表
          </Link>
          <h1 className="text-xl font-semibold">桌台详情</h1>
          <p className="text-sm text-muted-foreground">桌台号 {detail.table_no}</p>
        </div>
        <Badge variant="outline">{detail.status}</Badge>
      </div>

      <section className="grid gap-4 lg:grid-cols-[2fr_1fr]">
        <Card className="panel">
          <CardHeader>
            <CardTitle>桌台信息</CardTitle>
            <CardDescription>对应 /v1/tables/:id</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4 text-sm">
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">桌台号</span>
                <span className="font-medium">{detail.table_no}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">类型</span>
                <span className="font-medium">{detail.table_type}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">容量</span>
                <span className="font-medium">{detail.capacity} 人</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">最低消费</span>
                <span className="font-medium">¥{formatAmount(detail.minimum_spend)}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">标签</span>
                <span className="font-medium">
                  {detail.tags?.map((tag) => tag.name).join("、") || "-"}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">描述</span>
                <span className="font-medium">{detail.description || "-"}</span>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <TableStatusActions tableId={detail.id} currentStatus={detail.status} />
            </div>
          </CardContent>
        </Card>

        <Card className="panel">
          <CardHeader>
            <CardTitle>二维码</CardTitle>
            <CardDescription>对应 /v1/tables/:id/qrcode</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {detail.qr_code_url ? (
              <Image
                src={detail.qr_code_url}
                alt="桌台二维码"
                width={160}
                height={160}
                className="h-40 w-40 rounded-md border"
              />
            ) : (
              <div className="rounded-md border border-dashed p-6 text-center text-sm text-muted-foreground">
                暂无二维码
              </div>
            )}
            {detail.qr_code_url ? (
              <Button asChild variant="outline" className="w-full">
                <a href={detail.qr_code_url} target="_blank" rel="noreferrer">
                  下载二维码
                </a>
              </Button>
            ) : null}
          </CardContent>
        </Card>
      </section>

      <section className="grid gap-4 lg:grid-cols-[2fr_1fr]">
        <Card className="panel">
          <CardHeader>
            <CardTitle>桌台编辑</CardTitle>
            <CardDescription>对应 /v1/tables/:id</CardDescription>
          </CardHeader>
          <CardContent>
            <form action={updateTable} className="grid gap-4 md:grid-cols-2">
              <label className="space-y-1 text-sm">
                <span className="text-muted-foreground">桌台号</span>
                <Input name="table_no" defaultValue={detail.table_no} />
              </label>
              <label className="space-y-1 text-sm">
                <span className="text-muted-foreground">类型</span>
                <Input name="table_type" defaultValue={detail.table_type} />
              </label>
              <label className="space-y-1 text-sm">
                <span className="text-muted-foreground">容量</span>
                <Input
                  type="number"
                  min={1}
                  name="capacity"
                  defaultValue={detail.capacity}
                />
              </label>
              <label className="space-y-1 text-sm">
                <span className="text-muted-foreground">最低消费</span>
                <Input
                  type="number"
                  min={0}
                  name="minimum_spend"
                  defaultValue={detail.minimum_spend ?? 0}
                />
              </label>
              <label className="space-y-1 text-sm">
                <span className="text-muted-foreground">状态</span>
                <Input name="status" defaultValue={detail.status} />
              </label>
              <label className="space-y-1 text-sm md:col-span-2">
                <span className="text-muted-foreground">描述</span>
                <textarea
                  name="description"
                  defaultValue={detail.description || ""}
                  className="min-h-20 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                />
              </label>
              <div className="md:col-span-2 flex justify-end gap-2">
                <Button type="submit">保存修改</Button>
                <TableDeleteButton tableId={detail.id} />
              </div>
            </form>
          </CardContent>
        </Card>

        <Card className="panel">
          <CardHeader>
            <CardTitle>桌台图片</CardTitle>
            <CardDescription>对应 /v1/tables/:id/images</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>图片</TableHead>
                  <TableHead>主图</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {list.map((image) => (
                  <TableRow key={image.id}>
                    <TableCell>
                      {image.image_url ? (
                        <a
                          href={image.image_url}
                          target="_blank"
                          rel="noreferrer"
                          className="text-primary hover:underline"
                        >
                          查看图片
                        </a>
                      ) : (
                        "-"
                      )}
                    </TableCell>
                    <TableCell>{image.is_primary ? "是" : "否"}</TableCell>
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
