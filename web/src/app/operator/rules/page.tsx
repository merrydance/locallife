"use client";

import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  PageContent,
  PageHeader,
  PageShell,
} from "@/components/merchant/layout/page-shell";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiGet } from "@/lib/api";
import type { RuleSummary } from "@/types/platform-stats";

export default function OperatorRulesPage() {
  const [rules, setRules] = useState<RuleSummary[]>([]);
  const [loadState, setLoadState] = useState<"loading" | "loaded" | "error">("loading");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiGet<{ rules: RuleSummary[] }>("/operators/me/rules", { limit: 50, offset: 0 })
      .then((data) => {
        setRules(data?.rules ?? []);
        setLoadState("loaded");
      })
      .catch((err: unknown) => {
        const message = err instanceof Error ? err.message : "加载失败";
        setError(message);
        setLoadState("error");
      });
  }, []);

  const loading = loadState === "loading";

  return (
    <PageShell>
      <PageHeader
        title="规则配置"
        description="当前区域可见规则列表"
        actions={<Badge variant="secondary">运营商</Badge>}
      />
      <PageContent>
        {error && (
          <div className="mb-4 rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
            {error}
          </div>
        )}
        <Card>
          <CardHeader>
            <CardTitle>规则列表</CardTitle>
            <CardDescription>按区域过滤后的规则清单</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>规则名称</TableHead>
                  <TableHead>分类</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>更新时间</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading && (
                  <TableRow>
                    <TableCell colSpan={4} className="text-sm text-muted-foreground">
                      加载中...
                    </TableCell>
                  </TableRow>
                )}
                {!loading && rules.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={4} className="text-sm text-muted-foreground">
                      暂无规则
                    </TableCell>
                  </TableRow>
                )}
                {rules.map((rule) => (
                  <TableRow key={rule.id}>
                    <TableCell className="font-medium">{rule.name}</TableCell>
                    <TableCell>{rule.category}</TableCell>
                    <TableCell>
                      <Badge variant={rule.status === "active" ? "default" : "outline"}>
                        {rule.status === "active" ? "已生效" : rule.status}
                      </Badge>
                    </TableCell>
                    <TableCell>{new Date(rule.updated_at).toLocaleString("zh-CN")}</TableCell>
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
