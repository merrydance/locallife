"use client";

import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { PageContent, PageHeader, PageShell } from "@/components/merchant/layout/page-shell";
import { apiDelete, apiGet, apiPost } from "@/lib/api";
import { formatWeekdays, getActiveOperatorRegions } from "@/lib/operator-display";
import type { OperatorRegionListResponse } from "@/types/operator-stats";
import type { PeakHourConfigResponse } from "@/types/operator-console";

export default function OperatorPeakHoursPage() {
  const timeOptions = [
    "00:00", "00:30", "01:00", "01:30", "02:00", "02:30", "03:00", "03:30",
    "04:00", "04:30", "05:00", "05:30", "06:00", "06:30", "07:00", "07:30",
    "08:00", "08:30", "09:00", "09:30", "10:00", "10:30", "11:00", "11:30",
    "12:00", "12:30", "13:00", "13:30", "14:00", "14:30", "15:00", "15:30",
    "16:00", "16:30", "17:00", "17:30", "18:00", "18:30", "19:00", "19:30",
    "20:00", "20:30", "21:00", "21:30", "22:00", "22:30", "23:00", "23:30",
  ] as const;

  const weekdayOptions: Array<{ value: string; label: string; days: number[] }> = [
    { value: "workdays", label: "工作日（周一到周五）", days: [1, 2, 3, 4, 5] },
    { value: "weekend", label: "周末（周六、周日）", days: [6, 0] },
    { value: "all", label: "每天", days: [0, 1, 2, 3, 4, 5, 6] },
    { value: "mon-fri-sat", label: "周一到周六", days: [1, 2, 3, 4, 5, 6] },
    { value: "sun-thu", label: "周日到周四", days: [0, 1, 2, 3, 4] },
  ];

  const [regionId, setRegionId] = useState<number | null>(null);
  const [items, setItems] = useState<PeakHourConfigResponse[]>([]);
  const [error, setError] = useState<string | null>(null);

  const [startTime, setStartTime] = useState("11:00");
  const [endTime, setEndTime] = useState("13:00");
  const [coefficient, setCoefficient] = useState("1.2");
  const [weekdayPreset, setWeekdayPreset] = useState("workdays");

  useEffect(() => {
    apiGet<OperatorRegionListResponse>("/operator/regions", { page: 1, limit: 100 })
      .then((res) => {
        const id = getActiveOperatorRegions(res.regions)?.[0]?.id;
        if (!id) throw new Error("未找到运营中的管理区域");
        setRegionId(id);
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : "加载区域失败"));
  }, []);

  const load = useCallback((id: number) => {
    apiGet<PeakHourConfigResponse[]>(`/operator/regions/${id}/peak-hours`)
      .then((res) => {
        setItems(res ?? []);
        setError(null);
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : "加载失败"));
  }, []);

  useEffect(() => {
    if (!regionId) return;
    load(regionId);
  }, [regionId, load]);

  useEffect(() => {
    if (error) {
      toast.error(error);
    }
  }, [error]);

  const createConfig = async () => {
    if (!regionId) return;
    const selectedWeekday = weekdayOptions.find((option) => option.value === weekdayPreset);
    const parsedDays = selectedWeekday?.days ?? [1, 2, 3, 4, 5];

    await apiPost(`/operator/regions/${regionId}/peak-hours`, {
      region_id: regionId,
      start_time: startTime,
      end_time: endTime,
      coefficient: Number(coefficient),
      days_of_week: parsedDays,
    });
    load(regionId);
  };

  const remove = async (id: number) => {
    await apiDelete(`/operator/peak-hours/${id}`);
    if (regionId) load(regionId);
  };

  return (
    <PageShell>
      <PageHeader
        title="高峰时段"
        description="配置区域代取高峰时段系数"
        actions={<Badge variant="secondary">运营商</Badge>}
      />
      <PageContent className="space-y-4">
        <Card>
          <CardHeader>
            <CardTitle>新增时段配置</CardTitle>
          </CardHeader>
          <CardContent className="grid gap-3 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="peak-start-time">开始时间</Label>
              <Select value={startTime} onValueChange={setStartTime}>
                <SelectTrigger id="peak-start-time">
                  <SelectValue placeholder="选择开始时间" />
                </SelectTrigger>
                <SelectContent>
                  {timeOptions.map((option) => (
                    <SelectItem key={option} value={option}>
                      {option}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="peak-end-time">结束时间</Label>
              <Select value={endTime} onValueChange={setEndTime}>
                <SelectTrigger id="peak-end-time">
                  <SelectValue placeholder="选择结束时间" />
                </SelectTrigger>
                <SelectContent>
                  {timeOptions.map((option) => (
                    <SelectItem key={option} value={option}>
                      {option}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="peak-coefficient">高峰系数</Label>
              <Input
                id="peak-coefficient"
                value={coefficient}
                onChange={(e) => setCoefficient(e.target.value)}
                placeholder="大于 1，例如 1.2"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="peak-days">生效星期</Label>
              <Select value={weekdayPreset} onValueChange={setWeekdayPreset}>
                <SelectTrigger id="peak-days">
                  <SelectValue placeholder="选择生效星期" />
                </SelectTrigger>
                <SelectContent>
                  {weekdayOptions.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="md:col-span-2">
              <Button onClick={createConfig} disabled={!regionId}>
                创建配置
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>配置列表</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>ID</TableHead>
                  <TableHead>时段</TableHead>
                  <TableHead>系数</TableHead>
                  <TableHead>星期</TableHead>
                  <TableHead>操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell>{item.id}</TableCell>
                    <TableCell>
                      {item.start_time} - {item.end_time}
                    </TableCell>
                    <TableCell>{item.coefficient}</TableCell>
                    <TableCell>{formatWeekdays(item.days_of_week)}</TableCell>
                    <TableCell>
                      <Button variant="destructive" size="sm" onClick={() => remove(item.id)}>
                        删除
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
                {items.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={5} className="text-muted-foreground">
                      暂无配置
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
