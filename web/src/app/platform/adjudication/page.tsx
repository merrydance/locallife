"use client";

import { useEffect, useMemo, useState } from "react";
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
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { apiGet } from "@/lib/api";
import type { RuleHitRow, RuleSummary } from "@/types/platform-stats";

export default function PlatformAdjudicationPage() {
  const [rules, setRules] = useState<RuleSummary[]>([]);
  const [selectedRuleId, setSelectedRuleId] = useState<string>("");
  const [hits, setHits] = useState<RuleHitRow[] | null>(null);
  const [rulesState, setRulesState] = useState<"loading" | "loaded" | "error">("loading");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiGet<{ rules: RuleSummary[] }>("/platform/rules", { limit: 50, offset: 0 })
      .then((data) => {
        const claimRules = (data?.rules ?? []).filter(
          (rule) => rule.category === "claim"
        );
        setRules(claimRules);
        if (claimRules.length > 0) {
          setSelectedRuleId(String(claimRules[0].id));
        }
        setRulesState("loaded");
      })
      .catch((err: unknown) => {
        const message = err instanceof Error ? err.message : "加载失败";
        setError(message);
        setRulesState("error");
      });
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

  useEffect(() => {
    if (error) {
      toast.error(error);
    }
  }, [error]);

  const handleRuleChange = (value: string) => {
    setHits(null);
    setSelectedRuleId(value);
  };

  const selectedRule = useMemo(
    () => rules.find((rule) => String(rule.id) === selectedRuleId),
    [rules, selectedRuleId]
  );
  const loadingRules = rulesState === "loading";
  const loadingHits = selectedRuleId !== "" && hits === null;
  const displayHits = selectedRuleId ? hits ?? [] : [];

  return (
    <PageShell>
      <PageHeader
        title="异常裁决复核"
        description="平台抽样复核异常裁决结果"
        actions={<Badge variant="outline">抽样</Badge>}
      />
      <PageContent className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>裁决规则</CardTitle>
            <CardDescription>筛选规则后查看命中记录</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-3 sm:grid-cols-[220px_1fr] sm:items-center">
              <div className="text-sm text-muted-foreground">选择规则</div>
              <Select value={selectedRuleId} onValueChange={handleRuleChange}>
                <SelectTrigger>
                  <SelectValue placeholder="选择裁决规则" />
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

            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>规则名称</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>更新时间</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loadingRules && (
                  <TableRow>
                    <TableCell colSpan={3} className="text-sm text-muted-foreground">
                      加载中...
                    </TableCell>
                  </TableRow>
                )}
                {!loadingRules && rules.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={3} className="text-sm text-muted-foreground">
                      暂无裁决规则
                    </TableCell>
                  </TableRow>
                )}
                {rules.map((rule) => (
                  <TableRow key={rule.id}>
                    <TableCell className="font-medium">{rule.name}</TableCell>
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

        <Card>
          <CardHeader>
            <CardTitle>规则命中记录</CardTitle>
            <CardDescription>
              {selectedRule ? `最近命中 · ${selectedRule.name}` : "裁决规则命中记录"}
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
