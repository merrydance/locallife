"use client";

import { useCallback, useEffect, useState } from "react";
import { CheckCircle2, MapPin, Plus, RefreshCw, Search } from "lucide-react";
import { toast } from "sonner";
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
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  PageContent,
  PageHeader,
  PageShell,
} from "@/components/merchant/layout/page-shell";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { apiGet, apiPost } from "@/lib/api";
import type {
  OperatorRegionExpansionApplication,
  OperatorRegionExpansionResponse,
  OperatorRegionListResponse,
  RegionItem,
} from "@/types/operator-stats";

const STATUS_LABEL: Record<string, string> = {
  pending: "审核中",
  approved: "已通过",
  rejected: "已拒绝",
};

function statusBadge(status: string) {
  if (status === "pending") return <Badge>审核中</Badge>;
  if (status === "approved")
    return <Badge variant="secondary">已通过</Badge>;
  if (status === "rejected")
    return <Badge variant="destructive">已拒绝</Badge>;
  return <Badge variant="outline">{STATUS_LABEL[status] ?? status}</Badge>;
}

export default function OperatorRegionsPage() {
  // ── 管理区域 ──────────────────────────────────
  const [managedRegions, setManagedRegions] = useState<
    OperatorRegionListResponse["regions"]
  >([]);
  const [regionsLoading, setRegionsLoading] = useState(true);

  // ── 扩展申请列表 ──────────────────────────────
  const [applications, setApplications] = useState<
    OperatorRegionExpansionApplication[]
  >([]);
  const [appsLoading, setAppsLoading] = useState(true);

  // ── 申请新区域对话框 ──────────────────────────
  const [dialogOpen, setDialogOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  // 城市选项（level=2）
  const [cityOptions, setCityOptions] = useState<RegionItem[]>([]);
  const [selectedCityId, setSelectedCityId] = useState<number | null>(null);

  // 区县选项（level=3）
  const [districtOptions, setDistrictOptions] = useState<RegionItem[]>([]);
  const [filteredDistricts, setFilteredDistricts] = useState<RegionItem[]>([]);
  const [districtKeyword, setDistrictKeyword] = useState("");
  const [selectedDistrictId, setSelectedDistrictId] = useState<number | null>(
    null
  );
  const [districtLoading, setDistrictLoading] = useState(false);

  // ── 数据加载 ──────────────────────────────────
  const loadManagedRegions = useCallback(async () => {
    setRegionsLoading(true);
    try {
      const res = await apiGet<OperatorRegionListResponse>("/operator/regions", {
        page: 1,
        limit: 100,
      });
      setManagedRegions(res.regions ?? []);
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : "加载区域失败");
    } finally {
      setRegionsLoading(false);
    }
  }, []);

  const loadApplications = useCallback(async () => {
    setAppsLoading(true);
    try {
      const res = await apiGet<OperatorRegionExpansionResponse>(
        "/operator/region-expansion"
      );
      setApplications(res.applications ?? []);
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : "加载申请列表失败");
    } finally {
      setAppsLoading(false);
    }
  }, []);

  useEffect(() => {
    loadManagedRegions();
    loadApplications();
  }, [loadManagedRegions, loadApplications]);

  // ── 城市列表（打开对话框时加载一次）─────────────
  const loadCities = useCallback(async () => {
    if (cityOptions.length > 0) return;
    try {
      const all: RegionItem[] = [];
      let page = 1;
      for (;;) {
        const items = await apiGet<RegionItem[]>("/regions", {
          level: 2,
          page_id: page,
          page_size: 100,
        });
        if (!items || items.length === 0) break;
        all.push(...items);
        if (items.length < 100) break;
        page++;
      }
      setCityOptions(all);
      if (all.length > 0) {
        setSelectedCityId(all[0].id);
        loadDistricts(all[0].id);
      }
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : "城市列表加载失败");
    }
  }, [cityOptions.length]); // eslint-disable-line react-hooks/exhaustive-deps

  const loadDistricts = async (cityId: number) => {
    setDistrictLoading(true);
    setDistrictOptions([]);
    setFilteredDistricts([]);
    setSelectedDistrictId(null);
    setDistrictKeyword("");
    try {
      const all: RegionItem[] = [];
      let page = 1;
      for (;;) {
        const items = await apiGet<RegionItem[]>("/regions", {
          level: 3,
          parent_id: cityId,
          page_id: page,
          page_size: 100,
        });
        if (!items || items.length === 0) break;
        all.push(...items);
        if (items.length < 100) break;
        page++;
      }
      setDistrictOptions(all);
      setFilteredDistricts(all);
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : "区县列表加载失败");
    } finally {
      setDistrictLoading(false);
    }
  };

  const handleOpenDialog = () => {
    setDialogOpen(true);
    loadCities();
  };

  const handleCityChange = (value: string) => {
    const id = Number(value);
    setSelectedCityId(id);
    loadDistricts(id);
  };

  const handleDistrictSearch = (kw: string) => {
    setDistrictKeyword(kw);
    setFilteredDistricts(
      kw
        ? districtOptions.filter((d) => d.name.includes(kw))
        : districtOptions
    );
  };

  const handleSubmit = async () => {
    if (!selectedDistrictId) {
      toast.warning("请先选择目标区县");
      return;
    }
    setSubmitting(true);
    try {
      await apiPost("/operator/region-expansion", {
        region_id: selectedDistrictId,
      });
      toast.success("申请已提交，等待审核");
      setDialogOpen(false);
      setSelectedDistrictId(null);
      await loadApplications();
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : "提交失败");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <PageShell>
      <PageHeader
        title="区域管理"
        description="查看管理区域并申请扩展新区域"
        actions={
          <div className="flex gap-2">
            <Button
              variant="outline"
              onClick={() => {
                loadManagedRegions();
                loadApplications();
              }}
            >
              <RefreshCw className="mr-2 h-4 w-4" /> 刷新
            </Button>
            <Button onClick={handleOpenDialog}>
              <Plus className="mr-2 h-4 w-4" /> 申请新增区域
            </Button>
          </div>
        }
      />

      <PageContent className="space-y-4">
        <Tabs defaultValue="regions">
          <TabsList className="bg-slate-100 p-1 rounded-xl h-11">
            <TabsTrigger
              value="regions"
              className="rounded-lg px-6 font-bold transition-all data-[state=active]:bg-white data-[state=active]:text-foreground data-[state=active]:shadow-sm"
            >
              管理区域
              {!regionsLoading && managedRegions.length > 0 && (
                <Badge variant="secondary" className="ml-1.5 h-5 text-xs">
                  {managedRegions.length}
                </Badge>
              )}
            </TabsTrigger>
            <TabsTrigger
              value="applications"
              className="rounded-lg px-6 font-bold transition-all data-[state=active]:bg-white data-[state=active]:text-foreground data-[state=active]:shadow-sm"
            >
              扩展申请
              {!appsLoading && applications.length > 0 && (
                <Badge variant="secondary" className="ml-1.5 h-5 text-xs">
                  {applications.length}
                </Badge>
              )}
            </TabsTrigger>
          </TabsList>

          {/* 管理区域 */}
          <TabsContent value="regions" className="mt-4">
            <Card>
              <CardHeader>
                <CardTitle>当前管理的区域</CardTitle>
                <CardDescription>
                  这些是您已获权运营的区县，扩展申请通过后自动新增
                </CardDescription>
              </CardHeader>
              <CardContent>
                {regionsLoading ? (
                  <div className="space-y-2">
                    {Array.from({ length: 3 }).map((_, i) => (
                      <Skeleton key={i} className="h-12 w-full" />
                    ))}
                  </div>
                ) : managedRegions.length === 0 ? (
                  <div className="flex flex-col items-center gap-3 py-10 text-muted-foreground">
                    <MapPin className="h-10 w-10 opacity-30" />
                    <p className="text-sm">暂无管理区域</p>
                  </div>
                ) : (
                  <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                    {managedRegions.map((r) => (
                      <div
                        key={r.id}
                        className="flex items-center gap-3 rounded-lg border bg-muted/20 px-4 py-3"
                      >
                        <MapPin className="h-5 w-5 shrink-0 text-primary" />
                        <span className="font-medium">{r.name}</span>
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          {/* 扩展申请 */}
          <TabsContent value="applications" className="mt-4">
            <Card>
              <CardHeader>
                <CardTitle>区域扩展申请记录</CardTitle>
                <CardDescription>
                  申请通过后，对应区域将自动加入您的管理区域列表
                </CardDescription>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>区域名称</TableHead>
                      <TableHead>状态</TableHead>
                      <TableHead>申请时间</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {appsLoading ? (
                      Array.from({ length: 3 }).map((_, i) => (
                        <TableRow key={i}>
                          <TableCell colSpan={3}>
                            <Skeleton className="h-5 w-full" />
                          </TableCell>
                        </TableRow>
                      ))
                    ) : applications.length === 0 ? (
                      <TableRow>
                        <TableCell
                          colSpan={3}
                          className="text-center text-muted-foreground"
                        >
                          暂无申请记录
                        </TableCell>
                      </TableRow>
                    ) : (
                      applications.map((app) => (
                        <TableRow key={app.id}>
                          <TableCell className="font-medium">
                            {app.region_name}
                          </TableCell>
                          <TableCell>{statusBadge(app.status)}</TableCell>
                          <TableCell className="text-sm text-muted-foreground">
                            {new Date(app.created_at).toLocaleString("zh-CN")}
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </PageContent>

      {/* 申请新增区域对话框 */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>申请新增运营区域</DialogTitle>
            <DialogDescription>
              选择目标区县，提交后等待平台审核通过即可运营该区域
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            {/* 城市选择 */}
            <div className="space-y-1.5">
              <label className="text-sm font-medium">选择城市</label>
              <Select
                value={selectedCityId ? String(selectedCityId) : undefined}
                onValueChange={handleCityChange}
                disabled={cityOptions.length === 0}
              >
                <SelectTrigger>
                  <SelectValue placeholder="请选择城市" />
                </SelectTrigger>
                <SelectContent>
                  {cityOptions.map((c) => (
                    <SelectItem key={c.id} value={String(c.id)}>
                      {c.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {/* 区县搜索+列表 */}
            <div className="space-y-1.5">
              <label className="text-sm font-medium">选择区县</label>
              <div className="relative">
                <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  className="pl-9"
                  placeholder="搜索区县名称"
                  value={districtKeyword}
                  onChange={(e) => handleDistrictSearch(e.target.value)}
                  disabled={districtLoading}
                />
              </div>
              <ScrollArea className="h-52 rounded-md border">
                {districtLoading ? (
                  <div className="flex items-center justify-center h-full text-sm text-muted-foreground">
                    加载中...
                  </div>
                ) : filteredDistricts.length === 0 ? (
                  <div className="flex items-center justify-center h-full text-sm text-muted-foreground">
                    暂无区县数据
                  </div>
                ) : (
                  <div className="p-2 space-y-1">
                    {filteredDistricts.map((d) => (
                      <button
                        key={d.id}
                        onClick={() => setSelectedDistrictId(d.id)}
                        className={`w-full flex items-center justify-between rounded-md px-3 py-2 text-sm transition-colors hover:bg-muted ${
                          selectedDistrictId === d.id
                            ? "bg-primary/10 text-primary font-medium"
                            : ""
                        }`}
                      >
                        <span>{d.name}</span>
                        {selectedDistrictId === d.id && (
                          <CheckCircle2 className="h-4 w-4" />
                        )}
                      </button>
                    ))}
                  </div>
                )}
              </ScrollArea>
              {selectedDistrictId && (
                <p className="text-xs text-primary">
                  已选：
                  {
                    filteredDistricts.find((d) => d.id === selectedDistrictId)
                      ?.name
                  }
                </p>
              )}
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>
              取消
            </Button>
            <Button
              disabled={!selectedDistrictId || submitting}
              onClick={handleSubmit}
            >
              {submitting ? "提交中..." : "提交申请"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </PageShell>
  );
}
