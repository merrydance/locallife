"use client";

import { useEffect, useState, useCallback } from "react";
import Image from "next/image";
import { 
  Users, 
  GitPullRequest, 
  Settings2, 
  Plus, 
  Search,
  Loader2,
  RefreshCw,
  MapPin,
  Phone,
  LayoutDashboard,
  Store,
  Tag,
  AlertCircle,
  ChevronRight,
  Clock,
  Save
} from "lucide-react";
import { toast } from "sonner";
import { 
  apiGet, 
  apiPost, 
  apiPut
} from "@/lib/api";
import { getMediaDisplayUrl } from "@/lib/media";
import { PageShell, PageHeader, PageContent } from "@/components/merchant/layout/page-shell";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
  SheetFooter,
} from "@/components/ui/sheet";
import { 
  Dialog, 
  DialogContent, 
  DialogHeader, 
  DialogTitle, 
  DialogFooter,
  DialogDescription 
} from "@/components/ui/dialog";
import { 
  GroupResponse, 
  GroupMerchantResponse, 
  GroupJoinRequestResponse, 
  GroupPoliciesResponse,
  BrandResponse,
  GroupApplicationResponse
} from "@/types/group";
import { useMerchantSession } from "@/components/providers/merchant-session-provider";
import { cn } from "@/lib/utils";

