"use client";

import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { PageContent, PageHeader, PageShell } from "@/components/merchant/layout/page-shell";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { apiGet, apiPost } from "@/lib/api";
import {
  formatFoodSafetyIncidentType,
  formatSafetyStatus,
  safetyStatusOptions,
} from "@/lib/operator-display";
import type {
  OperatorFoodSafetyCaseDetailResponse,
  OperatorFoodSafetyCaseItem,
  OperatorFoodSafetyCaseListResponse,
} from "@/types/operator-console";

export default function OperatorSafetyPage() {
  const [status, setStatus] = useState<string>("all");
  const [data, setData] = useState<OperatorFoodSafetyCaseListResponse | null>(null);
  const [detail, setDetail] = useState<OperatorFoodSafetyCaseDetailResponse | null>(null);
  const [investigationReport, setInvestigationReport] = useState("");
  const [merchantRectificationReport, setMerchantRectificationReport] = useState("");
  const [resolution, setResolution] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [investigating, setInvestigating] = useState(false);
  const [resolving, setResolving] = useState(false);
  const [investigateConfirmOpen, setInvestigateConfirmOpen] = useState(false);
  const [resolveConfirmOpen, setResolveConfirmOpen] = useState(false);

  const load = useCallback(() => {
    apiGet<OperatorFoodSafetyCaseListResponse>("/operator/food-safety/cases", {
      page: 1,
      limit: 20,
      status: status === "all" ? undefined : status,
    })
      .then((res) => {
        setData(res);
        setError(null);
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : "加载失败"));
  }, [status]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (error) {
      toast.error(error);
    }
  }, [error]);

  useEffect(() => {
    if (message) {
      toast.success(message);
    }
  }, [message]);

  const syncDraftsFromCase = (item: OperatorFoodSafetyCaseItem) => {
    setInvestigationReport(item.investigation_report ?? "");
    setMerchantRectificationReport(item.merchant_rectification_report ?? "");
    setResolution(item.resolution ?? "");
  };

  const loadDetail = (id: number) => {
    apiGet<OperatorFoodSafetyCaseDetailResponse>(`/operator/food-safety/cases/${id}`)
      .then((res) => {
        setDetail(res);
        syncDraftsFromCase(res.case);
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : "详情加载失败"));
  };

  const submitInvestigation = async () => {
    if (!detail) {
      return;
    }
    if (investigationReport.trim().length < 10) {
      setError("调查报告至少需要 10 个字");
      return;
    }

    setInvestigating(true);
    try {
      const updated = await apiPost<OperatorFoodSafetyCaseItem>(`/operator/food-safety/cases/${detail.case.id}/investigate`, {
        investigation_report: investigationReport.trim(),
      });
      setMessage(`案件 #${updated.id} 已更新为调查中`);
      load();
      loadDetail(detail.case.id);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "提交调查报告失败");
    } finally {
      setInvestigating(false);
    }
  };

  const submitResolution = async () => {
    if (!detail) {
      return;
    }
    if (merchantRectificationReport.trim().length < 10) {
      setError("商户整改报告至少需要 10 个字");
      return;
    }
    if (resolution.trim().length < 5) {
      setError("处置结论至少需要 5 个字");
      return;
    }

    setResolving(true);
    try {
      const updated = await apiPost<OperatorFoodSafetyCaseItem>(`/operator/food-safety/cases/${detail.case.id}/resolve`, {
        investigation_report: investigationReport.trim() || undefined,
        merchant_rectification_report: merchantRectificationReport.trim(),
        resolution: resolution.trim(),
      });
      setMessage(`案件 #${updated.id} 已结案并恢复商户营业资格`);
      load();
      loadDetail(detail.case.id);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "提交结案失败");
    } finally {
      setResolving(false);
    }
  };

  return (
    <PageShell>
      <PageHeader
        title="食安案件"
        description="查看顾客食安上报触发的熔断案件，并完成调查与结案恢复"
        actions={<Badge variant="secondary">运营商</Badge>}
      />
      <PageContent className="space-y-4">
        <Card>
          <CardHeader>
            <CardTitle>筛选条件</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-3 md:flex-row md:items-center">
            <Select value={status} onValueChange={setStatus}>
              <SelectTrigger className="w-full md:w-52">
                <SelectValue placeholder="案件状态" />
              </SelectTrigger>
              <SelectContent>
                {safetyStatusOptions.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button onClick={load}>重新加载</Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>案件列表</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>案件ID</TableHead>
                  <TableHead>商户</TableHead>
                  <TableHead>问题产品</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>触发时间</TableHead>
                  <TableHead>操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(data?.items ?? []).map((item) => (
                  <TableRow key={item.id} className={detail?.case.id === item.id ? "bg-muted/50" : undefined}>
                    <TableCell>{item.id}</TableCell>
                    <TableCell>{item.merchant_id}</TableCell>
                    <TableCell>{item.primary_product_label || item.primary_product_key || "未识别"}</TableCell>
                    <TableCell>{formatSafetyStatus(item.status)}</TableCell>
                    <TableCell>{new Date(item.suspended_at).toLocaleString()}</TableCell>
                    <TableCell>
                      <Button variant="outline" size="sm" onClick={() => loadDetail(item.id)}>
                        查看详情
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
                {(!data || data.items.length === 0) && (
                  <TableRow>
                    <TableCell colSpan={6} className="text-muted-foreground">
                      当前筛选条件下暂无案件
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        {detail && (
          <>
            <Card>
              <CardHeader>
                <CardTitle>案件详情 #{detail.case.id}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid gap-2 text-sm md:grid-cols-2">
                  <div>商户 ID：{detail.case.merchant_id}</div>
                  <div>区域 ID：{detail.case.region_id}</div>
                  <div>问题产品：{detail.case.primary_product_label || detail.case.primary_product_key || "未识别"}</div>
                  <div>案件状态：{formatSafetyStatus(detail.case.status)}</div>
                  <div>熔断时间：{new Date(detail.case.suspended_at).toLocaleString()}</div>
                  <div>最近更新：{new Date(detail.case.updated_at).toLocaleString()}</div>
                  <div>案件创建：{new Date(detail.case.created_at).toLocaleString()}</div>
                  <div>关联上报数：{detail.incidents.length}</div>
                  {detail.case.resolved_at && <div>结案时间：{new Date(detail.case.resolved_at).toLocaleString()}</div>}
                </div>
                <div className="rounded-lg border p-4 text-sm">
                  <div className="font-medium">触发原因</div>
                  <div className="mt-2 text-muted-foreground">{detail.case.trigger_reason}</div>
                </div>
                {detail.case.investigation_report && (
                  <div className="rounded-lg border p-4 text-sm">
                    <div className="font-medium">调查报告</div>
                    <div className="mt-2 whitespace-pre-wrap text-muted-foreground">{detail.case.investigation_report}</div>
                  </div>
                )}
                {detail.case.merchant_rectification_report && (
                  <div className="rounded-lg border p-4 text-sm">
                    <div className="font-medium">商户整改报告</div>
                    <div className="mt-2 whitespace-pre-wrap text-muted-foreground">{detail.case.merchant_rectification_report}</div>
                  </div>
                )}
                {detail.case.resolution && (
                  <div className="rounded-lg border p-4 text-sm">
                    <div className="font-medium">处置结论</div>
                    <div className="mt-2 whitespace-pre-wrap text-muted-foreground">{detail.case.resolution}</div>
                  </div>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>关联顾客上报</CardTitle>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>事件ID</TableHead>
                      <TableHead>订单ID</TableHead>
                      <TableHead>用户ID</TableHead>
                      <TableHead>类型</TableHead>
                      <TableHead>状态</TableHead>
                      <TableHead>上报时间</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {detail.incidents.map((item) => (
                      <TableRow key={item.id}>
                        <TableCell>{item.id}</TableCell>
                        <TableCell>{item.order_id}</TableCell>
                        <TableCell>{item.user_id}</TableCell>
                        <TableCell>{formatFoodSafetyIncidentType(item.incident_type)}</TableCell>
                        <TableCell>{formatSafetyStatus(item.status)}</TableCell>
                        <TableCell>{new Date(item.created_at).toLocaleString()}</TableCell>
                      </TableRow>
                    ))}
                    {detail.incidents.length === 0 && (
                      <TableRow>
                        <TableCell colSpan={6} className="text-muted-foreground">
                          暂无关联上报事件
                        </TableCell>
                      </TableRow>
                    )}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>

            {detail.case.status !== "resolved" && (
              <Card>
                <CardHeader>
                  <CardTitle>调查与结案</CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid gap-3">
                    <Textarea
                      value={investigationReport}
                      onChange={(event) => setInvestigationReport(event.target.value)}
                      placeholder="填写调查报告，说明现场核查、样本判断、同批次排查等结论"
                      className="min-h-32"
                    />
                    {detail.case.status === "merchant-suspended" && (
                      <div className="flex justify-end">
                        <Button onClick={() => setInvestigateConfirmOpen(true)} disabled={investigating}>
                          {investigating ? "提交中" : "提交调查报告"}
                        </Button>
                      </div>
                    )}
                  </div>

                  <div className="grid gap-3 rounded-lg border p-4">
                    <div className="text-sm font-medium">结案恢复</div>
                    <Textarea
                      value={merchantRectificationReport}
                      onChange={(event) => setMerchantRectificationReport(event.target.value)}
                      placeholder="填写商户整改报告，例如后厨整改、原料更换、制度复核等内容"
                      className="min-h-28"
                    />
                    <Textarea
                      value={resolution}
                      onChange={(event) => setResolution(event.target.value)}
                      placeholder="填写处置结论，例如监管上报情况、复核结果与恢复依据"
                      className="min-h-24"
                    />
                    <div className="flex justify-end">
                      <Button onClick={() => setResolveConfirmOpen(true)} disabled={resolving}>
                        {resolving ? "提交中" : "完成结案并恢复商户"}
                      </Button>
                    </div>
                  </div>
                </CardContent>
              </Card>
            )}
          </>
        )}

        <ConfirmDialog
          open={investigateConfirmOpen}
          onOpenChange={setInvestigateConfirmOpen}
          title="确认提交调查报告？"
          description="提交后案件会进入调查中状态，并保留当前报告内容作为案件调查记录。"
          confirmText="确认提交"
          onConfirm={submitInvestigation}
        />

        <ConfirmDialog
          open={resolveConfirmOpen}
          onOpenChange={setResolveConfirmOpen}
          title="确认完成结案并恢复商户？"
          description="结案后将同步关闭关联食安事件，并恢复商户营业资格。请确认调查与整改材料已完整。"
          confirmText="确认结案"
          onConfirm={submitResolution}
        />
      </PageContent>
    </PageShell>
  );
}
