"use client";

import { useState, useEffect, useMemo, useCallback } from "react";
import {
  Wallet,
  Receipt,
  Clock,
  RefreshCw,
  CalendarDays,
  FileText,
  CircleDollarSign,
  ArrowRightLeft,
  BadgePercent,
  ChevronLeft,
  ChevronRight,
} from "lucide-react";
import { toast } from "sonner";
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  PageShell,
  PageHeader,
  PageContent,
} from "@/components/merchant/layout/page-shell";
import { apiGet, formatAmount, getRecentRange } from "@/lib/api";
import { cn } from "@/lib/utils";
import type {
  FinanceOverviewResponse,
  FinanceOrderItem,
  FinanceOrdersResponse,
  ServiceFeeItem,
  ServiceFeesResponse,
  PromotionExpenseItem,
  PromotionExpensesResponse,
  DailyFinanceItem,
  DailyFinanceResponse,
  SettlementItem,
  SettlementsResponse,
} from "@/types/finance";

// 日期范围选项
const RANGE_OPTIONS = [
  { value: "week", label: "近7天", days: 7 },
  { value: "month", label: "近30天", days: 30 },
  { value: "quarter", label: "近90天", days: 90 },
];

// 结算状态
const SETTLEMENT_STATUS_LABELS: Record<string, string> = {
  pending: "待处理",
  processing: "处理中",
  finished: "已完成",
  failed: "失败",
};

const SETTLEMENT_STATUS_COLORS: Record<string, string> = {
  pending: "bg-amber-100 text-amber-700 border-amber-200",
  processing: "bg-blue-100 text-blue-700 border-blue-200",
  finished: "bg-emerald-100 text-emerald-700 border-emerald-200",
  failed: "bg-rose-100 text-rose-700 border-rose-200",
};

// 订单来源
const ORDER_SOURCE_LABELS: Record<string, string> = {
  takeout: "外卖",
  dine_in: "堂食",
  pickup: "自取",
  reservation: "预订",
  order: "订单",
};

// 骨架屏
function StatCardSkeleton() {
  return (
    <Card>
      <CardContent className="p-5">
        <Skeleton className="h-4 w-20 mb-3" />
        <Skeleton className="h-8 w-28 mb-2" />
        <Skeleton className="h-3 w-16" />
      </CardContent>
    </Card>
  );
}

function TableSkeleton({ rows = 5, cols = 5 }: { rows?: number; cols?: number }) {
  return (
    <>
      {Array(rows)
        .fill(0)
        .map((_, i) => (
          <TableRow key={i}>
            {Array(cols)
              .fill(0)
              .map((_, j) => (
                <TableCell key={j}>
                  <Skeleton className="h-4 w-16" />
                </TableCell>
              ))}
          </TableRow>
        ))}
    </>
  );
}

