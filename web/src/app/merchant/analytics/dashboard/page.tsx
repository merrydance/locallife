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
import {
  apiGet,
  formatAmount,
  formatDate,
  formatGrowthRate,
  formatPercentage,
  getGrowthColor,
} from "@/lib/api";

type StatsOverview = {
  total_orders: number;
  total_revenue: number;
  total_customers: number;
  avg_order_value: number;
  completion_rate: number;
  growth_rate: number;
};

type DailyStat = {
  date: string;
  orders: number;
  revenue: number;
  customers: number;
  avg_order_value: number;
};

type HourlyStat = {
  hour: number;
  orders: number;
  revenue: number;
};

type TopDish = {
  dish_id: number;
  dish_name: string;
  sales_count: number;
  revenue: number;
  rank: number;
};

type CategoryStat = {
  category_id: number;
  category_name: string;
  sales_count: number;
  revenue: number;
  percentage: number;
};

type OrderSourceStat = {
  source: string;
  orders: number;
  revenue: number;
  percentage: number;
};

type CustomerStat = {
  user_id: number;
  username: string;
  total_orders: number;
  total_spent: number;
  avg_order_value: number;
  last_order_date: string;
};

type RepurchaseStat = {
  total_customers: number;
  repurchase_customers: number;
  repurchase_rate: number;
  avg_repurchase_interval: number;
};

type FinanceOverview = {
  completed_orders: number;
  net_income: number;
  pending_income: number;
  pending_orders: number;
  promotion_orders: number;
  total_gmv: number;
  total_income: number;
  total_operator_fee: number;
  total_platform_fee: number;
  total_promotion_exp: number;
  total_service_fee: number;
};

const fallbackOverview: StatsOverview = {
  total_orders: 0,
  total_revenue: 0,
  total_customers: 0,
  avg_order_value: 0,
  completion_rate: 0,
  growth_rate: 0,
};

const fallbackDaily: DailyStat[] = [];

const fallbackTopDishes: TopDish[] = [];

const fallbackCategories: CategoryStat[] = [];

const fallbackCustomers: CustomerStat[] = [];

const fallbackRepurchase: RepurchaseStat = {
  total_customers: 0,
  repurchase_customers: 0,
  repurchase_rate: 0,
  avg_repurchase_interval: 0,
};

const fallbackFinance: FinanceOverview = {
  completed_orders: 0,
  net_income: 0,
  pending_income: 0,
  pending_orders: 0,
  promotion_orders: 0,
  total_gmv: 0,
  total_income: 0,
  total_operator_fee: 0,
  total_platform_fee: 0,
  total_promotion_exp: 0,
  total_service_fee: 0,
};

const fallbackHourly: HourlyStat[] = [];

const fallbackSources: OrderSourceStat[] = [];

type TabKey = "overview" | "sales" | "finance" | "customer";

type OverviewData = { overview: StatsOverview; daily: DailyStat[] };
type SalesData = {
  topDishes: TopDish[];
  categories: CategoryStat[];
  hourly: HourlyStat[];
  sources: OrderSourceStat[];
};
type FinanceData = { finance: FinanceOverview };
type CustomerData = { customers: CustomerStat[]; repurchase: RepurchaseStat };
type DashboardData = OverviewData | SalesData | FinanceData | CustomerData;

function getDefaultRange() {
  const end = new Date();
  const start = new Date();
  start.setDate(end.getDate() - 30);
  return { start_date: formatDate(start), end_date: formatDate(end) };
}

function getQueryParam(
  value: string | string[] | undefined,
  fallback: string
) {
  if (!value) return fallback;
  return Array.isArray(value) ? value[0] : value;
}

