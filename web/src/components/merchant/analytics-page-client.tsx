"use client";

import { useState, useEffect, useMemo, useCallback } from "react";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Area,
  PieChart,
  Pie,
  Cell,
  ComposedChart,
} from "recharts";
import {
  TrendingUp,
  TrendingDown,
  DollarSign,
  ShoppingBag,
  Users,
  RefreshCw,
  ChefHat,
  Clock,
  PieChart as PieChartIcon,
  Utensils,
  Receipt,
  CalendarDays,
  Wallet,
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
import { ScrollArea } from "@/components/ui/scroll-area";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Progress } from "@/components/ui/progress";
import {
  PageShell,
  PageHeader,
  PageContent,
} from "@/components/merchant/layout/page-shell";
import { apiGet, formatAmount, formatDate, getRecentRange } from "@/lib/api";
import { cn } from "@/lib/utils";
import type {
  OverviewResponse,
  DailyStatRow,
  TopDishRow,
  HourlyStatsRow,
  OrderSourceStatsRow,
  RepurchaseRateResponse,
  CategoryStatsRow,
  CustomerStatRow,
  CustomerListResponse,
  FinanceOverviewResponse,
} from "@/types/stats";

// 图表配色
const CHART_COLORS = {
  primary: "hsl(var(--primary))",
  success: "#10b981",
  warning: "#f59e0b",
  danger: "#ec4899",
  info: "#06b6d4",
  purple: "#8b5cf6",
};

const PIE_COLORS = [
  CHART_COLORS.primary,
  CHART_COLORS.success,
  CHART_COLORS.warning,
  CHART_COLORS.purple,
  CHART_COLORS.info,
  CHART_COLORS.danger,
];

// 日期范围选项
const RANGE_OPTIONS = [
  { value: "today", label: "今日", days: 0 },
  { value: "week", label: "近7天", days: 7 },
  { value: "month", label: "近30天", days: 30 },
];

// 订单类型标签
const ORDER_TYPE_MAP: Record<string, string> = {
  takeout: "外卖配送",
  dine_in: "堂食点餐",
  pickup: "到店自取",
  reservation: "预订",
};

// 骨架屏组件
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

function ChartSkeleton({ height = 280 }: { height?: number }) {
  return (
    <div className="w-full" style={{ height }}>
      <Skeleton className="w-full h-full rounded-lg" />
    </div>
  );
}

