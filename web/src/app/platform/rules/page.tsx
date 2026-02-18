"use client";

import { useEffect, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";
import { toast } from "sonner";
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
import { Separator } from "@/components/ui/separator";
import { apiGet } from "@/lib/api";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { RuleHitRow, RuleSummary } from "@/types/platform-stats";

export default function PlatformRulesPage() {
  const searchParams = useSearchParams();
  const [rules, setRules] = useState<RuleSummary[]>([]);
  const [selectedRuleId, setSelectedRuleId] = useState<string>("");
  const [hits, setHits] = useState<RuleHitRow[] | null>(null);
  const [rulesState, setRulesState] = useState<"loading" | "loaded" | "error">("loading");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiGet<{ rules: RuleSummary[] }>("/platform/rules", { limit: 20, offset: 0 })
      .then((data) => {
        const list = data?.rules ?? [];
        setRules(list);
        if (list.length > 0) {
          setSelectedRuleId(String(list[0].id));
        }
        setRulesState("loaded");
      })
      .catch((err: unknown) => {
        const message = err instanceof Error ? err.message : "加载失败";
        setError(message);
        setRulesState("error");
      })
  }, []);

  useEffect(() => {
    if (!selectedRuleId) return;
    apiGet<{ hits: RuleHitRow[] }>("/platform/rules/hits", {
      rule_id: selectedRuleId,
      limit: 20,
      offset: 0,
    })
      .then((data) => setHits(data?.hits ?? []))
      .catch(() => setHits([]));
  }, [selectedRuleId]);
  const handleRuleChange = (value: string) => {
    setHits(null);
    setSelectedRuleId(value);
  };

  const loadingRules = rulesState === "loading";
  const loadingHits = selectedRuleId !== "" && hits === null;

  const selectedRule = useMemo(
    () => rules.find((rule) => String(rule.id) === selectedRuleId),
    [rules, selectedRuleId]
  );
  const displayHits = selectedRuleId ? hits ?? [] : [];

  useEffect(() => {
    const view = searchParams?.get("view");
    if (view !== "hits") return;
    const timer = window.setTimeout(() => {
      document.getElementById("hits")?.scrollIntoView({ behavior: "smooth", block: "start" });
    }, 120);
    return () => window.clearTimeout(timer);
  }, [searchParams, selectedRuleId, hits]);

  useEffect(() => {
    if (error) {
      toast.error(error);
    }
  }, [error]);

  return (
    <PageShell>
      <PageHeader
        title="规则变更与生效可视化"
        description="规则发布、灰度、生效与回滚记录"
        actions={<Badge variant="secondary">规则中心</Badge>}
      />
      <PageContent className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>规则列表</CardTitle>
            <CardDescription>规则发布与生效状态概览</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-3 sm:grid-cols-[220px_1fr] sm:items-center">
              <div className="text-sm text-muted-foreground">选择规则</div>
              <Select value={selectedRuleId} onValueChange={handleRuleChange}>
                <SelectTrigger>
                  <SelectValue placeholder="选择规则" />
                </SelectTrigger>
                <SelectContent>
                  {rules.map((rule) => (
                    <SelectItem key={rule.id} value={String(rule.id)}>
                      {rule.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <Separator />

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
                {loadingRules && (
                  <TableRow>
                    <TableCell colSpan={4} className="text-sm text-muted-foreground">
                      加载中...
                    </TableCell>
                  </TableRow>
                )}
                {!loadingRules && rules.length === 0 && (
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

        <Card id="hits">
          <CardHeader>
            <CardTitle>规则命中记录</CardTitle>
            <CardDescription>
              {selectedRule
                ? `最近命中记录 · ${selectedRule.name}`
                : "基于规则命中审计的记录"}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>时间</TableHead>
                  <TableHead>领域</TableHead>
                  <TableHead>裁决</TableHead>
                  <TableHead>角色</TableHead>
                  <TableHead>区域</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loadingHits && (
                  <TableRow>
                    <TableCell colSpan={5} className="text-sm text-muted-foreground">
                      加载中...
                    </TableCell>
                  </TableRow>
                )}
                {!loadingHits && displayHits.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={5} className="text-sm text-muted-foreground">
                      暂无命中记录
                    </TableCell>
                  </TableRow>
                )}
                {displayHits.map((hit) => (
                  <TableRow key={hit.id}>
                    <TableCell className="font-medium">
                      {new Date(hit.created_at).toLocaleString("zh-CN")}
                    </TableCell>
                    <TableCell>{hit.domain}</TableCell>
                    <TableCell>{hit.decision}</TableCell>
                    <TableCell>{hit.actor_role ?? "-"}</TableCell>
                    <TableCell>{hit.region_id ?? "-"}</TableCell>
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