async function loadDashboardData(
  tab: TabKey,
  range: { start_date: string; end_date: string }
): Promise<DashboardData> {
  if (tab === "overview") {
    const [overview, daily] = await Promise.all([
      apiGet<StatsOverview>("/merchant/stats/overview", range).catch(
        () => fallbackOverview
      ),
      apiGet<DailyStat[]>("/merchant/stats/daily", range).catch(
        () => fallbackDaily
      ),
    ]);

    return { overview, daily };
  }

  if (tab === "sales") {
    const [topDishes, categories, hourly, sources] = await Promise.all([
      apiGet<TopDish[]>("/merchant/stats/dishes/top", {
        ...range,
        limit: 10,
      }).catch(() => fallbackTopDishes),
      apiGet<CategoryStat[]>("/merchant/stats/categories", range).catch(
        () => fallbackCategories
      ),
      apiGet<HourlyStat[]>("/merchant/stats/hourly", { date: range.end_date }).catch(
        () => fallbackHourly
      ),
      apiGet<OrderSourceStat[]>("/merchant/stats/sources", range).catch(
        () => fallbackSources
      ),
    ]);

    return { topDishes, categories, hourly, sources };
  }

  if (tab === "finance") {
    const finance = await apiGet<FinanceOverview>(
      "/merchant/finance/overview",
      range
    ).catch(() => fallbackFinance);

    return { finance };
  }

  const [customers, repurchase] = await Promise.all([
    apiGet<CustomerStat[]>("/merchant/stats/customers", {
      ...range,
      page_id: 1,
      page_size: 20,
    }).catch(() => fallbackCustomers),
    apiGet<RepurchaseStat>("/merchant/stats/repurchase", range).catch(
      () => fallbackRepurchase
    ),
  ]);

  return { customers, repurchase };
}