export function GroupPageClient() {
  const session = useMerchantSession();
  const [activeTab, setActiveTab] = useState("overview");
  
  // Data States
  const [group, setGroup] = useState<GroupResponse | null>(null);
  const [merchants, setMerchants] = useState<GroupMerchantResponse[]>([]);
  const [joinRequests, setJoinRequests] = useState<GroupJoinRequestResponse[]>([]);
  const [brands, setBrands] = useState<BrandResponse[]>([]);
  const [policies, setPolicies] = useState<GroupPoliciesResponse | null>(null);
  const [application, setApplication] = useState<GroupApplicationResponse | null>(null);
  
  // Modals & Sheets
  const [brandModalOpen, setBrandModalOpen] = useState(false);
  const [editingBrand, setEditingBrand] = useState<BrandResponse | null>(null);
  const [merchantConfigOpen, setMerchantConfigOpen] = useState(false);
  const [editingMerchant, setEditingMerchant] = useState<GroupMerchantResponse | null>(null);
  type MerchantHour = {
    day_name?: string;
    day_of_week?: number;
    is_closed?: boolean;
    open_time?: string;
    close_time?: string;
  };
  const [merchantHours, setMerchantHours] = useState<MerchantHour[]>([]);
  const [savingConfig, setSavingConfig] = useState(false);
  
  // Loading States
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  
  const groupId = session?.merchant?.group_id;

  const fetchGroupData = useCallback(async () => {
    if (!groupId) {
      // Check for group application
      try {
        const app = await apiGet<GroupApplicationResponse>("/groups/applications/me");
        setApplication(app);
      } catch (err) {
        console.error("Failed to fetch group application", err);
      }
      setLoading(false);
      return;
    }

    setRefreshing(true);
    try {
      const [groupInfo, groupMerchants, groupRequests, groupPolicies, groupBrands] = await Promise.all([
        apiGet<GroupResponse>(`/groups/${groupId}`),
        apiGet<GroupMerchantResponse[]>(`/groups/${groupId}/merchants`),
        apiGet<GroupJoinRequestResponse[]>(`/groups/${groupId}/join-requests`).catch(() => []),
        apiGet<GroupPoliciesResponse>(`/groups/${groupId}/policies`).catch(() => null),
        apiGet<BrandResponse[]>(`/groups/${groupId}/brands`).catch(() => []),
      ]);
      
      setGroup(groupInfo);
      setMerchants(groupMerchants);
      setJoinRequests(groupRequests);
      setPolicies(groupPolicies);
      setBrands(groupBrands); 
      
    } catch (error) {
      console.error("Failed to fetch group data", error);
      toast.error("加载集团数据失败");
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, [groupId]);

  useEffect(() => {
    if (session?.isReady) {
      fetchGroupData();
    }
  }, [session?.isReady, fetchGroupData]);

  if (loading) {
    return (
      <PageShell>
        <div className="flex h-[60vh] items-center justify-center">
          <Loader2 className="h-8 w-8 animate-spin text-primary" />
        </div>
      </PageShell>
    );
  }

  // If not in a group and no application or rejected application
  if (!groupId && (!application || application.status === "rejected")) {
    return (
      <PageShell>
        <PageHeader 
          title="集团管理" 
          description="创建或加入集团以实现多门店协同管理"
        />
        <PageContent>
          <div className="max-w-4xl mx-auto grid grid-cols-1 md:grid-cols-2 gap-8 py-12">
            <Card className="border-2 hover:border-primary/50 transition-all cursor-pointer group">
              <CardHeader>
                <div className="w-12 h-12 bg-primary/10 rounded-xl flex items-center justify-center mb-2 group-hover:scale-110 transition-transform">
                  <Plus className="w-6 h-6 text-primary" />
                </div>
                <CardTitle>创建并入驻集团</CardTitle>
                <CardDescription>
                  如果您拥有多个品牌或门店，可以申请创建集团，统一查看经营数据、管理门店归属并维护协同策略。
                </CardDescription>
              </CardHeader>
              <CardContent>
                <Button className="w-full" onClick={() => toast.info("完善中...")}>开始申请</Button>
              </CardContent>
            </Card>

            <Card className="border-2 hover:border-primary/50 transition-all cursor-pointer group">
              <CardHeader>
                <div className="w-12 h-12 bg-blue-100 rounded-xl flex items-center justify-center mb-2 group-hover:scale-110 transition-transform">
                  <Search className="w-6 h-6 text-blue-600" />
                </div>
                <CardTitle>加入现有集团</CardTitle>
                <CardDescription>
                  如果您是连锁加盟店或希望加入某个集团，可以搜索集团并提交加入申请。
                </CardDescription>
              </CardHeader>
              <CardContent>
                <Button variant="outline" className="w-full" onClick={() => window.location.href='/merchant/settings?tab=group'}>前往搜索</Button>
              </CardContent>
            </Card>
          </div>
        </PageContent>
      </PageShell>
    );
  }

  // If application is pending
  if (!groupId && application && (application.status === "draft" || application.status === "submitted")) {
    return (
      <PageShell>
        <PageHeader title="集团入驻申请" description="您的集团入驻申请正在处理中" />
        <PageContent>
          <div className="max-w-2xl mx-auto bg-white rounded-xl border shadow-sm p-8 text-center space-y-6">
            <div className="mx-auto w-20 h-20 bg-amber-50 rounded-full flex items-center justify-center">
              <RefreshCw className="h-10 w-10 text-amber-500 animate-spin-slow" />
            </div>
            <div className="space-y-2">
              <h3 className="text-xl font-bold">申请审核中</h3>
              <p className="text-muted-foreground">
                您申请的集团：<strong>{application.group_name}</strong>
              </p>
              <Badge variant="secondary" className="bg-amber-100 text-amber-700">
                {application.status === "submitted" ? "已提交，等待审核" : "草稿状态"}
              </Badge>
            </div>
            <div className="pt-6 border-t text-sm text-muted-foreground">
              审核通常需要 1-3 个工作日。通过后，您将看到更加完整的集团管理视图。
            </div>
            {application.status === "draft" && (
              <Button onClick={() => toast.info("完善中...")}>继续完善资料</Button>
            )}
          </div>
        </PageContent>
      </PageShell>
    );
  }

  // Main Group Management View
  return (
    <PageShell>
      <PageHeader 
        title={group?.name || "集团管理"} 
        description="统一管理您的品牌、门店、政策和协同办公"
        actions={
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={fetchGroupData} disabled={refreshing}>
              <RefreshCw className={cn("h-4 w-4 mr-2", refreshing && "animate-spin")} />
              刷新数据
            </Button>
          </div>
        }
      />
      
      <PageContent>
        <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-6">
          <div className="flex flex-col md:flex-row gap-4 items-center justify-between bg-card p-2 rounded-xl border border-muted/50 shadow-sm">
            <TabsList className="bg-transparent border-none">
              <TabsTrigger value="overview" className="gap-2">
                <LayoutDashboard className="h-4 w-4" /> 概览
              </TabsTrigger>
              <TabsTrigger value="merchants" className="gap-2">
                <Store className="h-4 w-4" /> 门店列表
                {merchants.length > 0 && <Badge variant="secondary" className="ml-1 px-1.5 h-4 min-w-4 text-[10px]">{merchants.length}</Badge>}
              </TabsTrigger>
              <TabsTrigger value="requests" className="gap-2">
                <GitPullRequest className="h-4 w-4" /> 加入申请
                {joinRequests.filter(r => r.status === 'pending').length > 0 && (
                  <Badge variant="destructive" className="ml-1 px-1.5 h-4 min-w-4 text-[10px]">
                    {joinRequests.filter(r => r.status === 'pending').length}
                  </Badge>
                )}
              </TabsTrigger>
              <TabsTrigger value="brands" className="gap-2">
                <Tag className="h-4 w-4" /> 品牌库
              </TabsTrigger>
              <TabsTrigger value="policies" className="gap-2">
                <Settings2 className="h-4 w-4" /> 集团政策
              </TabsTrigger>
            </TabsList>
          </div>

          <TabsContent value="overview" className="m-0 space-y-6">
            <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
              <Card className="md:col-span-2">
                <CardHeader>
                  <CardTitle className="text-sm font-semibold border-l-4 border-primary pl-3">集团资料</CardTitle>
                </CardHeader>
                <CardContent className="grid grid-cols-1 sm:grid-cols-2 gap-6">
                  <div className="space-y-1">
                    <Label className="text-muted-foreground text-xs uppercase tracking-wider">集团名称</Label>
                    <p className="font-medium">{group?.name}</p>
                  </div>
                  <div className="space-y-1">
                    <Label className="text-muted-foreground text-xs uppercase tracking-wider">状态</Label>
                    <div>
                      <Badge variant={group?.status === 'active' ? "default" : "secondary"}>
                        {group?.status === 'active' ? "正常经营" : "非活跃"}
                      </Badge>
                    </div>
                  </div>
                  <div className="space-y-1">
                    <Label className="text-muted-foreground text-xs uppercase tracking-wider">联系电话</Label>
                    <div className="flex items-center gap-2">
                      <Phone className="h-3.5 w-3.5 text-slate-400" />
                      <p className="font-medium">{group?.contact_phone || "未设置"}</p>
                    </div>
                  </div>
                  <div className="space-y-1">
                    <Label className="text-muted-foreground text-xs uppercase tracking-wider">地址信息</Label>
                    <div className="flex items-center gap-2">
                      <MapPin className="h-3.5 w-3.5 text-slate-400" />
                      <p className="font-medium text-sm">{group?.address || "未设置"}</p>
                    </div>
                  </div>
                  <div className="space-y-1 sm:col-span-2">
                    <Label className="text-muted-foreground text-xs uppercase tracking-wider">统计概览</Label>
                    <div className="grid grid-cols-3 gap-4 pt-2">
                      <div className="bg-slate-50 p-3 rounded-lg border">
                        <p className="text-xs text-muted-foreground mb-1">直营/加盟店</p>
                        <p className="text-xl font-bold">{merchants.length}</p>
                      </div>
                      <div className="bg-slate-50 p-3 rounded-lg border">
                        <p className="text-xs text-muted-foreground mb-1">合作品牌</p>
                        <p className="text-xl font-bold">{brands.length}</p>
                      </div>
                      <div className="bg-slate-50 p-3 rounded-lg border">
                        <p className="text-xs text-muted-foreground mb-1">待审申请</p>
                        <p className="text-xl font-bold">{joinRequests.filter(r => r.status === 'pending').length}</p>
                      </div>
                    </div>
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle className="text-sm font-semibold border-l-4 border-primary pl-3">运营中心</CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <Button variant="outline" className="w-full justify-between group/btn" onClick={() => setActiveTab('merchants')}>
                    <span className="flex items-center"><Store className="h-4 w-4 mr-2 text-primary" /> 管理下属门店</span>
                    <ChevronRight className="h-4 w-4 text-muted-foreground group-hover/btn:translate-x-1 transition-transform" />
                  </Button>
                  <Button variant="outline" className="w-full justify-between group/btn" onClick={() => setActiveTab('requests')}>
                    <span className="flex items-center"><Users className="h-4 w-4 mr-2 text-blue-500" /> 处理加入申请</span>
                    <ChevronRight className="h-4 w-4 text-muted-foreground group-hover/btn:translate-x-1 transition-transform" />
                  </Button>
                  <Button variant="outline" className="w-full justify-between group/btn" onClick={() => setActiveTab('policies')}>
                    <span className="flex items-center"><Settings2 className="h-4 w-4 mr-2 text-amber-500" /> 配置全局政策</span>
                    <ChevronRight className="h-4 w-4 text-muted-foreground group-hover/btn:translate-x-1 transition-transform" />
                  </Button>
                  <Separator />
                  <div className="p-4 bg-primary/5 rounded-lg border border-primary/20 text-xs text-primary leading-relaxed flex items-start gap-2">
                    <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" />
                    <span> 您当前的身份为 <strong>集团管理员</strong>，可维护集团资料、门店归属和协同偏好；门店菜单、库存和营销执行仍以各自业务链路为准。</span>
                  </div>
                </CardContent>
              </Card>
            </div>
          </TabsContent>

          <TabsContent value="merchants" className="m-0 space-y-6">
            <div className="bg-white rounded-xl border shadow-sm">
              <div className="p-4 border-b flex items-center justify-between bg-white sticky top-0 z-10">
                <div>
                  <h3 className="text-sm font-semibold text-slate-900">门店列表</h3>
                  <p className="text-xs text-muted-foreground">管理集团下属的所有商户和门店</p>
                </div>
                <div className="flex gap-2">
                  <div className="relative">
                    <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                    <Input placeholder="搜索门店名称/地址..." className="pl-9 h-9 w-65" />
                  </div>
                  <Button size="sm">
                    <Plus className="h-4 w-4 mr-1" />
                    邀请新门店
                  </Button>
                </div>
              </div>
              <div className="divide-y">
                {merchants.map(merchant => (
                  <div key={merchant.id} className="p-4 flex items-center justify-between hover:bg-slate-50 transition-colors group">
                    <div className="flex items-center gap-4">
                      <div className="w-12 h-12 rounded-lg border overflow-hidden bg-slate-100 shrink-0">
                        {merchant.logo_url ? (
                          <Image src={getMediaDisplayUrl(merchant.logo_url)} alt={merchant.name} width={48} height={48} className="w-full h-full object-cover" />
                        ) : (
                          <div className="w-full h-full flex items-center justify-center text-slate-400">
                            <Store className="h-6 w-6" />
                          </div>
                        )}
                      </div>
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <h4 className="font-semibold text-slate-900 truncate">{merchant.name}</h4>
                          <Badge variant={merchant.status === 'active' ? "default" : "secondary"} className="h-5 text-[10px]">
                            {merchant.status === 'active' ? "正常" : "注销/冻结"}
                          </Badge>
                        </div>
                        <div className="flex items-center gap-3 mt-1 text-xs text-muted-foreground">
                          <span className="flex items-center gap-1"><MapPin className="h-3 w-3" /> {merchant.address}</span>
                          <span className="flex items-center gap-1"><Phone className="h-3 w-3" /> {merchant.phone}</span>
                        </div>
                      </div>
                    </div>
                    <div className="flex gap-2">
                      <Button variant="ghost" size="sm" onClick={() => toast.info("数据看板统计开发中")}>数据统计</Button>
                      <Button 
                        variant="outline" 
                        size="sm" 
                        onClick={async () => {
                          setEditingMerchant(merchant);
                          setMerchantConfigOpen(true);
                          setSavingConfig(true);
                          try {
                            const hours = await apiGet<{ hours: MerchantHour[] }>(`/merchants/${merchant.id}/business-hours`);
                            setMerchantHours(hours.hours || []);
                          } catch (error: unknown) {
                            const message = error instanceof Error ? error.message : "加载门店时间失败";
                            toast.error(message);
                          } finally {
                            setSavingConfig(false);
                          }
                        }}
                      >
                        配置项
                      </Button>
                    </div>
                  </div>
                ))}
                {merchants.length === 0 && (
                  <div className="py-20 text-center text-muted-foreground flex flex-col items-center">
                    <Store className="h-12 w-12 text-slate-200 mb-3" />
                    <p className="text-sm">暂无关联门店</p>
                  </div>
                )}
              </div>
            </div>
          </TabsContent>

          <TabsContent value="requests" className="m-0 space-y-6">
             <div className="bg-white rounded-xl border shadow-sm">
                <div className="p-4 border-b">
                  <h3 className="text-sm font-semibold text-slate-900">入驻申请管理</h3>
                  <p className="text-xs text-muted-foreground">审核来自外部门店的加入申请</p>
                </div>
                <div className="divide-y">
                  {joinRequests.map(req => (
                    <div key={req.id} className="p-6 flex flex-col gap-4">
                      <div className="flex items-start justify-between">
                        <div className="flex items-center gap-4">
                          <div className="w-10 h-10 bg-slate-100 rounded-full flex items-center justify-center">
                            <Users className="h-5 w-5 text-slate-500" />
                          </div>
                          <div>
                            <div className="flex items-center gap-3">
                              <h4 className="font-bold">店铺 ID: {req.merchant_id}</h4>
                              <Badge variant={req.status === 'pending' ? 'outline' : 'secondary'} className={cn(
                                req.status === 'pending' && "border-amber-500 text-amber-600 bg-amber-50",
                                req.status === 'approved' && "border-emerald-500 text-emerald-600 bg-emerald-50",
                                req.status === 'rejected' && "border-rose-500 text-rose-600 bg-rose-50",
                              )}>
                                {req.status === 'pending' ? '待审核' : 
                                 req.status === 'approved' ? '已通过' : 
                                 req.status === 'rejected' ? '已驳回' : '已撤回'}
                              </Badge>
                            </div>
                            <p className="text-xs text-muted-foreground mt-1">
                              申请人 ID: {req.applicant_user_id} · 提交于 {new Date(req.created_at).toLocaleString()}
                            </p>
                          </div>
                        </div>
                        {req.status === 'pending' && (
                          <div className="flex gap-2">
                            <Button variant="outline" size="sm" className="text-rose-600 border-rose-200 hover:bg-rose-50" onClick={() => handleReviewRequest(req, 'rejected')}>
                              驳回
                            </Button>
                            <Button size="sm" className="bg-emerald-600 hover:bg-emerald-700" onClick={() => handleReviewRequest(req, 'approved')}>
                              准许加入
                            </Button>
                          </div>
                        )}
                      </div>
                      <div className="bg-slate-50 p-3 rounded-lg border border-slate-100 text-sm italic text-slate-600">
                        &ldquo; {req.reason || "申请人未填写申请理由"} &rdquo;
                      </div>
                    </div>
                  ))}
                  {joinRequests.length === 0 && (
                    <div className="py-20 text-center text-muted-foreground flex flex-col items-center">
                      <GitPullRequest className="h-12 w-12 text-slate-200 mb-3" />
                      <p className="text-sm">暂无待处理申请</p>
                    </div>
                  )}
                </div>
             </div>
          </TabsContent>

          <TabsContent value="brands" className="m-0 space-y-6">
            <div className="bg-white rounded-xl border shadow-sm">
                <div className="p-4 border-b flex items-center justify-between">
                  <div>
                    <h3 className="text-sm font-semibold text-slate-900">品牌库</h3>
                    <p className="text-xs text-muted-foreground">维护集团旗下的子品牌和联名品牌</p>
                  </div>
                  <Button size="sm" onClick={() => {
                    setEditingBrand(null);
                    setBrandModalOpen(true);
                  }}>
                    <Plus className="h-4 w-4 mr-1" />
                    创建品牌
                  </Button>
                </div>
                <div className="p-6">
                  <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                    {brands.map(brand => (
                      <div key={brand.id} className="border rounded-xl p-4 space-y-3 hover:shadow-md transition-shadow">
                        <div className="flex items-center gap-3">
                          <div className="w-10 h-10 rounded-lg bg-slate-100 flex items-center justify-center border overflow-hidden">
                            {brand.logo_url ? (
                              <Image src={getMediaDisplayUrl(brand.logo_url)} alt={brand.name} width={40} height={40} className="w-full h-full object-cover" />
                            ) : (
                              <Tag className="h-5 w-5 text-slate-400" />
                            )}
                          </div>
                          <h4 className="font-bold">{brand.name}</h4>
                        </div>
                        <p className="text-xs text-muted-foreground line-clamp-2">{brand.description || "暂无描述"}</p>
                        <div className="pt-2 flex justify-end">
                           <Button variant="ghost" size="sm" onClick={() => {
                             setEditingBrand(brand);
                             setBrandModalOpen(true);
                           }}>编辑</Button>
                        </div>
                      </div>
                    ))}
                  </div>
                  {brands.length === 0 && (
                    <div className="py-12 border-2 border-dashed rounded-xl text-center text-muted-foreground">
                      暂无品牌信息，点击右上角添加。
                    </div>
                  )}
                </div>
            </div>
          </TabsContent>

          <TabsContent value="policies" className="m-0 space-y-6 animate-in fade-in slide-in-from-bottom-2">
            <Card className="overflow-hidden border-muted/60">
              <CardHeader className="bg-slate-50/50 border-b pb-6">
                <CardTitle className="text-sm font-semibold border-l-4 border-primary pl-3">集团协同偏好</CardTitle>
                <CardDescription className="pl-3 mt-1 underline-offset-4">记录集团与门店之间的管理偏好，当前主要用于组织协同与视图展示，暂不自动接管门店菜单、库存或营销执行</CardDescription>
              </CardHeader>
              <CardContent className="space-y-0 p-0">
                <div className="divide-y divide-slate-100">
                  <div className="p-6 flex items-center justify-between hover:bg-slate-50/30 transition-colors">
                    <div className="space-y-1">
                      <Label className="text-base font-bold text-slate-900">菜单协同偏好</Label>
                      <p className="text-xs text-muted-foreground font-normal max-w-lg">
                        <strong>集团主导：</strong> 记录集团希望统一维护菜单口径，便于后续人工协同；<br/>
                        <strong>门店主导：</strong> 记录各门店自行维护菜单信息的当前分工。
                      </p>
                    </div>
                    <div className="flex bg-slate-100 p-1 rounded-xl shadow-inner">
                      <Button variant={policies?.menu_mode === 'central' ? 'default' : 'ghost'} size="sm" className="h-8 text-xs px-4 rounded-lg" onClick={() => handleUpdatePolicy('menu_mode', 'central')}>集团主导</Button>
                      <Button variant={policies?.menu_mode === 'store' ? 'default' : 'ghost'} size="sm" className="h-8 text-xs px-4 rounded-lg" onClick={() => handleUpdatePolicy('menu_mode', 'store')}>门店主导</Button>
                    </div>
                  </div>

                  <div className="p-6 flex items-center justify-between hover:bg-slate-50/30 transition-colors">
                    <div className="space-y-1">
                      <Label className="text-base font-bold text-slate-900">价格管理偏好</Label>
                      <p className="text-xs text-muted-foreground font-normal max-w-lg">
                        <strong>集团主导：</strong> 记录集团希望统一价格口径，便于后续对齐和巡检；<br/>
                        <strong>门店主导：</strong> 记录门店可按本地经营情况独立定价的当前分工。
                      </p>
                    </div>
                    <div className="flex bg-slate-100 p-1 rounded-xl shadow-inner">
                      <Button variant={policies?.pricing_mode === 'central' ? 'default' : 'ghost'} size="sm" className="h-8 text-xs px-4 rounded-lg" onClick={() => handleUpdatePolicy('pricing_mode', 'central')}>集团主导</Button>
                      <Button variant={policies?.pricing_mode === 'store' ? 'default' : 'ghost'} size="sm" className="h-8 text-xs px-4 rounded-lg" onClick={() => handleUpdatePolicy('pricing_mode', 'store')}>门店主导</Button>
                    </div>
                  </div>

                  <div className="p-6 flex items-center justify-between hover:bg-slate-50/30 transition-colors">
                    <div className="space-y-1">
                      <Label className="text-base font-bold text-slate-900">库存协同偏好</Label>
                      <p className="text-xs text-muted-foreground font-normal max-w-lg">
                        <strong>集团协调：</strong> 记录集团参与库存预警、采购节奏或供应链协同的偏好；<br/>
                        <strong>门店自主：</strong> 记录门店独立管理库存的当前分工。
                      </p>
                    </div>
                    <div className="flex bg-slate-100 p-1 rounded-xl shadow-inner">
                      <Button variant={policies?.inventory_mode === 'central' ? 'default' : 'ghost'} size="sm" className="h-8 text-xs px-4 rounded-lg" onClick={() => handleUpdatePolicy('inventory_mode', 'central')}>集团协调</Button>
                      <Button variant={policies?.inventory_mode === 'store' ? 'default' : 'ghost'} size="sm" className="h-8 text-xs px-4 rounded-lg" onClick={() => handleUpdatePolicy('inventory_mode', 'store')}>门店自主</Button>
                    </div>
                  </div>
                </div>
                
                <div className="p-8 bg-slate-50 border-t flex justify-between items-center">
                  <div className="flex items-center gap-2 text-xs text-slate-400">
                    <AlertCircle className="h-4 w-4" />
                    保存后会更新集团协同偏好记录，供后续管理、审核与视图展示使用。
                  </div>
                  <Button size="lg" className="px-12 font-bold shadow-lg shadow-primary/20" onClick={handleSavePolicies}>保存协同偏好</Button>
                </div>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>

        {/* Brand Modal */}
        <Dialog open={brandModalOpen} onOpenChange={setBrandModalOpen}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{editingBrand ? "编辑品牌" : "创建新品牌"}</DialogTitle>
              <DialogDescription>为集团添加子品牌或关联品牌，以便分类管理门店。</DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="brand-name">品牌名称 *</Label>
                <Input id="brand-name" defaultValue={editingBrand?.name} placeholder="如：落落地茶饮" />
              </div>
              <div className="space-y-2">
                <Label htmlFor="brand-desc">品牌描述</Label>
                <Textarea id="brand-desc" defaultValue={editingBrand?.description || ""} placeholder="品牌的核心理念和简介..." />
              </div>
            </div>
            <DialogFooter>
              <Button variant="ghost" onClick={() => setBrandModalOpen(false)}>取消</Button>
              <Button onClick={() => {
                toast.info("功能完善中...");
                setBrandModalOpen(false);
              }}>保存品牌</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* Merchant Config Sheet */}
        <Sheet open={merchantConfigOpen} onOpenChange={setMerchantConfigOpen}>
          <SheetContent className="sm:max-w-xl">
            <SheetHeader>
              <SheetTitle>门店高级配置</SheetTitle>
              <SheetDescription>门店 ID: {editingMerchant?.id} - {editingMerchant?.name}</SheetDescription>
            </SheetHeader>
            
            <div className="py-6 space-y-8">
              <div className="space-y-4">
                <Label className="text-xs font-bold uppercase tracking-widest text-muted-foreground border-b pb-2 flex items-center gap-2">
                   <Clock className="h-4 w-4" /> 营业时间同步
                </Label>
                <div className="space-y-3">
                  {savingConfig ? (
                    <div className="flex items-center justify-center py-10">
                      <Loader2 className="h-6 w-6 animate-spin text-primary" />
                    </div>
                  ) : (
                    merchantHours.map((h, i) => (
                      <div key={i} className="flex items-center justify-between p-3 rounded-lg border bg-slate-50/50">
                        <span className="text-sm font-medium">{h.day_name || `周${h.day_of_week}`}</span>
                        <div className="flex items-center gap-2">
                           <Badge variant={h.is_closed ? "secondary" : "outline"} className="text-[10px] font-normal">
                             {h.is_closed ? "休息" : `${h.open_time} - ${h.close_time}`}
                           </Badge>
                        </div>
                      </div>
                    ))
                  )}
                  {merchantHours.length === 0 && !savingConfig && (
                    <p className="text-sm text-center text-muted-foreground py-10 italic">暂无营业时间数据</p>
                  )}
                </div>
              </div>

              <div className="space-y-4">
                <Label className="text-xs font-bold uppercase tracking-widest text-muted-foreground border-b pb-2 flex items-center gap-2">
                   <Settings2 className="h-4 w-4" /> 门店经营状态
                </Label>
                <div className="flex items-center justify-between p-4 rounded-xl border-2 border-primary/10 bg-primary/5">
                   <div className="space-y-0.5">
                     <p className="text-sm font-bold">手动营业/打烊</p>
                     <p className="text-xs text-muted-foreground">强制覆盖所有自动规则</p>
                   </div>
                   <Switch checked={editingMerchant?.status === 'active'} />
                </div>
              </div>
            </div>

            <SheetFooter>
              <Button size="lg" className="w-full font-bold" onClick={() => {
                toast.success("配置已同步");
                setMerchantConfigOpen(false);
              }}>
                <Save className="h-4 w-4 mr-2" />
                应用配置到门店
              </Button>
            </SheetFooter>
          </SheetContent>
        </Sheet>
      </PageContent>
    </PageShell>
  );

  async function handleReviewRequest(req: GroupJoinRequestResponse, status: 'approved' | 'rejected') {
    try {
      if (status === 'approved') {
        await apiPost(`/groups/${groupId}/join-requests/${req.id}/approve`);
        toast.success("已准许门店加入集团");
      } else {
        await apiPost(`/groups/${groupId}/join-requests/${req.id}/reject`, { reason: "集团管理员拒绝" });
        toast.success("已驳回申请");
      }
      fetchGroupData();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "操作失败";
      toast.error(message);
    }
  }

  function handleUpdatePolicy(key: keyof GroupPoliciesResponse, value: string) {
     if (!policies) return;
     setPolicies({ ...policies, [key]: value });
  }

  async function handleSavePolicies() {
    if (!policies || !groupId) return;
    try {
      await apiPut(`/groups/${groupId}/policies`, policies);
      toast.success("集团策略已更新");
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "更新政策失败";
      toast.error(message);
    }
  }
}
