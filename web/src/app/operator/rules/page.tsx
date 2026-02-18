"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { PageContent, PageHeader, PageShell } from "@/components/merchant/layout/page-shell";
import { apiGet, apiPatch } from "@/lib/api";
import type { OperatorRulesResponse } from "@/types/operator-console";
import type { OperatorRegionListResponse } from "@/types/operator-stats";

type Category = "delivery" | "timeslot" | "weather";

export default function OperatorRulesPage() {
  const [regions, setRegions] = useState<Array<{ id: number; name: string }>>([]);
  const [regionId, setRegionId] = useState<string>("all");
  const [activeCategory, setActiveCategory] = useState<Category>("delivery");
  const [rules, setRules] = useState<OperatorRulesResponse["rules"]>([]);
  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [editingValue, setEditingValue] = useState<string>("");
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  const loadRegions = useCallback(() => {
    apiGet<OperatorRegionListResponse>("/operator/regions", { page: 1, limit: 100 })
      .then((res) => setRegions(res.regions ?? []))
      .catch(() => setRegions([]));
  }, []);

  const loadRules = useCallback(() => {
    apiGet<OperatorRulesResponse>("/operator/rules", {
      region_id: regionId === "all" ? undefined : Number(regionId),
    })
      .then((res) => {
        setRules(res.rules ?? []);
        setError(null);
      })
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : "加载规则失败");
      });
  }, [regionId]);

  useEffect(() => {
    loadRegions();
  }, [loadRegions]);

  useEffect(() => {
    loadRules();
  }, [loadRules]);

  const categorized = useMemo(
    () => ({
      delivery: rules.filter((rule) => rule.category === "delivery"),
      timeslot: rules.filter((rule) => rule.category === "timeslot"),
      weather: rules.filter((rule) => rule.category === "weather"),
    }),
    [rules]
  );

  const onEdit = (key: string, value: string) => {
    setEditingKey(key);
    setEditingValue(value);
    setMessage(null);
  };

  const onSave = async () => {
    if (!editingKey) return;

    await apiPatch(
      `/operator/rules/${editingKey}${regionId === "all" ? "" : `?region_id=${regionId}`}`,
      { value: editingValue }
    );

    setEditingKey(null);
    setEditingValue("");
    setMessage("规则已更新");
    loadRules();
  };

  const activeRules = categorized[activeCategory];

  return (
    <PageShell>
      <PageHeader
        title="规则配置"
        description="按区域管理配送规则与天气系数"
        actions={<Badge variant="secondary">运营商</Badge>}
      />
      <PageContent className="space-y-4">
        <Card>
          <CardHeader>
            <CardTitle>筛选与分类</CardTitle>
            <CardDescription>选择生效区域并切换规则类别</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex flex-col gap-3 md:flex-row md:items-center">
              <Select value={regionId} onValueChange={setRegionId}>
                <SelectTrigger className="w-full md:w-72">
                  <SelectValue placeholder="选择区域" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">默认区域（当前运营商主区域）</SelectItem>
                  {regions.map((region) => (
                    <SelectItem key={region.id} value={String(region.id)}>
                      {region.name}（{region.id}）
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button variant="outline" onClick={loadRules}>
                重新加载
              </Button>
            </div>

            <Tabs value={activeCategory} onValueChange={(val) => setActiveCategory(val as Category)}>
              <TabsList className="w-full md:w-auto">
                <TabsTrigger value="delivery" className="md:min-w-24">
                  运费参数
                </TabsTrigger>
                <TabsTrigger value="timeslot" className="md:min-w-24">
                  时段系数
                </TabsTrigger>
                <TabsTrigger value="weather" className="md:min-w-24">
                  天气系数
                </TabsTrigger>
              </TabsList>
            </Tabs>
          </CardContent>
        </Card>

        {error && (
          <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
            {error}
          </div>
        )}
        {message && (
          <div className="rounded-lg border border-emerald-300 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
            {message}
          </div>
        )}

        <Card>
          <CardHeader>
            <CardTitle>规则列表</CardTitle>
            <CardDescription>仅展示当前分类下可见规则，可编辑项可直接修改</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>规则项</TableHead>
                  <TableHead>当前值</TableHead>
                  <TableHead>说明</TableHead>
                  <TableHead>操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {activeRules.map((rule) => {
                  const editing = editingKey === rule.key;
                  return (
                    <TableRow key={rule.id}>
                      <TableCell>
                        <div className="font-medium">{rule.name}</div>
                      </TableCell>
                      <TableCell>
                        {editing ? (
                          <div className="flex items-center gap-2">
                            <Input
                              value={editingValue}
                              onChange={(e) => setEditingValue(e.target.value)}
                              className="w-32"
                            />
                            <span className="text-xs text-muted-foreground">{rule.unit}</span>
                          </div>
                        ) : (
                          <span>
                            {rule.value}
                            {rule.unit ? ` ${rule.unit}` : ""}
                          </span>
                        )}
                      </TableCell>
                      <TableCell className="text-sm text-muted-foreground">{rule.desc}</TableCell>
                      <TableCell>
                        {rule.editable ? (
                          editing ? (
                            <div className="flex gap-2">
                              <Button size="sm" onClick={onSave}>
                                保存
                              </Button>
                              <Button
                                size="sm"
                                variant="outline"
                                onClick={() => {
                                  setEditingKey(null);
                                  setEditingValue("");
                                }}
                              >
                                取消
                              </Button>
                            </div>
                          ) : rule.action === "navigate_peak" ? (
                            <Button size="sm" variant="outline" onClick={() => (window.location.href = "/operator/peak-hours")}>
                              管理
                            </Button>
                          ) : (
                            <Button size="sm" variant="outline" onClick={() => onEdit(rule.key, rule.value)}>
                              修改
                            </Button>
                          )
                        ) : (
                          <Badge variant="outline">平台维护</Badge>
                        )}
                      </TableCell>
                    </TableRow>
                  );
                })}
                {activeRules.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={4} className="text-muted-foreground">
                      该分类暂无规则
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </PageContent>
    </PageShell>
  );
}
