"use client";

import { useEffect, useMemo, useState } from "react";
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
  PageContent,
  PageHeader,
  PageShell,
} from "@/components/merchant/layout/page-shell";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiGet, apiPatch, apiPost } from "@/lib/api";
import type {
  PlatformProfitSharingConfigItem,
  PlatformProfitSharingConfigListResponse,
} from "@/types/platform-profit-sharing";

type LoadState = "loading" | "loaded" | "error";

function formatDateTime(value?: string) {
  if (!value) return "-";
  return new Date(value).toLocaleString("zh-CN");
}

function formatConfigStatus(status: string) {
  switch (status) {
    case "active":
      return "生效中";
    case "disabled":
      return "已禁用";
    case "draft":
      return "草稿";
    default:
      return "未知状态";
  }
}

export default function PlatformProfitSharingPage() {
  const [configs, setConfigs] = useState<PlatformProfitSharingConfigItem[]>([]);
  const [loadState, setLoadState] = useState<LoadState>("loading");
  const [platformRateInput, setPlatformRateInput] = useState("");
  const [operatorRateInput, setOperatorRateInput] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const load = () => {
    setLoadState("loading");
    setError(null);
    apiGet<PlatformProfitSharingConfigListResponse>("/platform/profit-sharing/configs", {
      status: "active",
      order_source: "all",
      page: 1,
      limit: 50,
    })
      .then((res) => {
        const items = res.items ?? [];
        setConfigs(items);
        setLoadState("loaded");

        const globalConfig = items.find((item) => !item.region_id && !item.merchant_id);
        if (globalConfig) {
          setPlatformRateInput(String(globalConfig.platform_rate));
          setOperatorRateInput(String(globalConfig.operator_rate));
        }
      })
      .catch((err: unknown) => {
        setLoadState("error");
        setError(err instanceof Error ? err.message : "加载失败");
      });
  };

  useEffect(() => {
    load();
  }, []);

  const globalConfig = useMemo(
    () => configs.find((item) => !item.region_id && !item.merchant_id),
    [configs]
  );

  const save = async () => {
    const platformRate = Number(platformRateInput);
    const operatorRate = Number(operatorRateInput);

    if (!Number.isFinite(platformRate) || !Number.isFinite(operatorRate)) {
      setError("请输入有效的费率数字");
      return;
    }
    if (platformRate < 0 || platformRate > 100 || operatorRate < 0 || operatorRate > 100) {
      setError("平台费率与运营商费率必须在 0-100 之间");
      return;
    }
    if (platformRate + operatorRate > 100) {
      setError("平台费率与运营商费率之和不能超过 100");
      return;
    }

    setSubmitting(true);
    setError(null);
    setSuccess(null);
    try {
      if (globalConfig) {
        await apiPatch<PlatformProfitSharingConfigItem>(
          `/platform/profit-sharing/configs/${globalConfig.id}`,
          {
            platform_rate: Math.round(platformRate),
            operator_rate: Math.round(operatorRate),
            status: "active",
            order_source: "all",
          }
        );
      } else {
        await apiPost<PlatformProfitSharingConfigItem>("/platform/profit-sharing/configs", {
          status: "active",
          order_source: "all",
          platform_rate: Math.round(platformRate),
          operator_rate: Math.round(operatorRate),
          rider_enabled: true,
          priority: 100,
        });
      }

      setSuccess("分账比例配置已保存");
      load();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "保存失败");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <PageShell>
      <PageHeader
        title="分账比例配置"
        description="平台与运营商佣金比例配置（全局生效）"
        actions={<Badge variant="secondary">平台侧</Badge>}
      />
      <PageContent className="space-y-4">
        {error && (
          <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
            {error}
          </div>
        )}
        {success && (
          <div className="rounded-lg border border-emerald-300 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
            {success}
          </div>
        )}

        <Card>
          <CardHeader>
            <CardTitle>当前全局比例</CardTitle>
            <CardDescription>用于外卖分账的默认平台/运营商比例</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4 md:grid-cols-3">
            <div>
              <div className="mb-2 text-sm text-muted-foreground">平台费率 (%)</div>
              <Input
                value={platformRateInput}
                onChange={(event) => setPlatformRateInput(event.target.value)}
                placeholder="例如 2"
              />
            </div>
            <div>
              <div className="mb-2 text-sm text-muted-foreground">运营商费率 (%)</div>
              <Input
                value={operatorRateInput}
                onChange={(event) => setOperatorRateInput(event.target.value)}
                placeholder="例如 3"
              />
            </div>
            <div className="flex items-end">
              <Button onClick={save} disabled={submitting || loadState === "loading"}>
                {submitting ? "保存中..." : "保存配置"}
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>配置列表</CardTitle>
            <CardDescription>展示已生效的全局分账配置</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>ID</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>订单来源</TableHead>
                  <TableHead>平台费率</TableHead>
                  <TableHead>运营商费率</TableHead>
                  <TableHead>更新时间</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loadState === "loading" && (
                  <TableRow>
                    <TableCell colSpan={6} className="text-sm text-muted-foreground">
                      加载中...
                    </TableCell>
                  </TableRow>
                )}
                {loadState !== "loading" && configs.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={6} className="text-sm text-muted-foreground">
                      暂无配置
                    </TableCell>
                  </TableRow>
                )}
                {configs
                  .filter((item) => !item.region_id && !item.merchant_id)
                  .map((item) => (
                    <TableRow key={item.id}>
                      <TableCell>{item.id}</TableCell>
                      <TableCell>{formatConfigStatus(item.status)}</TableCell>
                      <TableCell>{item.order_source}</TableCell>
                      <TableCell>{item.platform_rate}</TableCell>
                      <TableCell>{item.operator_rate}</TableCell>
                      <TableCell>{formatDateTime(item.updated_at)}</TableCell>
                    </TableRow>
                  ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </PageContent>
    </PageShell>
  );
}
