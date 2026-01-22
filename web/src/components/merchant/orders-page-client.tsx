"use client";

import { useState, useMemo, useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { 
  Search, 
  RefreshCw, 
  Download, 
  Printer, 
  MoreVertical, 
  ShoppingBag,
  User,
  Phone,
  MapPin,
  Clock,
  ArrowRight,
  CheckCircle2,
  XCircle,
  Truck,
  Coffee,
  AlertCircle
} from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { Checkbox } from "@/components/ui/checkbox";
import { PageShell, PageHeader, PageContent } from "@/components/merchant/layout/page-shell";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { apiGet, apiPost, formatAmount, formatDate } from "@/lib/api";
import { cn } from "@/lib/utils";
import type { OrderResponse, OrderStatsResponse } from "@/types/order";

const STATUS_LABELS: Record<string, string> = {
  pending: "待支付",
  paid: "待接单",
  preparing: "制作中",
  ready: "待出餐",
  courier_accepted: "骑手已接单",
  picked: "已取餐",
  delivering: "配送中",
  rider_delivered: "骑手送达",
  user_delivered: "用户确认送达",
  completed: "已完成",
  cancelled: "已取消",
};

const STATUS_COLORS: Record<string, string> = {
  pending: "bg-amber-500",
  paid: "bg-blue-500",
  preparing: "bg-orange-500",
  ready: "bg-emerald-500",
  delivering: "bg-purple-500",
  completed: "bg-slate-500",
  cancelled: "bg-rose-500",
};

const TYPE_LABELS: Record<string, string> = {
  takeout: "外卖",
  dine_in: "堂食",
  takeaway: "自取",
  reservation: "预订",
};

const STATUS_TABS = [
  { label: "全部", value: "" },
  { label: "待接单", value: "paid" },
  { label: "制作中", value: "preparing" },
  { label: "待出餐", value: "ready" },
  { label: "已完成", value: "completed" },
  { label: "已取消", value: "cancelled" },
];

interface OrdersPageClientProps {
  initialOrders: OrderResponse[];
  totalCount: number;
  statusCounts: OrderStatsResponse;
  todayRevenue: number;
  todayOrders: number;
  page: number;
  pageSize: number;
  status: string;
  orderType: string;
  keyword: string;
}

export function OrdersPageClient({
  initialOrders,
  totalCount,
  statusCounts,
  todayRevenue,
  todayOrders,
  page,
  pageSize,
  status,
  orderType,
  keyword,
}: OrdersPageClientProps) {
  const router = useRouter();
  const searchParams = useSearchParams();
  
  const [orders, setOrders] = useState<OrderResponse[]>(initialOrders);
  const [selectedOrderId, setSelectedOrderId] = useState<number | null>(initialOrders[0]?.id || null);
  const [loading, setLoading] = useState(false);
  const [loadingDetail, setLoadingDetail] = useState(false);
  const [selectedOrder, setSelectedOrder] = useState<OrderResponse | null>(null);
  
  const [searchValue, setSearchValue] = useState(keyword);
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  
  const [rejectDialogOpen, setRejectDialogOpen] = useState(false);
  const [rejectReason, setRejectReason] = useState("");
  const [rejectingOrderId, setRejectingOrderId] = useState<number | null>(null);
  const [batchAcceptOpen, setBatchAcceptOpen] = useState(false);

  // Sync state with props
  useEffect(() => {
    setOrders(initialOrders);
    if (initialOrders.length > 0 && !selectedOrderId) {
      handleSelectOrder(initialOrders[0].id);
    }
  }, [initialOrders]);

  // Load order detail when selection changes
  useEffect(() => {
    if (selectedOrderId) {
      loadOrderDetail(selectedOrderId);
    }
  }, [selectedOrderId]);

  const loadOrderDetail = async (id: number) => {
    setLoadingDetail(true);
    try {
      const data = await apiGet<OrderResponse>(`/merchant/orders/${id}`);
      setSelectedOrder(data);
    } catch (error) {
      toast.error("加载订单详情失败");
    } finally {
      setLoadingDetail(false);
    }
  };

  const handleSelectOrder = (id: number) => {
    setSelectedOrderId(id);
  };

  const updateQuery = (next: Record<string, string | number | undefined>) => {
    const params = new URLSearchParams(searchParams?.toString());
    Object.entries(next).forEach(([key, value]) => {
      if (value === undefined || value === "") {
        params.delete(key);
      } else {
        params.set(key, String(value));
      }
    });
    router.push(`/merchant/orders?${params.toString()}`);
  };

  const handleSearch = () => {
    updateQuery({ keyword: searchValue, page: 1 });
  };

  const runAction = async (orderId: number, action: string, payload?: Record<string, unknown>) => {
    try {
      await apiPost(`/merchant/orders/${orderId}/${action}`, payload);
      toast.success("操作成功");
      // Refresh list or detail
      if (selectedOrderId === orderId) {
        loadOrderDetail(orderId);
      }
      updateQuery({}); // Trigger server components refresh
    } catch (error: any) {
      toast.error(error.message || "操作失败");
    }
  };

  const handleReject = (orderId: number) => {
    setRejectingOrderId(orderId);
    setRejectReason("");
    setRejectDialogOpen(true);
  };

  const confirmReject = async () => {
    if (!rejectingOrderId || !rejectReason.trim()) {
      toast.error("请输入拒单原因");
      return;
    }
    await runAction(rejectingOrderId, "reject", { reason: rejectReason });
    setRejectDialogOpen(false);
  };

  const toggleSelect = (e: React.MouseEvent, id: number) => {
    e.stopPropagation();
    const next = new Set(selectedIds);
    if (next.has(id)) {
      next.delete(id);
    } else {
      next.add(id);
    }
    setSelectedIds(next);
  };

  const toggleSelectAll = () => {
    if (selectedIds.size === orders.length) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(orders.map(o => o.id)));
    }
  };

  const batchAccept = async () => {
    const ids = Array.from(selectedIds);
    const paidOrderIds = orders.filter(o => selectedIds.has(o.id) && o.status === "paid").map(o => o.id);
    
    if (paidOrderIds.length === 0) {
      toast.error("请选择待接单状态的订单");
      return;
    }

    try {
      await Promise.all(paidOrderIds.map(id => apiPost(`/merchant/orders/${id}/accept`)));
      toast.success(`已成功接收 ${paidOrderIds.length} 个订单`);
      setSelectedIds(new Set());
      updateQuery({});
    } catch (error: any) {
      toast.error("批量处理部分失败");
    }
  };

  return (
    <PageShell>
      <PageHeader 
        title="订单管理" 
        description="实时监控和处理商户订单流程"
        actions={
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={() => updateQuery({})}>
              <RefreshCw className="h-4 w-4 mr-2" />
              刷新
            </Button>
            <Button variant="outline" size="sm" onClick={() => toast.info("开发中")}>
              <Download className="h-4 w-4 mr-2" />
              导出
            </Button>
          </div>
        }
      />

      <PageContent>
        {/* Stats Summary Cards */}
        <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-5 gap-4 mb-6">
          <Card 
            className={cn("cursor-pointer transition-all hover:ring-2 ring-primary/20", status === "paid" && "ring-2 ring-primary")}
            onClick={() => updateQuery({ status: "paid", page: 1 })}
          >
            <div className="p-4">
              <div className="text-xs text-muted-foreground mb-1">待接单</div>
              <div className="text-2xl font-bold text-blue-600">{statusCounts.paid_count}</div>
              <div className="mt-1 h-1 w-full bg-blue-100 rounded-full overflow-hidden">
                <div className="h-full bg-blue-500" style={{ width: "100%" }}></div>
              </div>
            </div>
          </Card>
          <Card 
            className={cn("cursor-pointer transition-all hover:ring-2 ring-primary/20", status === "preparing" && "ring-2 ring-primary")}
            onClick={() => updateQuery({ status: "preparing", page: 1 })}
          >
            <div className="p-4">
              <div className="text-xs text-muted-foreground mb-1">正在制作</div>
              <div className="text-2xl font-bold text-orange-500">{statusCounts.preparing_count}</div>
              <div className="mt-1 h-1 w-full bg-orange-100 rounded-full overflow-hidden">
                <div className="h-full bg-orange-500" style={{ width: "100%" }}></div>
              </div>
            </div>
          </Card>
          <Card 
            className={cn("cursor-pointer transition-all hover:ring-2 ring-primary/20", status === "ready" && "ring-2 ring-primary")}
            onClick={() => updateQuery({ status: "ready", page: 1 })}
          >
            <div className="p-4">
              <div className="text-xs text-muted-foreground mb-1">待出餐/待取</div>
              <div className="text-2xl font-bold text-emerald-500">{statusCounts.ready_count}</div>
              <div className="mt-1 h-1 w-full bg-emerald-100 rounded-full overflow-hidden">
                <div className="h-full bg-emerald-500" style={{ width: "100%" }}></div>
              </div>
            </div>
          </Card>
          <Card 
            className={cn("cursor-pointer transition-all hover:ring-2 ring-primary/20", status === "completed" && "ring-2 ring-primary")}
            onClick={() => updateQuery({ status: "completed", page: 1 })}
          >
            <div className="p-4">
              <div className="text-xs text-muted-foreground mb-1">今日已完成</div>
              <div className="text-2xl font-bold text-slate-700">{statusCounts.completed_count}</div>
              <div className="mt-1 h-1 w-full bg-slate-100 rounded-full overflow-hidden">
                <div className="h-full bg-slate-500" style={{ width: "100%" }}></div>
              </div>
            </div>
          </Card>
          <Card className="bg-primary/5 border-primary/20 lg:col-span-1 col-span-2">
            <div className="p-4">
              <div className="text-xs text-primary/70 mb-1 font-medium">今日概览</div>
              <div className="flex items-baseline gap-1 font-bold text-primary">
                <span className="text-sm">¥</span>
                <span className="text-2xl">{formatAmount(todayRevenue)}</span>
              </div>
              <div className="text-[10px] text-muted-foreground mt-1">
                今日共 {todayOrders} 单
              </div>
            </div>
          </Card>
        </div>

        <div className="flex h-[calc(100vh-16rem)] gap-6">
          {/* Left Panel: Order List */}
          <div className="w-1/3 min-w-[360px] flex flex-col bg-white rounded-xl border shadow-sm">
            {/* Filter & Search */}
            <div className="p-4 border-b space-y-4">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-semibold">订单列表 ({totalCount})</h3>
                <div className="flex items-center gap-2">
                  <Badge variant="outline" className="text-[10px] font-normal cursor-pointer" onClick={toggleSelectAll}>
                    {selectedIds.size === orders.length ? "取消全选" : "全选"}
                  </Badge>
                  {selectedIds.size > 0 && (
                     <Button size="sm" variant="default" className="h-6 text-[10px] px-2" onClick={() => setBatchAcceptOpen(true)}>
                       批量处理
                     </Button>
                  )}
                </div>
              </div>
              
              <div className="flex gap-1 overflow-x-auto pb-1 no-scrollbar">
                {STATUS_TABS.map(tab => (
                  <Button 
                    key={tab.value}
                    variant={status === tab.value ? "default" : "ghost"}
                    size="sm"
                    className={cn("h-7 px-3 text-xs rounded-full", status !== tab.value && "hover:bg-slate-100")}
                    onClick={() => updateQuery({ status: tab.value, page: 1 })}
                  >
                    {tab.label}
                  </Button>
                ))}
              </div>

              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input 
                  placeholder="搜索单号、手机号、备注" 
                  className="pl-9 h-9 bg-slate-50 border-slate-200 text-sm focus:bg-white transition-all"
                  value={searchValue}
                  onChange={(e) => setSearchValue(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && handleSearch()}
                />
              </div>
            </div>

            {/* List Scroll Area */}
            <ScrollArea className="flex-1">
              <div className="p-2 space-y-1">
                {orders.map(order => (
                  <div 
                    key={order.id}
                    onClick={() => handleSelectOrder(order.id)}
                    className={cn(
                      "group p-3 rounded-lg border transition-all cursor-pointer relative",
                      selectedOrderId === order.id 
                        ? "border-primary bg-primary/5 shadow-sm ring-1 ring-primary/20" 
                        : "border-transparent hover:bg-slate-50 hover:border-slate-200"
                    )}
                  >
                    {selectedOrderId === order.id && (
                       <div className="absolute left-0 top-3 bottom-3 w-1 bg-primary rounded-r-md" />
                    )}
                    
                    <div className="flex items-center justify-between mb-2">
                      <div className="flex items-center gap-2">
                        <Checkbox 
                          checked={selectedIds.has(order.id)}
                          onCheckedChange={() => {}}
                          onClick={(e) => toggleSelect(e as any, order.id)}
                          className="h-4 w-4"
                        />
                        <span className="font-bold text-slate-900 text-sm italic">#{order.order_no.slice(-4)}</span>
                        <Badge variant="outline" className="text-[10px] h-4 px-1 py-0 border-slate-200 text-slate-500 font-normal">
                          {TYPE_LABELS[order.order_type]}
                        </Badge>
                      </div>
                      <div className={cn("h-2 w-2 rounded-full", STATUS_COLORS[order.status] || "bg-slate-300")} />
                    </div>

                    <div className="flex justify-between items-end">
                      <div className="space-y-1">
                         <div className="text-xs font-medium text-slate-700 line-clamp-1">
                           {order.items.map(i => i.name).join("、")}
                         </div>
                         <div className="flex items-center text-[10px] text-muted-foreground gap-2">
                           <Clock className="h-3 w-3" />
                           {new Date(order.created_at).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}
                         </div>
                      </div>
                      <div className="text-right">
                        <div className="text-sm font-bold text-primary">
                          <span className="text-[10px] font-normal mr-0.5">¥</span>
                          {formatAmount(order.total_amount)}
                        </div>
                        <div className="text-[10px] text-muted-foreground">共 {order.items.reduce((s, i) => s + i.quantity, 0)} 件</div>
                      </div>
                    </div>
                    
                    {order.notes && (
                      <div className="mt-2 text-[10px] bg-amber-50 text-amber-700 px-2 py-0.5 rounded border border-amber-100 line-clamp-1">
                        备注: {order.notes}
                      </div>
                    )}
                  </div>
                ))}
                
                {orders.length === 0 && (
                  <div className="flex flex-col items-center justify-center py-20 text-muted-foreground space-y-2">
                    <ShoppingBag className="h-10 w-10 opacity-10" />
                    <p className="text-xs">暂无符合条件的订单</p>
                  </div>
                )}
              </div>
            </ScrollArea>

            {/* Pagination helper */}
            <div className="p-3 border-t bg-slate-50 rounded-b-xl flex items-center justify-between">
               <div className="text-xs text-muted-foreground">
                  {page} / {Math.ceil(totalCount / pageSize)} 页
               </div>
               <div className="flex gap-1">
                 <Button 
                   variant="outline" 
                   size="sm" 
                   className="h-7 w-7 p-0" 
                   disabled={page <= 1}
                   onClick={() => updateQuery({ page: page - 1 })}
                 >
                   &lt;
                 </Button>
                 <Button 
                   variant="outline" 
                   size="sm" 
                   className="h-7 w-7 p-0" 
                   disabled={orders.length < pageSize}
                   onClick={() => updateQuery({ page: page + 1 })}
                 >
                   &gt;
                 </Button>
               </div>
            </div>
          </div>

          {/* Right Panel: Order Detailed & Actions */}
          <div className="flex-1 bg-white rounded-xl border shadow-sm flex flex-col min-w-0">
            {loadingDetail ? (
               <div className="flex-1 flex flex-col items-center justify-center space-y-3">
                 <div className="relative">
                   <div className="h-12 w-12 rounded-full border-4 border-slate-100"></div>
                   <div className="h-12 w-12 rounded-full border-4 border-primary border-t-transparent animate-spin absolute top-0 left-0"></div>
                 </div>
                 <p className="text-sm text-muted-foreground animate-pulse">正在获取订单最新状态...</p>
               </div>
            ) : selectedOrder ? (
              <>
                {/* Header Action Bar */}
                <div className="p-4 border-b flex items-center justify-between bg-slate-50/50 rounded-t-xl shrink-0">
                  <div className="flex items-center gap-3">
                    <div className={cn("h-8 w-8 rounded-lg flex items-center justify-center text-white", STATUS_COLORS[selectedOrder.status] || "bg-slate-400")}>
                      {selectedOrder.status === 'paid' ? <AlertCircle className="h-5 w-5" /> : 
                       selectedOrder.status === 'preparing' ? <Coffee className="h-5 w-5" /> :
                       selectedOrder.status === 'ready' ? <CheckCircle2 className="h-5 w-5" /> :
                       selectedOrder.status === 'delivering' ? <Truck className="h-5 w-5" /> :
                       <ShoppingBag className="h-5 w-5" />}
                    </div>
                    <div>
                      <div className="flex items-center gap-2">
                        <h2 className="text-lg font-bold">订单 #{selectedOrder.order_no}</h2>
                        <Badge className={cn("text-[10px] font-medium border-0", STATUS_COLORS[selectedOrder.status] || "bg-slate-400")}>
                          {STATUS_LABELS[selectedOrder.status]}
                        </Badge>
                      </div>
                      <div className="text-[10px] text-muted-foreground flex items-center gap-3">
                        <span>下单: {new Date(selectedOrder.created_at).toLocaleString()}</span>
                        <span>渠道: {TYPE_LABELS[selectedOrder.order_type]}</span>
                      </div>
                    </div>
                  </div>

                  <div className="flex gap-2">
                    <Button variant="outline" size="sm" className="h-9 px-3" onClick={() => toast.info("打印已发送")}>
                      <Printer className="h-4 w-4 mr-2" />
                      打印
                    </Button>
                    
                    {/* Dynamic Actions */}
                    {selectedOrder.status === 'paid' && (
                      <>
                        <Button variant="destructive" size="sm" className="h-9 px-3" onClick={() => handleReject(selectedOrder.id)}>
                          拒单
                        </Button>
                        <Button variant="default" size="sm" className="h-9 px-4 bg-blue-600 hover:bg-blue-700" onClick={() => runAction(selectedOrder.id, "accept")}>
                          接单开制
                        </Button>
                      </>
                    )}

                    {selectedOrder.status === 'preparing' && (
                       <Button variant="default" size="sm" className="h-9 px-4 bg-orange-500 hover:bg-orange-600 border-orange-600" onClick={() => runAction(selectedOrder.id, "ready")}>
                         出餐/发货
                       </Button>
                    )}

                    {(selectedOrder.status === 'ready' || selectedOrder.status === 'delivering') && (
                       <Button variant="default" size="sm" className="h-9 px-4 bg-emerald-600 hover:bg-emerald-700" onClick={() => runAction(selectedOrder.id, "complete")}>
                         确认送达/完成
                       </Button>
                    )}
                  </div>
                </div>

                <ScrollArea className="flex-1">
                  <div className="p-6 space-y-8">
                    {/* Summary Info */}
                    <div className="grid grid-cols-3 gap-6">
                       <section className="space-y-3">
                          <h4 className="text-[10px] font-bold text-slate-400 uppercase tracking-wider flex items-center gap-1.5">
                            <User className="h-3 w-3" /> 客人信息
                          </h4>
                          <div className="space-y-2">
                            <div className="text-sm font-semibold">{selectedOrder.delivery_contact_name || "普通用户"}</div>
                            <div className="text-xs text-muted-foreground flex items-center gap-2">
                              <Phone className="h-3 w-3" />
                              {selectedOrder.delivery_contact_phone || "--"}
                            </div>
                            {selectedOrder.delivery_address && (
                              <div className="text-xs text-muted-foreground flex items-start gap-2">
                                <MapPin className="h-3 w-3 mt-0.5 shrink-0" />
                                <span className="line-clamp-2">{selectedOrder.delivery_address}</span>
                              </div>
                            )}
                            {selectedOrder.table_id && (
                               <div className="text-xs text-muted-foreground flex items-center gap-2">
                                 <Badge variant="secondary" className="font-normal text-[10px]">
                                   桌号: {selectedOrder.table_id}
                                 </Badge>
                               </div>
                            )}
                          </div>
                       </section>

                       <section className="space-y-3">
                          <h4 className="text-[10px] font-bold text-slate-400 uppercase tracking-wider flex items-center gap-1.5">
                            <Truck className="h-3 w-3" /> 履约详情
                          </h4>
                          <div className="space-y-2">
                             <div className="flex items-center gap-2">
                               <Badge variant="outline" className="font-normal border-amber-200 bg-amber-50 text-amber-700">
                                 {selectedOrder.fulfillment_status === 'pending_kitchen' ? '待接单' : 
                                  selectedOrder.fulfillment_status === 'preparing' ? '制作中' :
                                  selectedOrder.fulfillment_status === 'ready' ? '待配送/取' :
                                  selectedOrder.fulfillment_status === 'completed' ? '履约完成' : selectedOrder.fulfillment_status}
                               </Badge>
                             </div>
                             {selectedOrder.pickup_code && (
                                <div className="text-xs font-bold text-primary flex items-center gap-2">
                                  取餐码: <span className="text-lg tracking-widest">{selectedOrder.pickup_code}</span>
                                </div>
                             )}
                             {selectedOrder.delivery_distance && (
                               <div className="text-[10px] text-muted-foreground">
                                 距离: {(selectedOrder.delivery_distance / 1000).toFixed(1)}km
                               </div>
                             )}
                          </div>
                       </section>

                       <section className="space-y-3">
                          <h4 className="text-[10px] font-bold text-slate-400 uppercase tracking-wider flex items-center gap-1.5">
                            <RefreshCw className="h-3 w-3" /> 支付信息
                          </h4>
                          <div className="space-y-1.5">
                             <div className="text-sm font-bold flex items-baseline gap-1">
                               <span className="text-xs font-normal">实付 ¥</span>
                               {formatAmount(selectedOrder.total_amount)}
                             </div>
                             <div className="text-[10px] text-muted-foreground">
                               {selectedOrder.payment_method === 'wechat' ? '微信支付' : '余额支付'}
                             </div>
                             {selectedOrder.paid_at && (
                                <div className="text-[10px] text-muted-foreground">
                                  {new Date(selectedOrder.paid_at).toLocaleString()}
                                </div>
                             )}
                          </div>
                       </section>
                    </div>

                    <Separator />

                    {/* Order Items */}
                    <section className="space-y-4">
                      <h4 className="text-sm font-semibold border-l-4 border-primary pl-3">商品清单</h4>
                      <div className="rounded-xl border border-slate-100 overflow-hidden shadow-[0_2px_8px_-2px_rgba(0,0,0,0.05)]">
                        <table className="w-full text-sm">
                          <thead className="bg-slate-50 text-[10px] text-slate-500 uppercase font-bold">
                            <tr>
                              <th className="px-4 py-2 text-left font-semibold">商品</th>
                              <th className="px-4 py-2 text-center font-semibold">数量</th>
                              <th className="px-4 py-2 text-right font-semibold">小计</th>
                            </tr>
                          </thead>
                          <tbody className="divide-y divide-slate-50">
                            {selectedOrder.items.map((item) => (
                              <tr key={item.id} className="hover:bg-slate-50/50 transition-colors">
                                <td className="px-4 py-3">
                                  <div className="flex items-center gap-3">
                                    <div className="h-10 w-10 rounded bg-slate-100 shrink-0 overflow-hidden border border-slate-200">
                                      {item.image_url ? (
                                        <img src={item.image_url} alt={item.name} className="h-full w-full object-cover" />
                                      ) : (
                                        <div className="h-full w-full flex items-center justify-center text-slate-400">
                                          <ShoppingBag className="h-5 w-5 opacity-20" />
                                        </div>
                                      )}
                                    </div>
                                    <div>
                                      <div className="font-bold text-slate-800">{item.name}</div>
                                      {item.customizations && item.customizations.length > 0 && (
                                        <div className="text-[10px] text-muted-foreground">
                                          {item.customizations.map(c => `${c.name}:${c.value}`).join(" | ")}
                                        </div>
                                      )}
                                      <div className="text-[10px] text-slate-400">¥{formatAmount(item.unit_price)} / 件</div>
                                    </div>
                                  </div>
                                </td>
                                <td className="px-4 py-3 text-center font-medium text-slate-600">x{item.quantity}</td>
                                <td className="px-4 py-3 text-right font-bold text-slate-900">¥{formatAmount(item.subtotal)}</td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                        
                        <div className="bg-slate-50/80 p-4 space-y-2">
                           <div className="flex justify-between text-xs text-muted-foreground">
                             <span>商品小计</span>
                             <span>¥{formatAmount(selectedOrder.subtotal)}</span>
                           </div>
                           {selectedOrder.delivery_fee > 0 && (
                             <div className="flex justify-between text-xs text-muted-foreground">
                               <span>配送费</span>
                               <span>¥{formatAmount(selectedOrder.delivery_fee)}</span>
                             </div>
                           )}
                           {selectedOrder.discount_amount > 0 && (
                             <div className="flex justify-between text-xs text-rose-500">
                               <span>优惠立减</span>
                               <span>- ¥{formatAmount(selectedOrder.discount_amount)}</span>
                             </div>
                           )}
                           <div className="flex justify-between text-base font-bold text-primary pt-2 border-t border-slate-200">
                             <span>实付款 (元)</span>
                             <span>¥{formatAmount(selectedOrder.total_amount)}</span>
                           </div>
                        </div>
                      </div>
                    </section>
                    
                    {/* Notes & Status Log */}
                    <div className="grid grid-cols-2 gap-6 pb-6">
                      {selectedOrder.notes && (
                        <section className="space-y-3">
                           <h4 className="text-[10px] font-bold text-slate-400 uppercase tracking-wider">客户留言</h4>
                           <div className="bg-amber-50/50 border border-amber-100 text-amber-900 p-4 rounded-xl text-xs italic leading-relaxed">
                            "{selectedOrder.notes}"
                           </div>
                        </section>
                      )}
                      
                      <section className="space-y-3">
                         <h4 className="text-[10px] font-bold text-slate-400 uppercase tracking-wider">时间线</h4>
                         <div className="space-y-4 relative before:absolute before:left-2 before:top-2 before:bottom-2 before:w-px before:bg-slate-100">
                           <div className="relative pl-7 flex items-center justify-between">
                              <div className="absolute left-0 h-4 w-4 rounded-full border-2 border-white flex items-center justify-center bg-blue-500 shadow-sm"><CheckCircle2 className="h-2.5 w-2.5 text-white" /></div>
                              <span className="text-[10px] font-medium text-slate-700">订单创建</span>
                              <span className="text-[10px] text-muted-foreground">{new Date(selectedOrder.created_at).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}</span>
                           </div>
                           {selectedOrder.paid_at && (
                             <div className="relative pl-7 flex items-center justify-between">
                                <div className="absolute left-0 h-4 w-4 rounded-full border-2 border-white flex items-center justify-center bg-blue-500 shadow-sm"><CheckCircle2 className="h-2.5 w-2.5 text-white" /></div>
                                <span className="text-[10px] font-medium text-slate-700">买家已支付</span>
                                <span className="text-[10px] text-muted-foreground">{new Date(selectedOrder.paid_at).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}</span>
                             </div>
                           )}
                           {(selectedOrder.status === 'completed' || selectedOrder.completed_at) && (
                             <div className="relative pl-7 flex items-center justify-between">
                                <div className="absolute left-0 h-4 w-4 rounded-full border-2 border-white flex items-center justify-center bg-emerald-500 shadow-sm"><CheckCircle2 className="h-2.5 w-2.5 text-white" /></div>
                                <span className="text-[10px] font-medium text-slate-700">交易完成</span>
                                <span className="text-[10px] text-muted-foreground">{selectedOrder.completed_at ? new Date(selectedOrder.completed_at).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) : '--'}</span>
                             </div>
                           )}
                         </div>
                      </section>
                    </div>
                  </div>
                </ScrollArea>
              </>
            ) : (
              <div className="flex-1 flex flex-col items-center justify-center space-y-4 opacity-40">
                <div className="h-16 w-16 bg-slate-100 rounded-full flex items-center justify-center">
                  <ShoppingBag className="h-8 w-8 text-slate-400" />
                </div>
                <div className="text-center">
                  <p className="text-sm font-medium">请从左侧选择一个订单</p>
                  <p className="text-xs text-muted-foreground">点击预览订单详情并进行流转操作</p>
                </div>
              </div>
            )}
          </div>
        </div>
      </PageContent>

      {/* Rejection Reason Dialog */}
      <ConfirmDialog 
        open={rejectDialogOpen}
        onOpenChange={setRejectDialogOpen}
        title="拒绝订单"
        description={<div className="space-y-4 mt-2">
          <p className="text-sm">拒绝订单会导致退款给用户，请输入拒绝理由告知用户（如：食材告罄、休息中等）。</p>
          <Input 
            placeholder="请输入拒绝理由..." 
            value={rejectReason}
            onChange={(e) => setRejectReason(e.target.value)}
          />
        </div>}
        confirmText="确认拒单"
        variant="destructive"
        onConfirm={confirmReject}
      />

      {/* Batch Accept Confirm */}
      <ConfirmDialog 
        open={batchAcceptOpen}
        onOpenChange={setBatchAcceptOpen}
        title="批量处理订单"
        description={`确定要批量接单 ${orders.filter(o => selectedIds.has(o.id) && o.status === "paid").length} 个订单吗？接单后将开始制作流程。`}
        confirmText="确认接单"
        onConfirm={batchAccept}
      />

    </PageShell>
  );
}