export function AnalyticsPageClient() {
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [dateRange, setDateRange] = useState("week");
  const [activeTab, setActiveTab] = useState("overview");

  // Data States
  const [overview, setOverview] = useState<OverviewResponse | null>(null);
  const [prevOverview, setPrevOverview] = useState<OverviewResponse | null>(
    null
  );
  const [dailyStats, setDailyStats] = useState<DailyStatRow[]>([]);
  const [topDishes, setTopDishes] = useState<TopDishRow[]>([]);
  const [hourlyStats, setHourlyStats] = useState<HourlyStatsRow[]>([]);
  const [sourceStats, setSourceStats] = useState<OrderSourceStatsRow[]>([]);
  const [repurchaseStats, setRepurchaseStats] =
    useState<RepurchaseRateResponse | null>(null);
  const [categoryStats, setCategoryStats] = useState<CategoryStatsRow[]>([]);
  const [customerStats, setCustomerStats] = useState<CustomerStatRow[]>([]);
  const [financeOverview, setFinanceOverview] =
    useState<FinanceOverviewResponse | null>(null);

  // 计算日期范围
  const { start_date, end_date, prev_start_date, prev_end_date } =
    useMemo(() => {
      const days = RANGE_OPTIONS.find((o) => o.value === dateRange)?.days || 7;
      const current = getRecentRange(days);

      // 计算上一周期
      const prevEnd = new Date(current.start_date);
      prevEnd.setDate(prevEnd.getDate() - 1);
      const prevStart = new Date(prevEnd);
      prevStart.setDate(prevStart.getDate() - (days || 1));

      return {
        start_date: current.start_date,
        end_date: current.end_date,
        prev_start_date: formatDate(prevStart),
        prev_end_date: formatDate(prevEnd),
      };
    }, [dateRange]);

  // 加载所有数据（允许部分接口失败，避免整页不可用）
  const loadAllData = useCallback(async (manualRefresh = false) => {
    if (manualRefresh) {
      setRefreshing(true);
    } else {
      setLoading(true);
    }

    const params = { start_date, end_date };
    const prevParams = { start_date: prev_start_date, end_date: prev_end_date };

    try {
      const [
        overviewRes,
        prevOverviewRes,
        dailyRes,
        dishesRes,
        hourlyRes,
        sourceRes,
        repurchaseRes,
        categoryRes,
        customersRes,
        financeRes,
      ] = await Promise.allSettled([
        apiGet<OverviewResponse>("/merchant/stats/overview", params),
        apiGet<OverviewResponse>("/merchant/stats/overview", prevParams),
        apiGet<DailyStatRow[]>("/merchant/stats/daily", params),
        apiGet<TopDishRow[]>("/merchant/stats/dishes/top", {
          ...params,
          limit: 10,
        }),
        apiGet<HourlyStatsRow[]>("/merchant/stats/hourly", params),
        apiGet<OrderSourceStatsRow[]>("/merchant/stats/sources", params),
        apiGet<RepurchaseRateResponse>("/merchant/stats/repurchase", params),
        apiGet<CategoryStatsRow[]>("/merchant/stats/categories", params),
        apiGet<CustomerListResponse>("/merchant/stats/customers", {
          order_by: "total_amount",
          page: 1,
          limit: 10,
        }),
        apiGet<FinanceOverviewResponse>("/merchant/finance/overview", params),
      ]);

      const failedModules: string[] = [];
      const failedReasons: string[] = [];
      const unwrapSettled = <T,>(
        result: PromiseSettledResult<T>,
        fallback: T,
        moduleName: string
      ): T => {
        if (result.status === "fulfilled") {
          return result.value;
        }
        failedModules.push(moduleName);
        const reason =
          result.reason instanceof Error
            ? result.reason.message
            : String(result.reason || "unknown error");
        failedReasons.push(`${moduleName}: ${reason}`);
        return fallback;
      };

      const emptyCustomerList: CustomerListResponse = {
        data: [],
        total_count: 0,
        total: 0,
        page_id: 1,
        page_size: 10,
        page: 1,
        limit: 10,
      };

      setOverview(unwrapSettled(overviewRes, null, "概览") as OverviewResponse | null);
      setPrevOverview(
        unwrapSettled(prevOverviewRes, null, "上一周期概览") as OverviewResponse | null
      );
      setDailyStats(unwrapSettled(dailyRes, [], "日报"));
      setTopDishes(unwrapSettled(dishesRes, [], "热销菜品"));
      setHourlyStats(unwrapSettled(hourlyRes, [], "时段分析"));
      setSourceStats(unwrapSettled(sourceRes, [], "订单来源"));
      setRepurchaseStats(
        unwrapSettled(repurchaseRes, null, "复购分析") as RepurchaseRateResponse | null
      );
      setCategoryStats(unwrapSettled(categoryRes, [], "分类分析"));
      setCustomerStats(
        unwrapSettled(customersRes, emptyCustomerList, "客户分析").data || []
      );
      setFinanceOverview(
        unwrapSettled(financeRes, null, "财务分析") as FinanceOverviewResponse | null
      );

      if (manualRefresh && failedModules.length === 0) {
        toast.success("数据已刷新");
      }

      if (manualRefresh && failedModules.length > 0) {
        const modulesText = failedModules.join("、");
        const reasonText = failedReasons
          .slice(0, 2)
          .map((item) => item.slice(0, 80))
          .join("；");
        toast.warning(
          reasonText
            ? `部分模块加载失败：${modulesText}（${reasonText}）`
            : `部分模块加载失败：${modulesText}`
        );
      }
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "数据加载失败";
      if (manualRefresh) {
        toast.error(message);
      } else {
        toast.error("经营分析暂时不可用，请稍后重试");
      }
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, [start_date, end_date, prev_start_date, prev_end_date]);

  useEffect(() => {
    void loadAllData();
  }, [loadAllData]);

  // 计算环比增长
  const getGrowth = (current: number = 0, prev: number = 0) => {
    if (!prev) return current > 0 ? 100 : 0;
    return ((current - prev) / prev) * 100;
  };

  // 趋势图数据
  const trendData = useMemo(
    () =>
      dailyStats
        .slice()
        .reverse()
        .map((item) => ({
          date: item.date.slice(5),
          sales: item.total_sales / 100,
          orders: item.order_count,
          commission: item.commission / 100,
        })),
    [dailyStats]
  );

  // 来源饼图数据
  const sourceChartData = useMemo(() => {
    const total = sourceStats.reduce((sum, item) => sum + item.order_count, 0);
    return sourceStats.map((item) => ({
      name: ORDER_TYPE_MAP[item.order_type] || item.order_type,
      value: item.order_count,
      sales: item.total_sales,
      percent: total ? (item.order_count / total) * 100 : 0,
    }));
  }, [sourceStats]);

  // 分类饼图数据
  const categoryChartData = useMemo(() => {
    const total = categoryStats.reduce(
      (sum, item) => sum + item.total_sales,
      0
    );
    return categoryStats.map((item) => ({
      name: item.category_name,
      value: item.total_sales / 100,
      percent: total ? (item.total_sales / total) * 100 : 0,
    }));
  }, [categoryStats]);

  // 时段分析数据
  const hourlyChartData = useMemo(() => {
    const maxOrders = Math.max(...hourlyStats.map((h) => h.order_count), 1);
    return hourlyStats.map((item) => ({
      ...item,
      intensity: item.order_count / maxOrders,
    }));
  }, [hourlyStats]);

  // 概览统计卡片数据
  const statCards = useMemo(() => {
    const salesGrowth = getGrowth(
      overview?.total_sales,
      prevOverview?.total_sales
    );
    const ordersGrowth = getGrowth(
      overview?.total_orders,
      prevOverview?.total_orders
    );

    return [
      {
        title: "总营业额",
        value: `¥${formatAmount(overview?.total_sales)}`,
        subValue: `日均 ¥${formatAmount(overview?.avg_daily_sales)}`,
        icon: DollarSign,
        trend: salesGrowth,
        color: "text-primary",
        bgColor: "bg-primary/10",
      },
      {
        title: "有效订单",
        value: overview?.total_orders || 0,
        subValue: `${overview?.total_days || 0} 天统计`,
        icon: ShoppingBag,
        trend: ordersGrowth,
        color: "text-emerald-600",
        bgColor: "bg-emerald-50",
      },
      {
        title: "客户总数",
        value: repurchaseStats?.total_users || 0,
        subValue: `复购 ${repurchaseStats?.repeat_users || 0} 人`,
        icon: Users,
        color: "text-blue-600",
        bgColor: "bg-blue-50",
      },
      {
        title: "复购率",
        value: `${(repurchaseStats?.repurchase_rate || 0).toFixed(1)}%`,
        subValue: `人均 ${(repurchaseStats?.avg_orders_per_user || 0).toFixed(1)} 单`,
        icon: RefreshCw,
        color: "text-amber-600",
        bgColor: "bg-amber-50",
      },
    ];
  }, [overview, prevOverview, repurchaseStats]);

  return (
    <PageShell>
      <PageHeader
        title="经营分析"
        description="全方位经营数据透视，助力商业决策"
        actions={
          <div className="flex items-center gap-3">
            {/* 日期范围选择器 */}
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
              onClick={() => loadAllData(true)}
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
        {/* 统计卡片网格 */}
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          {loading
            ? Array(4)
                .fill(0)
                .map((_, i) => <StatCardSkeleton key={i} />)
            : statCards.map((card, i) => (
                <Card
                  key={i}
                  className="bg-white rounded-xl border shadow-sm hover:shadow-md transition-shadow"
                >
                  <CardContent className="p-5">
                    <div className="flex items-center justify-between mb-3">
                      <div className={cn("p-2 rounded-lg", card.bgColor)}>
                        <card.icon className={cn("h-4 w-4", card.color)} />
                      </div>
                      {card.trend !== undefined && (
                        <Badge
                          variant={card.trend >= 0 ? "default" : "destructive"}
                          className="text-xs h-5 px-1.5"
                        >
                          {card.trend >= 0 ? (
                            <TrendingUp className="h-3 w-3 mr-0.5" />
                          ) : (
                            <TrendingDown className="h-3 w-3 mr-0.5" />
                          )}
                          {Math.abs(card.trend).toFixed(1)}%
                        </Badge>
                      )}
                    </div>
                    <p className="text-xs text-muted-foreground font-medium mb-1">
                      {card.title}
                    </p>
                    <p className="text-2xl font-bold text-slate-900">
                      {card.value}
                    </p>
                    <p className="text-xs text-muted-foreground mt-1">
                      {card.subValue}
                    </p>
                  </CardContent>
                </Card>
              ))}
        </div>

        {/* 标签页 */}
        <Tabs value={activeTab} onValueChange={setActiveTab}>
          <TabsList className="grid w-full grid-cols-4 lg:w-auto lg:inline-grid">
            <TabsTrigger value="overview">概览趋势</TabsTrigger>
            <TabsTrigger value="sales">销售分析</TabsTrigger>
            <TabsTrigger value="finance">财务分析</TabsTrigger>
            <TabsTrigger value="customer">客户分析</TabsTrigger>
          </TabsList>

          {/* 概览趋势 */}
          <TabsContent value="overview" className="space-y-6 mt-6">
            <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
              {/* 销售趋势图 */}
              <Card className="lg:col-span-2 bg-white rounded-xl border shadow-sm">
                <CardHeader className="pb-2">
                  <CardTitle className="text-base font-semibold flex items-center gap-2">
                    <TrendingUp className="h-4 w-4 text-primary" />
                    销售趋势
                  </CardTitle>
                  <CardDescription>
                    {start_date} 至 {end_date}
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  {loading ? (
                    <ChartSkeleton />
                  ) : (
                    <div className="h-70 w-full">
                      <ResponsiveContainer width="100%" height="100%">
                        <ComposedChart data={trendData}>
                          <defs>
                            <linearGradient
                              id="colorSales"
                              x1="0"
                              y1="0"
                              x2="0"
                              y2="1"
                            >
                              <stop
                                offset="5%"
                                stopColor="hsl(var(--primary))"
                                stopOpacity={0.1}
                              />
                              <stop
                                offset="95%"
                                stopColor="hsl(var(--primary))"
                                stopOpacity={0}
                              />
                            </linearGradient>
                          </defs>
                          <CartesianGrid
                            strokeDasharray="3 3"
                            vertical={false}
                            stroke="#f1f5f9"
                          />
                          <XAxis
                            dataKey="date"
                            axisLine={false}
                            tickLine={false}
                            tick={{ fill: "#94a3b8", fontSize: 12 }}
                          />
                          <YAxis
                            yAxisId="left"
                            axisLine={false}
                            tickLine={false}
                            tick={{ fill: "#94a3b8", fontSize: 12 }}
                            tickFormatter={(v) => `¥${v}`}
                          />
                          <YAxis
                            yAxisId="right"
                            orientation="right"
                            axisLine={false}
                            tickLine={false}
                            tick={{ fill: "#94a3b8", fontSize: 12 }}
                          />
                          <Tooltip
                            contentStyle={{
                              borderRadius: "12px",
                              border: "none",
                              boxShadow: "0 4px 12px rgb(0 0 0 / 0.1)",
                            }}
                            formatter={(value?: number | string, name?: string) => [
                              name === "sales" ? `¥${value ?? 0}` : `${value ?? 0}`,
                              name === "sales" ? "销售额" : "订单量",
                            ]}
                          />
                          <Bar
                            yAxisId="right"
                            dataKey="orders"
                            fill="#e2e8f0"
                            radius={[4, 4, 4, 4]}
                            barSize={20}
                          />
                          <Area
                            yAxisId="left"
                            type="monotone"
                            dataKey="sales"
                            stroke="hsl(var(--primary))"
                            strokeWidth={3}
                            fill="url(#colorSales)"
                          />
                        </ComposedChart>
                      </ResponsiveContainer>
                    </div>
                  )}
                </CardContent>
              </Card>

              {/* 订单来源分析 */}
              <Card className="bg-white rounded-xl border shadow-sm">
                <CardHeader className="pb-2">
                  <CardTitle className="text-base font-semibold flex items-center gap-2">
                    <PieChartIcon className="h-4 w-4 text-blue-600" />
                    订单来源
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  {loading ? (
                    <ChartSkeleton height={200} />
                  ) : (
                    <>
                      <div className="h-40 relative">
                        <ResponsiveContainer width="100%" height="100%">
                          <PieChart>
                            <Pie
                              data={sourceChartData}
                              cx="50%"
                              cy="50%"
                              innerRadius={50}
                              outerRadius={70}
                              paddingAngle={4}
                              dataKey="value"
                            >
                              {sourceChartData.map((_, index) => (
                                <Cell
                                  key={`cell-${index}`}
                                  fill={PIE_COLORS[index % PIE_COLORS.length]}
                                  strokeWidth={0}
                                />
                              ))}
                            </Pie>
                            <Tooltip
                              contentStyle={{ borderRadius: "8px" }}
                              formatter={(value) => [`${value ?? 0} 单`, "订单"]}
                            />
                          </PieChart>
                        </ResponsiveContainer>
                      </div>
                      <div className="space-y-2 mt-4">
                        {sourceChartData.map((item, i) => (
                          <div
                            key={i}
                            className="flex items-center justify-between text-sm"
                          >
                            <div className="flex items-center gap-2">
                              <div
                                className="w-3 h-3 rounded-full"
                                style={{
                                  backgroundColor:
                                    PIE_COLORS[i % PIE_COLORS.length],
                                }}
                              />
                              <span className="text-muted-foreground">
                                {item.name}
                              </span>
                            </div>
                            <span className="font-medium">
                              {item.percent.toFixed(0)}%
                            </span>
                          </div>
                        ))}
                      </div>
                    </>
                  )}
                </CardContent>
              </Card>
            </div>

            {/* 每日数据表格 */}
            <Card className="bg-white rounded-xl border shadow-sm">
              <CardHeader className="pb-2">
                <CardTitle className="text-base font-semibold flex items-center gap-2">
                  <CalendarDays className="h-4 w-4 text-slate-600" />
                  每日明细
                </CardTitle>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>日期</TableHead>
                      <TableHead className="text-right">订单数</TableHead>
                      <TableHead className="text-right">销售额</TableHead>
                      <TableHead className="text-right">佣金</TableHead>
                      <TableHead className="text-right">堂食</TableHead>
                      <TableHead className="text-right">外卖</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {loading
                      ? Array(5)
                          .fill(0)
                          .map((_, i) => (
                            <TableRow key={i}>
                              {Array(6)
                                .fill(0)
                                .map((_, j) => (
                                  <TableCell key={j}>
                                    <Skeleton className="h-4 w-16" />
                                  </TableCell>
                                ))}
                            </TableRow>
                          ))
                      : dailyStats.map((item) => (
                          <TableRow key={item.date}>
                            <TableCell className="font-medium">
                              {item.date}
                            </TableCell>
                            <TableCell className="text-right">
                              {item.order_count}
                            </TableCell>
                            <TableCell className="text-right font-medium">
                              ¥{formatAmount(item.total_sales)}
                            </TableCell>
                            <TableCell className="text-right text-muted-foreground">
                              ¥{formatAmount(item.commission)}
                            </TableCell>
                            <TableCell className="text-right">
                              {item.dine_in_orders}
                            </TableCell>
                            <TableCell className="text-right">
                              {item.takeout_orders}
                            </TableCell>
                          </TableRow>
                        ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </TabsContent>

          {/* 销售分析 */}
          <TabsContent value="sales" className="space-y-6 mt-6">
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
              {/* 热销菜品 */}
              <Card className="bg-white rounded-xl border shadow-sm">
                <CardHeader className="pb-2">
                  <CardTitle className="text-base font-semibold flex items-center gap-2">
                    <ChefHat className="h-4 w-4 text-orange-600" />
                    热销菜品 TOP 10
                  </CardTitle>
                  <CardDescription>按销量排名</CardDescription>
                </CardHeader>
                <CardContent>
                  <ScrollArea className="h-100 pr-4">
                    <div className="space-y-3">
                      {loading
                        ? Array(5)
                            .fill(0)
                            .map((_, i) => (
                              <Skeleton key={i} className="h-14 w-full" />
                            ))
                        : topDishes.map((dish, i) => {
                            const maxSold = topDishes[0]?.total_sold || 1;
                            const percent = (dish.total_sold / maxSold) * 100;
                            return (
                              <div
                                key={dish.dish_id}
                                className="group flex items-center gap-3 p-3 rounded-lg border hover:bg-slate-50 transition-colors"
                              >
                                <div
                                  className={cn(
                                    "w-8 h-8 rounded-lg flex items-center justify-center text-xs font-bold shrink-0",
                                    i === 0
                                      ? "bg-amber-400 text-white"
                                      : i === 1
                                        ? "bg-slate-300 text-white"
                                        : i === 2
                                          ? "bg-orange-200 text-orange-700"
                                          : "bg-slate-100 text-slate-500"
                                  )}
                                >
                                  {i + 1}
                                </div>
                                <div className="flex-1 min-w-0">
                                  <div className="flex justify-between items-center mb-1">
                                    <span className="text-sm font-medium text-slate-900 truncate">
                                      {dish.dish_name}
                                    </span>
                                    <span className="text-xs text-muted-foreground">
                                      {dish.total_sold} 份
                                    </span>
                                  </div>
                                  <Progress
                                    value={percent}
                                    className="h-1.5"
                                  />
                                  <div className="flex justify-between items-center mt-1">
                                    <span className="text-xs text-muted-foreground">
                                      ¥{formatAmount(dish.dish_price)}/份
                                    </span>
                                    <span className="text-xs font-medium text-primary">
                                      ¥{formatAmount(dish.total_revenue)}
                                    </span>
                                  </div>
                                </div>
                              </div>
                            );
                          })}
                    </div>
                  </ScrollArea>
                </CardContent>
              </Card>

              {/* 分类销售占比 */}
              <Card className="bg-white rounded-xl border shadow-sm">
                <CardHeader className="pb-2">
                  <CardTitle className="text-base font-semibold flex items-center gap-2">
                    <Utensils className="h-4 w-4 text-purple-600" />
                    品类销售分布
                  </CardTitle>
                  <CardDescription>按销售额占比</CardDescription>
                </CardHeader>
                <CardContent>
                  {loading ? (
                    <ChartSkeleton height={200} />
                  ) : (
                    <>
                      <div className="h-50 relative">
                        <ResponsiveContainer width="100%" height="100%">
                          <PieChart>
                            <Pie
                              data={categoryChartData}
                              cx="50%"
                              cy="50%"
                              innerRadius={60}
                              outerRadius={85}
                              paddingAngle={4}
                              dataKey="value"
                            >
                              {categoryChartData.map((_, index) => (
                                <Cell
                                  key={`cell-${index}`}
                                  fill={PIE_COLORS[index % PIE_COLORS.length]}
                                  strokeWidth={0}
                                />
                              ))}
                            </Pie>
                            <Tooltip
                              contentStyle={{ borderRadius: "8px" }}
                              formatter={(value) => [`¥${value ?? 0}`, "销售额"]}
                            />
                          </PieChart>
                        </ResponsiveContainer>
                        <div className="absolute inset-0 flex flex-col items-center justify-center pointer-events-none">
                          <span className="text-2xl font-bold text-slate-900">
                            {categoryStats.length}
                          </span>
                          <span className="text-xs text-muted-foreground">
                            个品类
                          </span>
                        </div>
                      </div>
                      <div className="space-y-2 mt-4">
                        {categoryChartData.slice(0, 5).map((item, i) => (
                          <div
                            key={i}
                            className="flex items-center justify-between"
                          >
                            <div className="flex items-center gap-2">
                              <div
                                className="w-3 h-3 rounded-full"
                                style={{
                                  backgroundColor:
                                    PIE_COLORS[i % PIE_COLORS.length],
                                }}
                              />
                              <span className="text-sm text-muted-foreground">
                                {item.name}
                              </span>
                            </div>
                            <div className="flex items-center gap-2">
                              <span className="text-sm font-medium">
                                ¥{item.value.toFixed(0)}
                              </span>
                              <Badge variant="secondary" className="text-xs">
                                {item.percent.toFixed(0)}%
                              </Badge>
                            </div>
                          </div>
                        ))}
                      </div>
                    </>
                  )}
                </CardContent>
              </Card>
            </div>

            {/* 时段分析 */}
            <Card className="bg-white rounded-xl border shadow-sm">
              <CardHeader className="pb-2">
                <CardTitle className="text-base font-semibold flex items-center gap-2">
                  <Clock className="h-4 w-4 text-cyan-600" />
                  时段分析
                </CardTitle>
                <CardDescription>按小时统计订单分布</CardDescription>
              </CardHeader>
              <CardContent>
                {loading ? (
                  <ChartSkeleton height={200} />
                ) : (
                  <div className="h-50 w-full">
                    <ResponsiveContainer width="100%" height="100%">
                      <BarChart data={hourlyChartData}>
                        <CartesianGrid
                          strokeDasharray="3 3"
                          vertical={false}
                          stroke="#f1f5f9"
                        />
                        <XAxis
                          dataKey="hour"
                          tickFormatter={(v) => `${v}:00`}
                          axisLine={false}
                          tickLine={false}
                          tick={{ fill: "#94a3b8", fontSize: 11 }}
                          interval={2}
                        />
                        <YAxis
                          axisLine={false}
                          tickLine={false}
                          tick={{ fill: "#94a3b8", fontSize: 11 }}
                        />
                        <Tooltip
                          contentStyle={{ borderRadius: "8px" }}
                          formatter={(value, name) => {
                            const safeValue = value ?? 0;

                            return [
                              name === "order_count"
                                ? `${safeValue} 单`
                                : `¥${formatAmount(Number(safeValue))}`,
                              name === "order_count" ? "订单数" : "均单价",
                            ];
                          }}
                          labelFormatter={(label) => `${label}:00`}
                        />
                        <Bar dataKey="order_count" radius={[4, 4, 4, 4]}>
                          {hourlyChartData.map((entry, index) => (
                            <Cell
                              key={`cell-${index}`}
                              fill="hsl(var(--primary))"
                              fillOpacity={Math.max(0.2, entry.intensity)}
                            />
                          ))}
                        </Bar>
                      </BarChart>
                    </ResponsiveContainer>
                  </div>
                )}
                <p className="text-xs text-muted-foreground text-center mt-2">
                  颜色越深代表订单越密集
                </p>
              </CardContent>
            </Card>
          </TabsContent>

          {/* 财务分析 */}
          <TabsContent value="finance" className="space-y-6 mt-6">
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {/* 财务统计卡片 */}
              {[
                {
                  title: "总 GMV",
                  value: financeOverview?.total_gmv,
                  icon: Receipt,
                  color: "text-slate-600",
                  bgColor: "bg-slate-50",
                },
                {
                  title: "商户净收入",
                  value: financeOverview?.net_income,
                  icon: Wallet,
                  color: "text-emerald-600",
                  bgColor: "bg-emerald-50",
                },
                {
                  title: "待结算收入",
                  value: financeOverview?.pending_income,
                  icon: Clock,
                  color: "text-amber-600",
                  bgColor: "bg-amber-50",
                },
              ].map((item, i) => (
                <Card key={i} className="bg-white rounded-xl border shadow-sm">
                  <CardContent className="p-5">
                    <div className="flex items-center gap-3 mb-3">
                      <div className={cn("p-2 rounded-lg", item.bgColor)}>
                        <item.icon className={cn("h-4 w-4", item.color)} />
                      </div>
                      <span className="text-sm text-muted-foreground">
                        {item.title}
                      </span>
                    </div>
                    <p className="text-2xl font-bold text-slate-900">
                      {loading ? (
                        <Skeleton className="h-8 w-28" />
                      ) : (
                        `¥${formatAmount(item.value)}`
                      )}
                    </p>
                  </CardContent>
                </Card>
              ))}
            </div>

            {/* 财务明细 */}
            <Card className="bg-white rounded-xl border shadow-sm">
              <CardHeader className="pb-2">
                <CardTitle className="text-base font-semibold">
                  财务明细
                </CardTitle>
                <CardDescription>
                  {start_date} 至 {end_date}
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="grid gap-4 sm:grid-cols-2">
                  {[
                    {
                      label: "已完成订单",
                      value: financeOverview?.completed_orders || 0,
                      isAmount: false,
                    },
                    {
                      label: "待结算订单",
                      value: financeOverview?.pending_orders || 0,
                      isAmount: false,
                    },
                    {
                      label: "商户收入",
                      value: financeOverview?.total_income,
                      isAmount: true,
                    },
                    {
                      label: "平台服务费",
                      value: financeOverview?.total_platform_fee,
                      isAmount: true,
                    },
                    {
                      label: "运营商服务费",
                      value: financeOverview?.total_operator_fee,
                      isAmount: true,
                    },
                    {
                      label: "服务费合计",
                      value: financeOverview?.total_service_fee,
                      isAmount: true,
                    },
                    {
                      label: "满返支出",
                      value: financeOverview?.total_promotion_exp,
                      isAmount: true,
                    },
                    {
                      label: "满返订单数",
                      value: financeOverview?.promotion_orders || 0,
                      isAmount: false,
                    },
                  ].map((item, i) => (
                    <div
                      key={i}
                      className="flex items-center justify-between py-2 border-b last:border-0"
                    >
                      <span className="text-sm text-muted-foreground">
                        {item.label}
                      </span>
                      <span className="font-medium">
                        {loading ? (
                          <Skeleton className="h-4 w-16" />
                        ) : item.isAmount ? (
                          `¥${formatAmount(item.value as number)}`
                        ) : (
                          item.value
                        )}
                      </span>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          {/* 客户分析 */}
          <TabsContent value="customer" className="space-y-6 mt-6">
            {/* 复购分析卡片 */}
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
              {[
                {
                  label: "总客户数",
                  value: repurchaseStats?.total_users || 0,
                  icon: Users,
                  bgColor: "bg-blue-50",
                  color: "text-blue-600",
                },
                {
                  label: "复购客户",
                  value: repurchaseStats?.repeat_users || 0,
                  icon: RefreshCw,
                  bgColor: "bg-emerald-50",
                  color: "text-emerald-600",
                },
                {
                  label: "复购率",
                  value: `${(repurchaseStats?.repurchase_rate || 0).toFixed(1)}%`,
                  icon: TrendingUp,
                  bgColor: "bg-amber-50",
                  color: "text-amber-600",
                },
                {
                  label: "人均订单",
                  value: `${(repurchaseStats?.avg_orders_per_user || 0).toFixed(1)} 单`,
                  icon: ShoppingBag,
                  bgColor: "bg-purple-50",
                  color: "text-purple-600",
                },
              ].map((item, i) => (
                <Card key={i} className="bg-white rounded-xl border shadow-sm">
                  <CardContent className="p-5">
                    <div className="flex items-center gap-3 mb-3">
                      <div className={cn("p-2 rounded-lg", item.bgColor)}>
                        <item.icon className={cn("h-4 w-4", item.color)} />
                      </div>
                    </div>
                    <p className="text-xs text-muted-foreground mb-1">
                      {item.label}
                    </p>
                    <p className="text-2xl font-bold text-slate-900">
                      {loading ? <Skeleton className="h-8 w-20" /> : item.value}
                    </p>
                  </CardContent>
                </Card>
              ))}
            </div>

            {/* 客户消费排行 */}
            <Card className="bg-white rounded-xl border shadow-sm">
              <CardHeader className="pb-2">
                <CardTitle className="text-base font-semibold flex items-center gap-2">
                  <Users className="h-4 w-4 text-purple-600" />
                  优质客户排行
                </CardTitle>
                <CardDescription>按消费金额排名</CardDescription>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>客户</TableHead>
                      <TableHead className="text-right">订单数</TableHead>
                      <TableHead className="text-right">总消费</TableHead>
                      <TableHead className="text-right">均单价</TableHead>
                      <TableHead className="text-right hidden md:table-cell">
                        最近下单
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {loading
                      ? Array(5)
                          .fill(0)
                          .map((_, i) => (
                            <TableRow key={i}>
                              {Array(5)
                                .fill(0)
                                .map((_, j) => (
                                  <TableCell key={j}>
                                    <Skeleton className="h-4 w-16" />
                                  </TableCell>
                                ))}
                            </TableRow>
                          ))
                        : customerStats.map((customer) => (
                          <TableRow key={customer.user_id}>
                            <TableCell>
                              <div className="flex items-center gap-3">
                                <Avatar className="h-8 w-8">
                                  <AvatarImage src={customer.avatar_url} />
                                  <AvatarFallback className="text-xs">
                                    {customer.full_name?.slice(0, 1) || "U"}
                                  </AvatarFallback>
                                </Avatar>
                                <div>
                                  <p className="font-medium text-sm">
                                    {customer.full_name ||
                                      customer.phone ||
                                      `用户${customer.user_id}`}
                                  </p>
                                  {customer.phone && (
                                    <p className="text-xs text-muted-foreground">
                                      {customer.phone}
                                    </p>
                                  )}
                                </div>
                              </div>
                            </TableCell>
                            <TableCell className="text-right">
                              <Badge variant="secondary">
                                {customer.total_orders}
                              </Badge>
                            </TableCell>
                            <TableCell className="text-right font-medium text-primary">
                              ¥{formatAmount(customer.total_amount)}
                            </TableCell>
                            <TableCell className="text-right">
                              ¥{formatAmount(customer.avg_order_amount)}
                            </TableCell>
                            <TableCell className="text-right text-muted-foreground hidden md:table-cell">
                              {customer.last_order_at
                                ? new Date(
                                    customer.last_order_at
                                  ).toLocaleDateString()
                                : "-"}
                            </TableCell>
                          </TableRow>
                        ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </PageContent>
    </PageShell>
  );
}