export default async function AnalyticsDashboardPage({
  searchParams,
}: {
  searchParams?: Record<string, string | string[] | undefined>;
}) {
  const defaultRange = getDefaultRange();
  const tab = (getQueryParam(searchParams?.tab, "overview") || "overview") as TabKey;
  const start_date = getQueryParam(searchParams?.start_date, defaultRange.start_date);
  const end_date = getQueryParam(searchParams?.end_date, defaultRange.end_date);
  const range = { start_date, end_date };

  const data = await loadDashboardData(tab, range);
  const overviewData = tab === "overview" ? (data as OverviewData) : null;
  const salesData = tab === "sales" ? (data as SalesData) : null;
  const financeData = tab === "finance" ? (data as FinanceData) : null;
  const customerData = tab === "customer" ? (data as CustomerData) : null;
  return (
    <div className="space-y-6">
      <div className="rounded-lg border bg-card p-6 lg:hidden">
        <div className="text-center text-lg font-semibold">数据分析中心</div>
        <p className="mt-2 text-sm text-muted-foreground">
          建议在平板或电脑端查看完整报表。
        </p>
        <Button variant="outline" className="mt-4 w-full" asChild>
          <a href="/merchant/dashboard">返回工作台</a>
        </Button>
      </div>

      <div className="hidden space-y-6 lg:block">
        <div className="flex flex-wrap items-center justify-between gap-4 rounded-lg border bg-card p-4">
          <div className="flex items-center gap-3">
            <Button variant="outline" size="sm" asChild>
              <a
                href={`/merchant/analytics/dashboard?tab=${tab}&start_date=${start_date}&end_date=${end_date}`}
              >
                {start_date}
              </a>
            </Button>
            <span className="text-sm text-muted-foreground">至</span>
            <Button variant="outline" size="sm" asChild>
              <a
                href={`/merchant/analytics/dashboard?tab=${tab}&start_date=${start_date}&end_date=${end_date}`}
              >
                {end_date}
              </a>
            </Button>
          </div>
          <div className="flex gap-2">
            {[7, 30, 90].map((days) => {
              const end = new Date();
              const start = new Date();
              start.setDate(end.getDate() - days);
              const quickStart = formatDate(start);
              const quickEnd = formatDate(end);
              return (
                <Button key={days} variant="ghost" size="sm" asChild>
                  <a
                    href={`/merchant/analytics/dashboard?tab=${tab}&start_date=${quickStart}&end_date=${quickEnd}`}
                  >
                    近{days}天
                  </a>
                </Button>
              );
            })}
          </div>
        </div>

        {tab === "overview" && overviewData ? (
          <div className="space-y-6">
            <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <Card>
                <CardHeader className="pb-2">
                  <CardDescription>总订单数</CardDescription>
                  <CardTitle className="text-2xl">
                    {overviewData.overview.total_orders}
                  </CardTitle>
                </CardHeader>
                <CardContent
                  className={`text-xs ${getGrowthColor(overviewData.overview.growth_rate)}`}
                >
                  {formatGrowthRate(overviewData.overview.growth_rate)}
                </CardContent>
              </Card>
              <Card>
                <CardHeader className="pb-2">
                  <CardDescription>总营收</CardDescription>
                  <CardTitle className="text-2xl">
                    ¥{formatAmount(overviewData.overview.total_revenue)}
                  </CardTitle>
                </CardHeader>
                <CardContent
                  className={`text-xs ${getGrowthColor(overviewData.overview.growth_rate)}`}
                >
                  {formatGrowthRate(overviewData.overview.growth_rate)}
                </CardContent>
              </Card>
              <Card>
                <CardHeader className="pb-2">
                  <CardDescription>客单价</CardDescription>
                  <CardTitle className="text-2xl">
                    ¥{formatAmount(overviewData.overview.avg_order_value)}
                  </CardTitle>
                </CardHeader>
                <CardContent className="text-xs text-muted-foreground">
                  客户 {overviewData.overview.total_customers}
                </CardContent>
              </Card>
              <Card>
                <CardHeader className="pb-2">
                  <CardDescription>完成率</CardDescription>
                  <CardTitle className="text-2xl">
                    {formatPercentage(overviewData.overview.completion_rate)}
                  </CardTitle>
                </CardHeader>
                <CardContent className="text-xs text-muted-foreground">
                  指标区间 {start_date} - {end_date}
                </CardContent>
              </Card>
            </section>

            <Card>
              <CardHeader>
                <CardTitle>销售趋势</CardTitle>
                <CardDescription>对应 /v1/merchant/stats/daily</CardDescription>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>日期</TableHead>
                      <TableHead>订单</TableHead>
                      <TableHead>营收</TableHead>
                      <TableHead>客户数</TableHead>
                      <TableHead className="text-right">客单价</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {overviewData.daily.map((item) => (
                      <TableRow key={item.date}>
                        <TableCell className="font-medium">{item.date}</TableCell>
                        <TableCell>{item.orders}</TableCell>
                        <TableCell>¥{formatAmount(item.revenue)}</TableCell>
                        <TableCell>{item.customers}</TableCell>
                        <TableCell className="text-right">
                          ¥{formatAmount(item.avg_order_value)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </div>
        ) : null}

        {tab === "sales" && salesData ? (
          <div className="space-y-6">
            <Card>
              <CardHeader>
                <CardTitle>热门菜品 TOP10</CardTitle>
                <CardDescription>对应 /v1/merchant/stats/dishes/top</CardDescription>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>排名</TableHead>
                      <TableHead>菜品</TableHead>
                      <TableHead>销量</TableHead>
                      <TableHead className="text-right">营收</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {salesData.topDishes.map((dish) => (
                      <TableRow key={dish.dish_id}>
                        <TableCell>
                          <Badge variant="secondary">#{dish.rank}</Badge>
                        </TableCell>
                        <TableCell className="font-medium">
                          {dish.dish_name}
                        </TableCell>
                        <TableCell>{dish.sales_count}</TableCell>
                        <TableCell className="text-right">
                          ¥{formatAmount(dish.revenue)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>分类销售占比</CardTitle>
                <CardDescription>对应 /v1/merchant/stats/categories</CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                {salesData.categories.map((category) => (
                  <div key={category.category_id} className="space-y-1">
                    <div className="flex items-center justify-between text-sm">
                      <span className="font-medium">{category.category_name}</span>
                      <span className="text-muted-foreground">
                        ¥{formatAmount(category.revenue)}
                      </span>
                    </div>
                    <div className="h-2 w-full rounded-full bg-muted">
                      <div
                        className="h-2 rounded-full bg-primary"
                        style={{ width: `${category.percentage * 100}%` }}
                      />
                    </div>
                    <div className="flex items-center justify-between text-xs text-muted-foreground">
                      <span>销量 {category.sales_count}</span>
                      <span>{formatPercentage(category.percentage)}</span>
                    </div>
                  </div>
                ))}
              </CardContent>
            </Card>

            <section className="grid gap-4 lg:grid-cols-2">
              <Card>
                <CardHeader>
                  <CardTitle>时段分布</CardTitle>
                  <CardDescription>对应 /v1/merchant/stats/hourly</CardDescription>
                </CardHeader>
                <CardContent>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>时段</TableHead>
                        <TableHead>订单</TableHead>
                        <TableHead className="text-right">营收</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {salesData.hourly.map((item) => (
                        <TableRow key={item.hour}>
                          <TableCell className="font-medium">{item.hour}:00</TableCell>
                          <TableCell>{item.orders}</TableCell>
                          <TableCell className="text-right">
                            ¥{formatAmount(item.revenue)}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>来源分析</CardTitle>
                  <CardDescription>对应 /v1/merchant/stats/sources</CardDescription>
                </CardHeader>
                <CardContent>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>来源</TableHead>
                        <TableHead>订单</TableHead>
                        <TableHead>占比</TableHead>
                        <TableHead className="text-right">营收</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {salesData.sources.map((item) => (
                        <TableRow key={item.source}>
                          <TableCell className="font-medium">{item.source}</TableCell>
                          <TableCell>{item.orders}</TableCell>
                          <TableCell>{formatPercentage(item.percentage)}</TableCell>
                          <TableCell className="text-right">
                            ¥{formatAmount(item.revenue)}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </CardContent>
              </Card>
            </section>
          </div>
        ) : null}

        {tab === "finance" && financeData ? (
          <Card>
            <CardHeader>
              <CardTitle>财务概览</CardTitle>
              <CardDescription>对应 /v1/merchant/finance/overview</CardDescription>
            </CardHeader>
            <CardContent className="grid gap-3 text-sm sm:grid-cols-2">
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">已完成订单</span>
                <span className="font-medium">{financeData.finance.completed_orders}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">总 GMV</span>
                <span className="font-medium">¥{formatAmount(financeData.finance.total_gmv)}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">净收入</span>
                <span className="font-medium">¥{formatAmount(financeData.finance.net_income)}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">商户收入</span>
                <span className="font-medium">¥{formatAmount(financeData.finance.total_income)}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">平台服务费</span>
                <span className="font-medium">¥{formatAmount(financeData.finance.total_platform_fee)}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">运营服务费</span>
                <span className="font-medium">¥{formatAmount(financeData.finance.total_operator_fee)}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">服务费合计</span>
                <span className="font-medium">¥{formatAmount(financeData.finance.total_service_fee)}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">满返支出</span>
                <span className="font-medium">¥{formatAmount(financeData.finance.total_promotion_exp)}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">待结算收入</span>
                <span className="font-medium">¥{formatAmount(financeData.finance.pending_income)}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">待结算订单</span>
                <span className="font-medium">{financeData.finance.pending_orders}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">满返订单</span>
                <span className="font-medium">{financeData.finance.promotion_orders}</span>
              </div>
            </CardContent>
          </Card>
        ) : null}

        {tab === "customer" && customerData ? (
          <div className="space-y-6">
            <Card>
              <CardHeader>
                <CardTitle>复购分析</CardTitle>
                <CardDescription>对应 /v1/merchant/stats/repurchase</CardDescription>
              </CardHeader>
              <CardContent className="grid gap-3 text-sm sm:grid-cols-2">
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">总客户数</span>
                  <span className="font-medium">{customerData.repurchase.total_customers}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">复购客户</span>
                  <span className="font-medium">
                    {customerData.repurchase.repurchase_customers}
                  </span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">复购率</span>
                  <Badge variant="outline">
                    {formatPercentage(customerData.repurchase.repurchase_rate)}
                  </Badge>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">平均复购间隔</span>
                  <span className="font-medium">
                    {customerData.repurchase.avg_repurchase_interval} 天
                  </span>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>客户消费排行</CardTitle>
                <CardDescription>对应 /v1/merchant/stats/customers</CardDescription>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>客户</TableHead>
                      <TableHead>订单数</TableHead>
                      <TableHead>总消费</TableHead>
                      <TableHead>客单价</TableHead>
                      <TableHead className="text-right">最近下单</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {customerData.customers.map((customer) => (
                      <TableRow key={customer.user_id}>
                        <TableCell className="font-medium">
                          {customer.username}
                        </TableCell>
                        <TableCell>{customer.total_orders}</TableCell>
                        <TableCell>¥{formatAmount(customer.total_spent)}</TableCell>
                        <TableCell>
                          ¥{formatAmount(customer.avg_order_value)}
                        </TableCell>
                        <TableCell className="text-right">
                          {customer.last_order_date}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </div>
        ) : null}
      </div>
    </div>
  );
}
