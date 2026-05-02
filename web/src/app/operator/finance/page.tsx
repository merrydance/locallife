"use client";

import { useCallback, useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { PageContent, PageHeader, PageShell } from "@/components/merchant/layout/page-shell";
import { apiGet, formatAmount, getRecentRange } from "@/lib/api";
import { getUserFacingErrorMessage } from "@/lib/user-facing-errors";
import { formatProfitConfigStatus } from "@/lib/operator-display";
import type {
  OperatorApplicationStatusResponse,
  OperatorCommissionResponse,
  OperatorProfitSharingConfigListResponse,
} from "@/types/operator-console";
import type { OperatorFinanceOverviewResponse } from "@/types/operator-stats";

export default function OperatorFinancePage() {
  const [overview, setOverview] = useState<OperatorFinanceOverviewResponse | null>(null);
  const [commission, setCommission] = useState<OperatorCommissionResponse | null>(null);
  const [configs, setConfigs] = useState<OperatorProfitSharingConfigListResponse | null>(null);
  const [financeNotice, setFinanceNotice] = useState<string | null>(null);
  const [financeError, setFinanceError] = useState<string | null>(null);

  const load = useCallback(() => {
    const range = getRecentRange(30);
    Promise.allSettled([
      apiGet<OperatorFinanceOverviewResponse>("/operators/me/finance/overview"),
      apiGet<OperatorCommissionResponse>("/operators/me/commission", { ...range, page: 1, limit: 20 }),
      apiGet<OperatorProfitSharingConfigListResponse>("/operators/me/profit-sharing/configs", {
        page: 1,
        limit: 20,
      }),
      apiGet<OperatorApplicationStatusResponse>("/operator/application"),
    ])
      .then(([overviewResult, commissionResult, configResult, operatorApplicationResult]) => {
        const overviewRes = overviewResult.status === "fulfilled" ? overviewResult.value : null;
        const commissionRes = commissionResult.status === "fulfilled" ? commissionResult.value : null;
        const configRes = configResult.status === "fulfilled" ? configResult.value : null;
        const operatorApplication =
          operatorApplicationResult.status === "fulfilled" ? operatorApplicationResult.value : null;

        let nextFinanceNotice: string | null = null;
        const errors: string[] = [];

        if (overviewResult.status === "rejected") {
          console.error("Load operator finance overview failed", overviewResult.reason);
          errors.push(
            getUserFacingErrorMessage(
              overviewResult.reason,
              "加载运营商财务概览失败，请刷新重试；如持续失败请联系平台管理员处理。",
            ),
          );
        }
        if (commissionResult.status === "rejected") {
          console.error("Load operator commission list failed", commissionResult.reason);
          errors.push(
            getUserFacingErrorMessage(
              commissionResult.reason,
              "加载平台佣金明细失败，请刷新重试；如持续失败请联系平台管理员处理。",
            ),
          );
        }
        if (configResult.status === "rejected") {
          console.error("Load operator profit sharing configs failed", configResult.reason);
          errors.push(
            getUserFacingErrorMessage(
              configResult.reason,
              "加载分账规则失败，请刷新重试；如持续失败请联系平台管理员处理。",
            ),
          );
        }
        if (operatorApplicationResult.status === "rejected") {
          console.error("Load operator application status failed", operatorApplicationResult.reason);
          nextFinanceNotice = "暂未获取运营商入驻状态；如分账收入或规则展示异常，请刷新后重试。";
        } else if (operatorApplication?.status !== "active") {
          nextFinanceNotice = "当前运营商入驻尚未激活；激活后才会产生普通服务商分账收入。";
        }

        setOverview(overviewRes);
        setCommission(commissionRes);
        setConfigs(configRes);
        setFinanceNotice(nextFinanceNotice);
        setFinanceError(errors.length > 0 ? Array.from(new Set(errors)).join("；") : null);
      })
      .catch((error: unknown) => {
        console.error("Load operator finance page failed", error);
        setFinanceError(
          getUserFacingErrorMessage(
            error,
            "加载运营商财务页失败，请刷新重试；如持续失败请联系平台管理员处理。",
          ),
        );
      });
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <PageShell>
      <PageHeader
        title="财务管理"
        description="平台佣金明细、分账规则与微信支付资金处理指引"
        actions={
          <div className="flex items-center gap-2">
            <Badge variant="secondary">运营商</Badge>
            <Button variant="outline" size="sm" onClick={load}>
              刷新
            </Button>
          </div>
        }
      />
      <PageContent className="space-y-4">
        {financeError && (
          <div className="rounded-lg border border-rose-300 bg-rose-50 px-4 py-3 text-sm text-rose-700">
            {financeError}
          </div>
        )}
        <Tabs defaultValue="overview" className="space-y-4">
          <TabsList className="grid w-full grid-cols-3 lg:w-auto lg:inline-grid">
            <TabsTrigger value="overview">概览</TabsTrigger>
            <TabsTrigger value="finance">资金指引</TabsTrigger>
            <TabsTrigger value="config">分账规则</TabsTrigger>
          </TabsList>

          <TabsContent value="overview" className="space-y-4 m-0">
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm text-muted-foreground">当月运营商收入</CardTitle>
                </CardHeader>
                <CardContent className="text-2xl font-semibold">
                  ¥{formatAmount(overview?.current_month.operator_income ?? 0)}
                </CardContent>
              </Card>
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm text-muted-foreground">当月平台佣金</CardTitle>
                </CardHeader>
                <CardContent className="text-2xl font-semibold">
                  ¥{formatAmount(overview?.current_month.total_commission ?? 0)}
                </CardContent>
              </Card>
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm text-muted-foreground">累计运营商收入</CardTitle>
                </CardHeader>
                <CardContent className="text-2xl font-semibold">
                  ¥{formatAmount(overview?.total.operator_income ?? 0)}
                </CardContent>
              </Card>
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm text-muted-foreground">待分账佣金</CardTitle>
                </CardHeader>
                <CardContent className="text-2xl font-semibold">
                  ¥{formatAmount(overview?.current_month.pending_commission ?? 0)}
                </CardContent>
              </Card>
            </div>

            <Card>
              <CardHeader>
                <CardTitle>平台佣金明细（近30天）</CardTitle>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>日期</TableHead>
                      <TableHead>订单数</TableHead>
                      <TableHead>GMV</TableHead>
                      <TableHead>平台佣金率</TableHead>
                      <TableHead>平台佣金</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(commission?.items ?? []).map((item) => (
                      <TableRow key={item.date}>
                        <TableCell>{item.date}</TableCell>
                        <TableCell>{item.order_count}</TableCell>
                        <TableCell>¥{formatAmount(item.total_gmv)}</TableCell>
                        <TableCell>{item.commission_rate}</TableCell>
                        <TableCell>¥{formatAmount(item.commission)}</TableCell>
                      </TableRow>
                    ))}
                    {(commission?.items?.length ?? 0) === 0 && (
                      <TableRow>
                        <TableCell colSpan={5} className="text-muted-foreground">
                          暂无可展示数据
                        </TableCell>
                      </TableRow>
                    )}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="finance" className="space-y-4 m-0">
            {financeNotice && (
              <div className="rounded-lg border px-4 py-3 text-sm">
                {financeNotice}
              </div>
            )}

            <Card>
              <CardHeader>
                <CardTitle>普通服务商资金处理</CardTitle>
              </CardHeader>
              <CardContent className="grid gap-3 text-sm md:grid-cols-2">
                <div className="rounded-lg border bg-muted/40 p-4">
                  <div className="font-medium">资金操作入口</div>
                  <p className="mt-2 text-muted-foreground">
                    普通服务商模式不支持在平台内查询余额、发起提现、注销提现、补差或垫付回补。
                    请前往微信支付商户平台或微信支付商家助手处理资金操作。
                  </p>
                </div>
                <div className="rounded-lg border bg-muted/40 p-4">
                  <div className="font-medium">平台内保留能力</div>
                  <p className="mt-2 text-muted-foreground">
                    本页保留运营商分账收入统计、平台佣金明细和分账规则展示；商户侧保留普通服务商进件与结算账户查询/修改。
                  </p>
                </div>
                <div className="rounded-lg border bg-muted/40 p-4">
                  <div className="font-medium">异常处理指引</div>
                  <p className="mt-2 text-muted-foreground">
                    如支付、退款或分账被限制，请联系平台管理员查看普通服务商商户管控诊断，并按微信侧恢复指引处理。
                  </p>
                </div>
                <div className="rounded-lg border bg-muted/40 p-4">
                  <div className="font-medium">当前可核对数据</div>
                  <p className="mt-2 text-muted-foreground">
                    可在“概览”和“分账规则”中核对分账收入、平台佣金和规则配置；资金到账状态以微信支付商户平台为准。
                  </p>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="config" className="space-y-4 m-0">
            <Card>
              <CardHeader>
                <CardTitle>分账规则配置</CardTitle>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>ID</TableHead>
                      <TableHead>状态</TableHead>
                      <TableHead>来源</TableHead>
                      <TableHead>平台费率</TableHead>
                      <TableHead>运营商费率</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(configs?.items ?? []).map((item) => (
                      <TableRow key={item.id}>
                        <TableCell>{item.id}</TableCell>
                        <TableCell>{formatProfitConfigStatus(item.status)}</TableCell>
                        <TableCell>{item.order_source}</TableCell>
                        <TableCell>{item.platform_rate}</TableCell>
                        <TableCell>{item.operator_rate}</TableCell>
                      </TableRow>
                    ))}
                    {(configs?.items?.length ?? 0) === 0 && (
                      <TableRow>
                        <TableCell colSpan={5} className="text-muted-foreground">
                          暂无可展示数据
                        </TableCell>
                      </TableRow>
                    )}
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
