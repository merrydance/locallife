import Link from "next/link";
import { Badge } from "@/components/ui/badge";
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
import { OrderActions } from "@/components/merchant/order-actions";
import { apiGet } from "@/lib/api";

type OrderItem = {
  name: string;
  quantity: number;
  unit_price: number;
  subtotal: number;
  customizations?: string[];
};

type OrderDetail = {
  id: number;
  order_no: string;
  order_type: string;
  status: string;
  user_id: number;
  items: OrderItem[];
  subtotal: number;
  delivery_fee: number;
  delivery_fee_discount: number;
  discount_amount: number;
  total_amount: number;
  payment_method?: string;
  notes?: string;
  created_at: string;
  paid_at?: string;
  completed_at?: string;
  table_no?: string;
};

const fallbackDetail: OrderDetail = {
  id: 0,
  order_no: "",
  order_type: "",
  status: "",
  user_id: 0,
  items: [],
  subtotal: 0,
  delivery_fee: 0,
  delivery_fee_discount: 0,
  discount_amount: 0,
  total_amount: 0,
  payment_method: "",
  notes: "",
  created_at: "",
  paid_at: "",
  completed_at: "",
  table_no: "",
};

export default async function OrderDetailPage({
  params,
}: {
  params: { id: string };
}) {
  const detail = await apiGet<OrderDetail>(
    `/merchant/orders/${params.id}`
  ).catch(() => fallbackDetail);

  return (
    <div className="page-content">
      <div className="flex items-center justify-between">
        <div className="space-y-1">
          <Link href="/merchant/orders" className="text-sm text-muted-foreground">
            ← 返回订单列表
          </Link>
          <h1 className="text-xl font-semibold">订单详情</h1>
          <p className="text-sm text-muted-foreground">{detail.order_no}</p>
        </div>
        <Badge variant="outline">{detail.status || "-"}</Badge>
      </div>

      <section className="grid gap-4 lg:grid-cols-[2fr_1fr]">
        <Card className="panel">
          <CardHeader>
            <CardTitle>订单信息</CardTitle>
            <CardDescription>对应 /v1/merchant/orders/:id</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4 text-sm">
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">订单类型</span>
                <span className="font-medium">{detail.order_type}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">桌台</span>
                <span className="font-medium">{detail.table_no || "-"}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">用户 ID</span>
                <span className="font-medium">{detail.user_id}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">支付方式</span>
                <span className="font-medium">{detail.payment_method || "-"}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">下单时间</span>
                <span className="font-medium">{detail.created_at}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">支付时间</span>
                <span className="font-medium">{detail.paid_at || "-"}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">完成时间</span>
                <span className="font-medium">{detail.completed_at || "-"}</span>
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
            <CardTitle>操作</CardTitle>
            <CardDescription>接单/拒单/出餐/完成</CardDescription>
          </CardHeader>
          <CardContent>
            <OrderActions orderId={detail.id} status={detail.status} />
          </CardContent>
        </Card>
      </section>

      <Card className="panel">
        <CardHeader>
          <CardTitle>商品明细</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>菜品</TableHead>
                <TableHead>数量</TableHead>
                <TableHead>单价</TableHead>
                <TableHead className="text-right">小计</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {detail.items.map((item, index) => (
                <TableRow key={`${item.name}-${index}`}>
                  <TableCell>
                    <div className="font-medium">{item.name}</div>
                    {item.customizations && item.customizations.length > 0 ? (
                      <div className="text-xs text-muted-foreground">
                        {item.customizations.join("、")}
                      </div>
                    ) : null}
                  </TableCell>
                  <TableCell>{item.quantity}</TableCell>
                  <TableCell>¥{item.unit_price.toFixed(2)}</TableCell>
                  <TableCell className="text-right">
                    ¥{item.subtotal.toFixed(2)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card className="panel">
        <CardHeader>
          <CardTitle>费用汇总</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">商品小计</span>
            <span className="font-medium">¥{detail.subtotal.toFixed(2)}</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">配送费</span>
            <span className="font-medium">¥{detail.delivery_fee.toFixed(2)}</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">配送减免</span>
            <span className="font-medium">
              -¥{detail.delivery_fee_discount.toFixed(2)}
            </span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">优惠金额</span>
            <span className="font-medium">-¥{detail.discount_amount.toFixed(2)}</span>
          </div>
          <div className="flex items-center justify-between text-base">
            <span className="font-semibold">应付总计</span>
            <span className="font-semibold">¥{detail.total_amount.toFixed(2)}</span>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