export function FinancePageClient() {
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [dateRange, setDateRange] = useState("month");
  const [activeTab, setActiveTab] = useState("overview");

  // 概览数据
  const [overview, setOverview] = useState<FinanceOverviewResponse | null>(null);

  // 每日汇总
  const [dailyStats, setDailyStats] = useState<DailyFinanceItem[]>([]);

  // 订单明细
  const [orders, setOrders] = useState<FinanceOrderItem[]>([]);
  const [ordersPage, setOrdersPage] = useState(1);
  const [ordersTotalPages, setOrdersTotalPages] = useState(1);

  // 服务费明细
  const [serviceFees, setServiceFees] = useState<ServiceFeeItem[]>([]);
  const [serviceFeeSummary, setServiceFeeSummary] = useState({
    total_platform_fee: 0,
    total_operator_fee: 0,
    total_payment_fee: 0,
    total_service_fee: 0,
    total_deduction_fee: 0,
  });

  // 满返支出
  const [promotions, setPromotions] = useState<PromotionExpenseItem[]>([]);
  const [promotionsPage, setPromotionsPage] = useState(1);
  const [promotionsTotalPages, setPromotionsTotalPages] = useState(1);
  const [promotionsSummary, setPromotionsSummary] = useState({
    total_promo_orders: 0,
    total_promo_amount: 0,
  });

  // 结算记录
  const [settlements, setSettlements] = useState<SettlementItem[]>([]);
  const [settlementsPage, setSettlementsPage] = useState(1);
  const [settlementsTotalPages, setSettlementsTotalPages] = useState(1);
  const [settlementStatus, setSettlementStatus] = useState<string>("all");
  const [settlementsSummary, setSettlementsSummary] = useState({
    total_amount: 0,
    total_merchant_amount: 0,
    total_platform_fee: 0,
    total_operator_fee: 0,
  });

  // 日期范围
  const { start_date, end_date } = useMemo(() => {
    const days = RANGE_OPTIONS.find((o) => o.value === dateRange)?.days || 30;
    return getRecentRange(days);
  }, [dateRange]);

  // 加载概览
  const loadOverview = useCallback(async () => {
    try {
      const data = await apiGet<FinanceOverviewResponse>(
        "/merchant/finance/overview",
        { start_date, end_date }
      );
      setOverview(data);
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "加载财务概览失败";
      toast.error(message);
    }
  }, [start_date, end_date]);

  // 加载每日汇总
  const loadDailyStats = useCallback(async () => {
    try {
      const data = await apiGet<DailyFinanceResponse>(
        "/merchant/finance/daily",
        { start_date, end_date }
      );
      setDailyStats(data.daily_stats || []);
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "加载每日汇总失败";
      toast.error(message);
    }
  }, [start_date, end_date]);

  // 加载订单明细
  const loadOrders = useCallback(async (page: number = 1) => {
    try {
      const data = await apiGet<FinanceOrdersResponse>(
        "/merchant/finance/orders",
        { start_date, end_date, page, limit: 10 }
      );
      setOrders(data.orders || []);
      setOrdersTotalPages(data.total_pages || 1);
      setOrdersPage(page);
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "加载订单明细失败";
      toast.error(message);
    }
  }, [start_date, end_date]);

  // 加载服务费
  const loadServiceFees = useCallback(async () => {
    try {
      const data = await apiGet<ServiceFeesResponse>(
        "/merchant/finance/service-fees",
        { start_date, end_date }
      );
      setServiceFees(data.details || []);
      setServiceFeeSummary({
        total_platform_fee: data.total_platform_fee || 0,
        total_operator_fee: data.total_operator_fee || 0,
        total_payment_fee: data.total_payment_fee || 0,
        total_service_fee: data.total_service_fee || 0,
        total_deduction_fee: data.total_deduction_fee || 0,
      });
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "加载服务费明细失败";
      toast.error(message);
    }
  }, [start_date, end_date]);

  // 加载满返支出
  const loadPromotions = useCallback(async (page: number = 1) => {
    try {
      const data = await apiGet<PromotionExpensesResponse>(
        "/merchant/finance/promotions",
        { start_date, end_date, page, limit: 10 }
      );
      setPromotions(data.orders || []);
      setPromotionsTotalPages(data.total_pages || 1);
      setPromotionsPage(page);
      setPromotionsSummary({
        total_promo_orders: data.total_promo_orders || 0,
        total_promo_amount: data.total_promo_amount || 0,
      });
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "加载满返支出失败";
      toast.error(message);
    }
  }, [start_date, end_date]);

  // 加载结算记录
  const loadSettlements = useCallback(async (page: number = 1, status?: string) => {
    try {
      const params: Record<string, string | number> = { start_date, end_date, page, limit: 10 };
      if (status && status !== "all") {
        params.status = status;
      }
      const data = await apiGet<SettlementsResponse>(
        "/merchant/finance/settlements",
        params
      );
      setSettlements(data.settlements || []);
      setSettlementsTotalPages(data.total_pages || 1);
      setSettlementsPage(page);
      setSettlementsSummary({
        total_amount: data.total_amount || 0,
        total_merchant_amount: data.total_merchant_amount || 0,
        total_platform_fee: data.total_platform_fee || 0,
        total_operator_fee: data.total_operator_fee || 0,
      });
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "加载结算记录失败";
      toast.error(message);
    }
  }, [start_date, end_date]);

  // 加载当前Tab数据
  const loadTabData = useCallback(async (tab: string) => {
    setLoading(true);
    try {
      switch (tab) {
        case "overview":
          await loadOverview();
          break;
        case "daily":
          await loadDailyStats();
          break;
        case "orders":
          await loadOrders(1);
          break;
        case "fees":
          await loadServiceFees();
          break;
        case "promotions":
          await loadPromotions(1);
          break;
        case "settlements":
          await loadSettlements(1, settlementStatus);
          break;
      }
    } finally {
      setLoading(false);
    }
  }, [loadOverview, loadDailyStats, loadOrders, loadServiceFees, loadPromotions, loadSettlements, settlementStatus]);

  // 刷新数据
  const handleRefresh = async () => {
    setRefreshing(true);
    await loadTabData(activeTab);
    setRefreshing(false);
    toast.success("数据已刷新");
  };

  // 初始加载
  useEffect(() => {
    loadTabData(activeTab);
  }, [dateRange, activeTab, loadTabData]);

  // Tab 切换
  const handleTabChange = (tab: string) => {
    setActiveTab(tab);
    loadTabData(tab);
  };

  // 概览统计卡片
  const overviewCards = useMemo(() => {
    if (!overview) return [];
    return [
      {
        title: "总交易额 (GMV)",
        value: overview.total_gmv,
        icon: Receipt,
        color: "text-slate-600",
        bgColor: "bg-slate-50",
      },
      {
        title: "商户净收入",
        value: overview.net_income,
        icon: Wallet,
        color: "text-emerald-600",
        bgColor: "bg-emerald-50",
        highlight: true,
      },
      {
        title: "待结算金额",
        value: overview.pending_income,
        icon: Clock,
        color: "text-amber-600",
        bgColor: "bg-amber-50",
      },
      {
        title: "支付手续费",
        value: overview.total_payment_fee,
        icon: BadgePercent,
        color: "text-orange-600",
        bgColor: "bg-orange-50",
      },
      {
        title: "总服务费",
        value: overview.total_service_fee,
        icon: CircleDollarSign,
        color: "text-blue-600",
        bgColor: "bg-blue-50",
      },
    ];
  }, [overview]);

  return (
    <PageShell>
      <PageHeader
        title="财务管理"
        description="查看财务明细、服务费和结算记录"
        actions={
          <div className="flex items-center gap-3">
            {/* 日期范围选择 */}
            <div className="hidden md:flex items-center bg-muted/50 rounded-lg p-1 border">
              {RANGE_OPTIONS.map((opt) => (
                <button
                  key={opt.value}
                  onClick={() => setDateRange(opt.value)}
                  className={cn(
                    "px-4 py-1.5 text-sm font-medium rounded-md transition-all",
                    dateRange === opt.value
                      ? "bg-white shadow-sm text-slate-900"
                      : "text-muted-foreground hover:text-slate-900"
                  )}
                >
                  {opt.label}
                </button>
              ))}
            </div>

            <Button
              size="icon"
              variant="outline"
              onClick={handleRefresh}
              disabled={refreshing}
            >
              <RefreshCw
                className={cn("h-4 w-4", refreshing && "animate-spin")}
              />
            </Button>
          </div>
        }
      />

      <PageContent className="space-y-6">
        {/* 标签页 */}
        <Tabs value={activeTab} onValueChange={handleTabChange}>
          <TabsList className="grid w-full grid-cols-3 lg:grid-cols-6 lg:w-auto">
            <TabsTrigger value="overview">概览</TabsTrigger>
            <TabsTrigger value="daily">日报</TabsTrigger>
            <TabsTrigger value="orders">收入明细</TabsTrigger>
            <TabsTrigger value="fees">服务费</TabsTrigger>
            <TabsTrigger value="promotions">满返支出</TabsTrigger>
            <TabsTrigger value="settlements">结算记录</TabsTrigger>
          </TabsList>

          {/* 概览 */}
          <TabsContent value="overview" className="space-y-6 mt-6">
            {/* 统计卡片 */}
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
              {loading
                ? Array(5).fill(0).map((_, i) => <StatCardSkeleton key={i} />)
                : overviewCards.map((card, i) => (
                    <Card
                      key={i}
                      className={cn(
                        "bg-white rounded-xl border shadow-sm",
                        card.highlight && "ring-2 ring-emerald-500/20"
                      )}
                    >
                      <CardContent className="p-5">
                        <div className="flex items-center gap-3 mb-3">
                          <div className={cn("p-2 rounded-lg", card.bgColor)}>
                            <card.icon className={cn("h-4 w-4", card.color)} />
                          </div>
                        </div>
                        <p className="text-xs text-muted-foreground mb-1">
                          {card.title}
                        </p>
                        <p className={cn(
                          "text-2xl font-bold",
                          card.highlight ? "text-emerald-600" : "text-slate-900"
                        )}>
                          ¥{formatAmount(card.value)}
                        </p>
                      </CardContent>
                    </Card>
                  ))}
            </div>

            {/* 详细信息 */}
            {!loading && overview && (
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                {/* 订单统计 */}
                <Card className="bg-white rounded-xl border shadow-sm">
                  <CardHeader className="pb-2">
                    <CardTitle className="text-base font-semibold flex items-center gap-2">
                      <FileText className="h-4 w-4 text-slate-600" />
                      订单统计
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-3">
                    <div className="flex justify-between items-center py-2 border-b">
                      <span className="text-sm text-muted-foreground">已完成订单</span>
                      <span className="font-medium">{overview.completed_orders} 单</span>
                    </div>
                    <div className="flex justify-between items-center py-2 border-b">
                      <span className="text-sm text-muted-foreground">待结算订单</span>
                      <span className="font-medium">{overview.pending_orders} 单</span>
                    </div>
                    <div className="flex justify-between items-center py-2 border-b">
                      <span className="text-sm text-muted-foreground">满返订单</span>
                      <span className="font-medium">{overview.promotion_orders} 单</span>
                    </div>
                  </CardContent>
                </Card>

                {/* 费用明细 */}
                <Card className="bg-white rounded-xl border shadow-sm">
                  <CardHeader className="pb-2">
                    <CardTitle className="text-base font-semibold flex items-center gap-2">
                      <CircleDollarSign className="h-4 w-4 text-blue-600" />
                      费用明细
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-3">
                    <div className="flex justify-between items-center py-2 border-b">
                      <span className="text-sm text-muted-foreground">商户收入</span>
                      <span className="font-medium text-emerald-600">¥{formatAmount(overview.total_income)}</span>
                    </div>
                    <div className="flex justify-between items-center py-2 border-b">
                      <span className="text-sm text-muted-foreground">平台服务费</span>
                      <span className="font-medium text-rose-600">-¥{formatAmount(overview.total_platform_fee)}</span>
                    </div>
                    <div className="flex justify-between items-center py-2 border-b">
                      <span className="text-sm text-muted-foreground">运营商服务费</span>
                      <span className="font-medium text-rose-600">-¥{formatAmount(overview.total_operator_fee)}</span>
                    </div>
                    <div className="flex justify-between items-center py-2 border-b">
                      <span className="text-sm text-muted-foreground">支付手续费</span>
                      <span className="font-medium text-rose-600">-¥{formatAmount(overview.total_payment_fee)}</span>
                    </div>
                    <div className="flex justify-between items-center py-2 border-b">
                      <span className="text-sm text-muted-foreground">满返支出</span>
                      <span className="font-medium text-rose-600">-¥{formatAmount(overview.total_promotion_exp)}</span>
                    </div>
                    <div className="flex justify-between items-center py-2 bg-slate-50 -mx-6 px-6 rounded-b-xl">
                      <span className="text-sm font-medium">净收入</span>
                      <span className="font-bold text-lg text-emerald-600">¥{formatAmount(overview.net_income)}</span>
                    </div>
                  </CardContent>
                </Card>
              </div>
            )}
          </TabsContent>

          {/* 每日汇总 */}
          <TabsContent value="daily" className="space-y-6 mt-6">
            <Card className="bg-white rounded-xl border shadow-sm">
              <CardHeader className="pb-2">
                <CardTitle className="text-base font-semibold flex items-center gap-2">
                  <CalendarDays className="h-4 w-4 text-slate-600" />
                  每日财务汇总
                </CardTitle>
                <CardDescription>{start_date} 至 {end_date}</CardDescription>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>日期</TableHead>
                      <TableHead className="text-right">订单数</TableHead>
                      <TableHead className="text-right">交易额</TableHead>
                      <TableHead className="text-right">商户收入</TableHead>
                      <TableHead className="text-right">服务费</TableHead>
                      <TableHead className="text-right">支付手续费</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {loading ? (
                      <TableSkeleton rows={7} cols={6} />
                    ) : dailyStats.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={6} className="text-center py-10 text-muted-foreground">
                          暂无数据
                        </TableCell>
                      </TableRow>
                    ) : (
                      dailyStats.map((item) => (
                        <TableRow key={item.date}>
                          <TableCell className="font-medium">{item.date}</TableCell>
                          <TableCell className="text-right">{item.order_count}</TableCell>
                          <TableCell className="text-right">¥{formatAmount(item.total_gmv)}</TableCell>
                          <TableCell className="text-right text-emerald-600 font-medium">
                            ¥{formatAmount(item.merchant_income)}
                          </TableCell>
                          <TableCell className="text-right text-muted-foreground">
                            ¥{formatAmount(item.total_fee)}
                          </TableCell>
                          <TableCell className="text-right text-orange-600">
                            ¥{formatAmount(item.payment_fee)}
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </TabsContent>

          {/* 订单收入明细 */}
          <TabsContent value="orders" className="space-y-6 mt-6">
            <Card className="bg-white rounded-xl border shadow-sm">
              <CardHeader className="pb-2">
                <CardTitle className="text-base font-semibold flex items-center gap-2">
                  <Receipt className="h-4 w-4 text-slate-600" />
                  订单收入明细
                </CardTitle>
                <CardDescription>每笔订单的分账详情</CardDescription>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>订单号</TableHead>
                      <TableHead>来源</TableHead>
                      <TableHead className="text-right">订单金额</TableHead>
                      <TableHead className="text-right">服务费</TableHead>
                      <TableHead className="text-right">支付手续费</TableHead>
                      <TableHead className="text-right">到账金额</TableHead>
                      <TableHead>状态</TableHead>
                      <TableHead className="text-right">时间</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {loading ? (
                      <TableSkeleton rows={5} cols={8} />
                    ) : orders.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={8} className="text-center py-10 text-muted-foreground">
                          暂无数据
                        </TableCell>
                      </TableRow>
                    ) : (
                      orders.map((item) => (
                        <TableRow key={item.id}>
                          <TableCell className="font-mono text-xs">
                            {item.payment_order_id}
                          </TableCell>
                          <TableCell>
                            <Badge variant="secondary" className="text-xs">
                              {ORDER_SOURCE_LABELS[item.order_source] || item.order_source}
                            </Badge>
                          </TableCell>
                          <TableCell className="text-right">
                            ¥{formatAmount(item.total_amount)}
                          </TableCell>
                          <TableCell className="text-right text-muted-foreground">
                            ¥{formatAmount(item.platform_commission + item.operator_commission)}
                          </TableCell>
                          <TableCell className="text-right text-orange-600">
                            ¥{formatAmount(item.payment_fee)}
                          </TableCell>
                          <TableCell className="text-right text-emerald-600 font-medium">
                            ¥{formatAmount(item.merchant_amount)}
                          </TableCell>
                          <TableCell>
                            <Badge
                              variant="outline"
                              className={cn("text-xs", SETTLEMENT_STATUS_COLORS[item.status])}
                            >
                              {SETTLEMENT_STATUS_LABELS[item.status] || item.status}
                            </Badge>
                          </TableCell>
                          <TableCell className="text-right text-xs text-muted-foreground">
                            {item.created_at ? new Date(item.created_at).toLocaleDateString() : "-"}
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>

                {/* 分页 */}
                {ordersTotalPages > 1 && (
                  <div className="flex items-center justify-end gap-2 mt-4">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => loadOrders(ordersPage - 1)}
                      disabled={ordersPage <= 1 || loading}
                    >
                      <ChevronLeft className="h-4 w-4" />
                    </Button>
                    <span className="text-sm text-muted-foreground">
                      {ordersPage} / {ordersTotalPages}
                    </span>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => loadOrders(ordersPage + 1)}
                      disabled={ordersPage >= ordersTotalPages || loading}
                    >
                      <ChevronRight className="h-4 w-4" />
                    </Button>
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          {/* 服务费明细 */}
          <TabsContent value="fees" className="space-y-6 mt-6">
            {/* 服务费汇总 */}
            {!loading && (
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                <Card className="bg-white rounded-xl border shadow-sm">
                  <CardContent className="p-5">
                    <p className="text-xs text-muted-foreground mb-1">平台服务费</p>
                    <p className="text-xl font-bold text-blue-600">
                      ¥{formatAmount(serviceFeeSummary.total_platform_fee)}
                    </p>
                  </CardContent>
                </Card>
                <Card className="bg-white rounded-xl border shadow-sm">
                  <CardContent className="p-5">
                    <p className="text-xs text-muted-foreground mb-1">运营商服务费</p>
                    <p className="text-xl font-bold text-purple-600">
                      ¥{formatAmount(serviceFeeSummary.total_operator_fee)}
                    </p>
                  </CardContent>
                </Card>
                <Card className="bg-white rounded-xl border shadow-sm">
                  <CardContent className="p-5">
                    <p className="text-xs text-muted-foreground mb-1">服务费合计</p>
                    <p className="text-xl font-bold text-rose-600">
                      ¥{formatAmount(serviceFeeSummary.total_service_fee)}
                    </p>
                  </CardContent>
                </Card>
                <Card className="bg-white rounded-xl border shadow-sm">
                  <CardContent className="p-5">
                    <p className="text-xs text-muted-foreground mb-1">支付手续费</p>
                    <p className="text-xl font-bold text-orange-600">
                      ¥{formatAmount(serviceFeeSummary.total_payment_fee)}
                    </p>
                  </CardContent>
                </Card>
              </div>
            )}

            <Card className="bg-white rounded-xl border shadow-sm">
              <CardHeader className="pb-2">
                <CardTitle className="text-base font-semibold flex items-center gap-2">
                  <CircleDollarSign className="h-4 w-4 text-blue-600" />
                  服务费明细
                </CardTitle>
                <CardDescription>按日期和来源分组</CardDescription>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>日期</TableHead>
                      <TableHead>来源</TableHead>
                      <TableHead className="text-right">订单数</TableHead>
                      <TableHead className="text-right">订单金额</TableHead>
                      <TableHead className="text-right">平台费</TableHead>
                      <TableHead className="text-right">运营商费</TableHead>
                      <TableHead className="text-right">支付手续费</TableHead>
                      <TableHead className="text-right">扣减合计</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {loading ? (
                      <TableSkeleton rows={5} cols={8} />
                    ) : serviceFees.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={8} className="text-center py-10 text-muted-foreground">
                          暂无数据
                        </TableCell>
                      </TableRow>
                    ) : (
                      serviceFees.map((item, i) => (
                        <TableRow key={i}>
                          <TableCell className="font-medium">{item.date}</TableCell>
                          <TableCell>
                            <Badge variant="secondary" className="text-xs">
                              {ORDER_SOURCE_LABELS[item.order_source] || item.order_source}
                            </Badge>
                          </TableCell>
                          <TableCell className="text-right">{item.order_count}</TableCell>
                          <TableCell className="text-right">¥{formatAmount(item.total_amount)}</TableCell>
                          <TableCell className="text-right text-blue-600">
                            ¥{formatAmount(item.platform_fee)}
                          </TableCell>
                          <TableCell className="text-right text-purple-600">
                            ¥{formatAmount(item.operator_fee)}
                          </TableCell>
                          <TableCell className="text-right text-orange-600">
                            ¥{formatAmount(item.payment_fee)}
                          </TableCell>
                          <TableCell className="text-right font-medium">
                            ¥{formatAmount(item.total_deduction_fee)}
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </TabsContent>

          {/* 满返支出 */}
          <TabsContent value="promotions" className="space-y-6 mt-6">
            {/* 满返汇总 */}
            {!loading && (
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <Card className="bg-white rounded-xl border shadow-sm">
                  <CardContent className="p-5">
                    <p className="text-xs text-muted-foreground mb-1">满返订单数</p>
                    <p className="text-xl font-bold text-slate-900">
                      {promotionsSummary.total_promo_orders} 单
                    </p>
                  </CardContent>
                </Card>
                <Card className="bg-white rounded-xl border shadow-sm">
                  <CardContent className="p-5">
                    <p className="text-xs text-muted-foreground mb-1">满返支出总额</p>
                    <p className="text-xl font-bold text-amber-600">
                      ¥{formatAmount(promotionsSummary.total_promo_amount)}
                    </p>
                  </CardContent>
                </Card>
              </div>
            )}

            <Card className="bg-white rounded-xl border shadow-sm">
              <CardHeader className="pb-2">
                <CardTitle className="text-base font-semibold flex items-center gap-2">
                  <BadgePercent className="h-4 w-4 text-amber-600" />
                  满返支出明细
                </CardTitle>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>订单号</TableHead>
                      <TableHead>类型</TableHead>
                      <TableHead className="text-right">商品金额</TableHead>
                      <TableHead className="text-right">代取费</TableHead>
                      <TableHead className="text-right">代取优惠</TableHead>
                      <TableHead className="text-right">订单金额</TableHead>
                      <TableHead className="text-right">时间</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {loading ? (
                      <TableSkeleton rows={5} cols={7} />
                    ) : promotions.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={7} className="text-center py-10 text-muted-foreground">
                          暂无数据
                        </TableCell>
                      </TableRow>
                    ) : (
                      promotions.map((item) => (
                        <TableRow key={item.id}>
                          <TableCell className="font-mono text-xs">{item.order_no}</TableCell>
                          <TableCell>
                            <Badge variant="secondary" className="text-xs">
                              {ORDER_SOURCE_LABELS[item.order_type] || item.order_type}
                            </Badge>
                          </TableCell>
                          <TableCell className="text-right">¥{formatAmount(item.subtotal)}</TableCell>
                          <TableCell className="text-right">¥{formatAmount(item.delivery_fee)}</TableCell>
                          <TableCell className="text-right text-amber-600">
                            -¥{formatAmount(item.delivery_fee_discount)}
                          </TableCell>
                          <TableCell className="text-right font-medium">
                            ¥{formatAmount(item.total_amount)}
                          </TableCell>
                          <TableCell className="text-right text-xs text-muted-foreground">
                            {item.created_at ? new Date(item.created_at).toLocaleDateString() : "-"}
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>

                {/* 分页 */}
                {promotionsTotalPages > 1 && (
                  <div className="flex items-center justify-end gap-2 mt-4">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => loadPromotions(promotionsPage - 1)}
                      disabled={promotionsPage <= 1 || loading}
                    >
                      <ChevronLeft className="h-4 w-4" />
                    </Button>
                    <span className="text-sm text-muted-foreground">
                      {promotionsPage} / {promotionsTotalPages}
                    </span>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => loadPromotions(promotionsPage + 1)}
                      disabled={promotionsPage >= promotionsTotalPages || loading}
                    >
                      <ChevronRight className="h-4 w-4" />
                    </Button>
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          {/* 结算记录 */}
          <TabsContent value="settlements" className="space-y-6 mt-6">
            {/* 结算汇总 */}
            {!loading && (
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                <Card className="bg-white rounded-xl border shadow-sm">
                  <CardContent className="p-5">
                    <p className="text-xs text-muted-foreground mb-1">总交易额</p>
                    <p className="text-xl font-bold text-slate-900">
                      ¥{formatAmount(settlementsSummary.total_amount)}
                    </p>
                  </CardContent>
                </Card>
                <Card className="bg-white rounded-xl border shadow-sm">
                  <CardContent className="p-5">
                    <p className="text-xs text-muted-foreground mb-1">商户到账</p>
                    <p className="text-xl font-bold text-emerald-600">
                      ¥{formatAmount(settlementsSummary.total_merchant_amount)}
                    </p>
                  </CardContent>
                </Card>
                <Card className="bg-white rounded-xl border shadow-sm">
                  <CardContent className="p-5">
                    <p className="text-xs text-muted-foreground mb-1">平台抽成</p>
                    <p className="text-xl font-bold text-blue-600">
                      ¥{formatAmount(settlementsSummary.total_platform_fee)}
                    </p>
                  </CardContent>
                </Card>
                <Card className="bg-white rounded-xl border shadow-sm">
                  <CardContent className="p-5">
                    <p className="text-xs text-muted-foreground mb-1">运营商抽成</p>
                    <p className="text-xl font-bold text-purple-600">
                      ¥{formatAmount(settlementsSummary.total_operator_fee)}
                    </p>
                  </CardContent>
                </Card>
              </div>
            )}

            <Card className="bg-white rounded-xl border shadow-sm">
              <CardHeader className="pb-2 flex flex-row items-center justify-between">
                <div>
                  <CardTitle className="text-base font-semibold flex items-center gap-2">
                    <ArrowRightLeft className="h-4 w-4 text-slate-600" />
                    结算记录
                  </CardTitle>
                  <CardDescription>分账订单详情</CardDescription>
                </div>
                <Select
                  value={settlementStatus}
                  onValueChange={(v) => {
                    setSettlementStatus(v);
                    loadSettlements(1, v);
                  }}
                >
                  <SelectTrigger className="w-30">
                    <SelectValue placeholder="状态筛选" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">全部状态</SelectItem>
                    <SelectItem value="pending">待处理</SelectItem>
                    <SelectItem value="processing">处理中</SelectItem>
                    <SelectItem value="finished">已完成</SelectItem>
                    <SelectItem value="failed">失败</SelectItem>
                  </SelectContent>
                </Select>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>支付单号</TableHead>
                      <TableHead>来源</TableHead>
                      <TableHead className="text-right">订单金额</TableHead>
                      <TableHead className="text-right">平台佣金</TableHead>
                      <TableHead className="text-right">运营商佣金</TableHead>
                      <TableHead className="text-right">商户到账</TableHead>
                      <TableHead>状态</TableHead>
                      <TableHead className="text-right">时间</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {loading ? (
                      <TableSkeleton rows={5} cols={8} />
                    ) : settlements.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={8} className="text-center py-10 text-muted-foreground">
                          暂无数据
                        </TableCell>
                      </TableRow>
                    ) : (
                      settlements.map((item) => (
                        <TableRow key={item.id}>
                          <TableCell className="font-mono text-xs">
                            {item.payment_order_id}
                          </TableCell>
                          <TableCell>
                            <Badge variant="secondary" className="text-xs">
                              {ORDER_SOURCE_LABELS[item.order_source] || item.order_source}
                            </Badge>
                          </TableCell>
                          <TableCell className="text-right">
                            ¥{formatAmount(item.total_amount)}
                          </TableCell>
                          <TableCell className="text-right text-blue-600">
                            ¥{formatAmount(item.platform_commission)}
                          </TableCell>
                          <TableCell className="text-right text-purple-600">
                            ¥{formatAmount(item.operator_commission)}
                          </TableCell>
                          <TableCell className="text-right text-emerald-600 font-medium">
                            ¥{formatAmount(item.merchant_amount)}
                          </TableCell>
                          <TableCell>
                            <Badge
                              variant="outline"
                              className={cn("text-xs", SETTLEMENT_STATUS_COLORS[item.status])}
                            >
                              {SETTLEMENT_STATUS_LABELS[item.status] || item.status}
                            </Badge>
                          </TableCell>
                          <TableCell className="text-right text-xs text-muted-foreground">
                            {item.created_at ? new Date(item.created_at).toLocaleDateString() : "-"}
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>

                {/* 分页 */}
                {settlementsTotalPages > 1 && (
                  <div className="flex items-center justify-end gap-2 mt-4">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => loadSettlements(settlementsPage - 1, settlementStatus)}
                      disabled={settlementsPage <= 1 || loading}
                    >
                      <ChevronLeft className="h-4 w-4" />
                    </Button>
                    <span className="text-sm text-muted-foreground">
                      {settlementsPage} / {settlementsTotalPages}
                    </span>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => loadSettlements(settlementsPage + 1, settlementStatus)}
                      disabled={settlementsPage >= settlementsTotalPages || loading}
                    >
                      <ChevronRight className="h-4 w-4" />
                    </Button>
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </PageContent>
    </PageShell>
  );
}
