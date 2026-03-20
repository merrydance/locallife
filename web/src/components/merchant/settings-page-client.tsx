"use client";

import React, { useEffect, useState, useCallback } from "react";
import Image from "next/image";
import { 
  Building2, 
  Clock, 
  Printer, 
  Settings2, 
  Save, 
  Plus, 
  Trash2, 
  Edit, 
  MapPin, 
  Phone, 
  Image as ImageIcon,
  Loader2,
  RefreshCw,
  CheckCircle2,
  AlertCircle,
  XCircle,
  Tag,
  Info
} from "lucide-react";
import { toast } from "sonner";
import { 
  apiGet, 
  apiPost, 
  apiPatch,
  apiPut, 
  apiDelete, 
  getMediaUrl
} from "@/lib/api";
import { uploadMedia } from "@/lib/media";
import { useMerchantSession } from "@/components/providers/merchant-session-provider";
import { PageShell, PageHeader, PageContent } from "@/components/merchant/layout/page-shell";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { 
  Dialog, 
  DialogContent, 
  DialogHeader, 
  DialogTitle, 
  DialogFooter,
  DialogDescription 
} from "@/components/ui/dialog";
import { 
  Select, 
  SelectContent, 
  SelectItem, 
  SelectTrigger, 
  SelectValue 
} from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { cn } from "@/lib/utils";
import type { 
  MerchantProfile, 
  BusinessHour, 
  CloudPrinter, 
  DisplayConfig,
  Group
} from "@/types/merchant-settings";

export function MerchantSettingsPageClient() {
  const [activeTab, setActiveTab] = useState("profile");
  
  // Data States
  const [profile, setProfile] = useState<MerchantProfile | null>(null);
  const [businessHours, setBusinessHours] = useState<BusinessHour[]>([]);
  const [printers, setPrinters] = useState<CloudPrinter[]>([]);
  const [displayConfig, setDisplayConfig] = useState<DisplayConfig | null>(null);
  
  const session = useMerchantSession();

  // Loading States
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  
  // Modal States
  const [printerModalOpen, setPrinterModalOpen] = useState(false);
  const [editingPrinter, setEditingPrinter] = useState<Partial<CloudPrinter> | null>(null);
  const [deletePrinterConfirm, setDeletePrinterConfirm] = useState<number | null>(null);

  // Category Tag States
  const [availableMerchantTags, setAvailableMerchantTags] = useState<{ id: number; name: string }[]>([]);
  const [selectedTagIds, setSelectedTagIds] = useState<number[]>([]);
  const [savingTags, setSavingTags] = useState(false);

  // Group States
  const [groupKeyword, setGroupKeyword] = useState("");
  const [groupResults, setGroupResults] = useState<Group[]>([]);
  const [searchingGroups, setSearchingGroups] = useState(false);
  const [currentGroup, setCurrentGroup] = useState<Group | null>(null);
  const [joinReasonModal, setJoinReasonModal] = useState<Group | null>(null);
  const [joinReason, setJoinReason] = useState("");
  
  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [profileData, hoursData, printersData, configData, currentTagsData, allTagsData] = await Promise.all([
        apiGet<MerchantProfile>("/merchants/me"),
        apiGet<{ hours: BusinessHour[] }>("/merchants/me/business-hours"),
        apiGet<{ printers: CloudPrinter[] }>("/merchant/devices"),
        apiGet<DisplayConfig>("/merchant/display-config").catch(() => null),
        apiGet<{ tags: { id: number; name: string }[] }>("/merchants/me/tags").catch(() => ({ tags: [] })),
        apiGet<{ tags: { id: number; name: string }[] }>("/tags", { type: "merchant" }).catch(() => ({ tags: [] }))
      ]);
      
      setProfile(profileData);
      setBusinessHours(hoursData.hours || []);
      setPrinters(printersData.printers || []);
      setSelectedTagIds((currentTagsData.tags || []).map(t => t.id));
      setAvailableMerchantTags(allTagsData.tags || []);
      setDisplayConfig(configData || {
        enable_print: true,
        print_takeout: true,
        print_dine_in: true,
        print_reservation: true,
        enable_voice: false,
        voice_takeout: true,
        voice_dine_in: true,
        enable_kds: false
      });

      // If in a group, fetch group details
      if (profileData.group_id) {
        apiGet<Group>(`/groups/${profileData.group_id}`)
          .then(setCurrentGroup)
          .catch(err => console.error("Failed to fetch group info", err));
      }
    } catch (error) {
      console.error("Failed to fetch settings", error);
      toast.error("加载设置失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const handleSaveProfile = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!profile) return;
    
    setSaving(true);
    try {
      const updated = await apiPatch<MerchantProfile>("/merchants/me", {
        name: profile.name,
        description: profile.description,
        phone: profile.phone,
        address: profile.address,
        latitude: profile.latitude,
        longitude: profile.longitude,
        version: profile.version
      });
      setProfile(updated);
      toast.success("商户资料已更新");
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "保存失败";
      toast.error(message);
    } finally {
      setSaving(false);
    }
  };

  const handleToggleOpen = async (isOpen: boolean) => {
    if (!profile) return;
    try {
      if (session) {
        await session.setOpen(isOpen);
      } else {
        await apiPatch("/merchants/me/status", { is_open: isOpen });
      }
      setProfile({ ...profile, is_open: isOpen });
      toast.success(isOpen ? "店铺已营业" : "店铺已打烊");
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "更新状态失败";
      toast.error(message);
    }
  };

  const handleLogoUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file || !profile) return;

    toast.loading("上传中...", { id: "upload" });
    try {
      const { mediaId, urls } = await uploadMedia(file, {
        businessType: "merchant",
        mediaCategory: "logo",
      });

      // 立即保存 logo_asset_id 到后端（使用当前 version）
      const updated = await apiPatch<MerchantProfile>("/merchants/me", {
        logo_asset_id: mediaId,
        version: profile.version,
      });
      // 用返回的最新 profile（包含新 version 和新 logo_url）更新本地状态
      setProfile({ ...updated, logo_url: urls["card"] ?? urls["original"] ?? updated.logo_url });
      toast.success("上传成功", { id: "upload" });
    } catch {
      toast.error("上传失败", { id: "upload" });
    }
  };

  const handleSaveDisplayConfig = async () => {
    if (!displayConfig) return;
    setSaving(true);
    try {
      await apiPut("/merchant/display-config", displayConfig);
      toast.success("显示配置已保存");
    } catch {
      toast.error("保存失败");
    } finally {
      setSaving(false);
    }
  };

  const handleSaveBusinessHours = async () => {
    setSaving(true);
    try {
      const hoursToSave = businessHours.map(h => ({
        day_of_week: h.day_of_week,
        open_time: h.open_time,
        close_time: h.close_time,
        is_closed: h.is_closed,
        special_date: h.special_date || undefined
      }));
      
      const response = await apiPut<{ hours: BusinessHour[] }>("/merchants/me/business-hours", { 
        hours: hoursToSave 
      });
      setBusinessHours(response.hours || []);
      toast.success("营业时间已更新");
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "更新营业时间失败";
      toast.error(message);
    } finally {
      setSaving(false);
    }
  };

  const handleAddPrinter = () => {
    setEditingPrinter({
      printer_name: "",
      printer_sn: "",
      printer_key: "",
      printer_type: "feieyun",
      print_takeout: true,
      print_dine_in: true,
      print_reservation: true,
      is_active: true
    });
    setPrinterModalOpen(true);
  };

  const handleEditPrinter = (printer: CloudPrinter) => {
    setEditingPrinter(printer);
    setPrinterModalOpen(true);
  };

  const handleSavePrinter = async () => {
    if (!editingPrinter) return;
    
    setSaving(true);
    try {
      if (editingPrinter.id) {
        await apiPut(`/merchant/devices/${editingPrinter.id}`, editingPrinter);
        toast.success("打印机已更新");
      } else {
        await apiPost("/merchant/devices", editingPrinter);
        toast.success("打印机已添加");
      }
      setPrinterModalOpen(false);
      const printersData = await apiGet<{ printers: CloudPrinter[] }>("/merchant/devices");
      setPrinters(printersData.printers || []);
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "保存失败";
      toast.error(message);
    } finally {
      setSaving(false);
    }
  };

  const handleDeletePrinter = async () => {
    if (!deletePrinterConfirm) return;
    try {
      await apiDelete(`/merchant/devices/${deletePrinterConfirm}`);
      toast.success("打印机已删除");
      setPrinters(printers.filter(p => p.id !== deletePrinterConfirm));
    } catch {
      toast.error("删除失败");
    } finally {
      setDeletePrinterConfirm(null);
    }
  };

  const handleTogglePrinter = async (id: number, isActive: boolean) => {
    try {
      await apiPut(`/merchant/devices/${id}`, { is_active: isActive });
      setPrinters(printers.map(p => p.id === id ? { ...p, is_active: isActive } : p));
      toast.success(isActive ? "已启用" : "已禁用");
    } catch {
      toast.error("操作失败");
    }
  };

  const handleSaveMerchantTags = async () => {
    setSavingTags(true);
    try {
      const updated = await apiPut<{ tags: { id: number; name: string }[] }>("/merchants/me/tags", {
        tag_ids: selectedTagIds
      });
      setSelectedTagIds((updated.tags || []).map(t => t.id));
      toast.success("经营类目已保存");
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "保存失败";
      toast.error(message);
    } finally {
      setSavingTags(false);
    }
  };

  const handleSearchGroups = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!groupKeyword.trim()) return;
    
    setSearchingGroups(true);
    try {
      const results = await apiGet<{ groups: Group[] }>("/groups", { 
        keyword: groupKeyword 
      });
      setGroupResults(results.groups || []);
      if ((results.groups || []).length === 0) {
        toast.info("未找到匹配的集团");
      }
    } catch {
      toast.error("搜索集团失败");
    } finally {
      setSearchingGroups(false);
    }
  };

  const handleJoinGroup = async () => {
    if (!joinReasonModal) return;
    
    setSaving(true);
    try {
      await apiPost(`/groups/${joinReasonModal.id}/join-requests`, { 
        reason: joinReason 
      });
      toast.success("申请已提交，请等待集团审核");
      setJoinReasonModal(null);
      setJoinReason("");
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "操作失败";
      toast.error(message);
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <PageShell>
        <div className="flex h-[60vh] items-center justify-center">
          <Loader2 className="h-8 w-8 animate-spin text-primary" />
        </div>
      </PageShell>
    );
  }

  return (
    <PageShell>
      <PageHeader 
        title="店铺设置" 
        description="管理店铺基础信息、营业时间及硬件设施"
        actions={
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={fetchData}>
              <RefreshCw className="h-4 w-4 mr-2" />
              刷新
            </Button>
          </div>
        }
      />
      
      <PageContent>
        <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-6">
          <div className="flex flex-col md:flex-row gap-4 items-center justify-between bg-card p-2 rounded-xl border border-muted/50 shadow-sm">
            <TabsList className="bg-transparent border-none">
              <TabsTrigger value="profile" className="data-[state=active]:bg-white data-[state=active]:text-foreground data-[state=active]:shadow-sm">
                <Building2 className="h-4 w-4 mr-2" />
                店铺详情
              </TabsTrigger>
              <TabsTrigger value="business" className="data-[state=active]:bg-white data-[state=active]:text-foreground data-[state=active]:shadow-sm">
                <Clock className="h-4 w-4 mr-2" />
                营业管理
              </TabsTrigger>
              <TabsTrigger value="devices" className="data-[state=active]:bg-white data-[state=active]:text-foreground data-[state=active]:shadow-sm">
                <Printer className="h-4 w-4 mr-2" />
                设备连接
              </TabsTrigger>
              <TabsTrigger value="display" className="data-[state=active]:bg-white data-[state=active]:text-foreground data-[state=active]:shadow-sm">
                <Settings2 className="h-4 w-4 mr-2" />
                展示设置
              </TabsTrigger>
              <TabsTrigger value="group" className="data-[state=active]:bg-white data-[state=active]:text-foreground data-[state=active]:shadow-sm">
                <Building2 className="h-4 w-4 mr-2" />
                加入集团
              </TabsTrigger>
            </TabsList>
          </div>

          <TabsContent value="profile" className="space-y-6 m-0">
            {/* 经营类目提示横幅（未选时警示，已选时正常） */}
            {availableMerchantTags.length > 0 && selectedTagIds.length === 0 && (
              <div className="flex items-start gap-3 rounded-xl border border-amber-300 bg-amber-50 px-5 py-4">
                <AlertCircle className="h-5 w-5 text-amber-600 mt-0.5 shrink-0" />
                <div>
                  <p className="text-sm font-semibold text-amber-800">您还未选择经营类目</p>
                  <p className="text-xs text-amber-700 mt-0.5">经营类目决定您的店铺出现在首页哪些分类筛选标签下，直接影响顾客发现您的概率。请在下方完成设置。</p>
                </div>
              </div>
            )}
            <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
              <div className="lg:col-span-2 space-y-6">
                <div className="bg-white rounded-xl border shadow-sm">
                  <div className="p-4 border-b">
                    <h3 className="text-sm font-semibold text-slate-900 border-l-4 border-primary pl-3">
                      基本信息
                    </h3>
                  </div>
                  <div className="p-6">
                    <form onSubmit={handleSaveProfile} className="space-y-6">
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                        <div className="space-y-2">
                          <Label htmlFor="name">店铺名称 <span className="text-destructive">*</span></Label>
                          <Input 
                            id="name" 
                            value={profile?.name || ""} 
                            onChange={e => profile && setProfile({...profile, name: e.target.value})}
                            placeholder="输入店铺名称"
                            required
                          />
                        </div>
                        <div className="space-y-2">
                          <Label htmlFor="phone">联系电话 <span className="text-destructive">*</span></Label>
                          <div className="relative">
                            <Phone className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                            <Input 
                              id="phone" 
                              className="pl-9"
                              value={profile?.phone || ""} 
                              onChange={e => profile && setProfile({...profile, phone: e.target.value})}
                              placeholder="输入手机号"
                              required
                            />
                          </div>
                        </div>
                        <div className="md:col-span-2 space-y-2">
                          <Label htmlFor="address">店铺详情地址 <span className="text-destructive">*</span></Label>
                          <div className="relative">
                            <MapPin className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                            <Input 
                              id="address" 
                              className="pl-9"
                              value={profile?.address || ""} 
                              onChange={e => profile && setProfile({...profile, address: e.target.value})}
                              placeholder="选择或输入详细地址"
                              required
                            />
                          </div>
                        </div>
                        <div className="md:col-span-2 space-y-2">
                          <Label htmlFor="description">店铺简介</Label>
                          <Textarea 
                            id="description" 
                            className="min-h-25"
                            value={profile?.description || ""} 
                            onChange={e => profile && setProfile({...profile, description: e.target.value})}
                            placeholder="向顾客介绍一下您的店铺..."
                          />
                        </div>
                        <div className="space-y-2">
                          <Label>经度</Label>
                          <Input value={profile?.longitude || ""} readOnly className="bg-slate-50" />
                        </div>
                        <div className="space-y-2">
                          <Label>纬度</Label>
                          <Input value={profile?.latitude || ""} readOnly className="bg-slate-50" />
                        </div>
                      </div>
                      
                      <div className="pt-4 border-t flex justify-end">
                        <Button type="submit" disabled={saving}>
                          {saving ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : <Save className="h-4 w-4 mr-2" />}
                          保存更改
                        </Button>
                      </div>
                    </form>
                  </div>
                </div>
              </div>

              <div className="space-y-6">
                {/* ====== 经营类目 ====== */}
                <div className="bg-white rounded-xl border shadow-sm">
                  <div className="p-4 border-b flex items-center gap-2">
                    <h3 className="text-sm font-semibold text-slate-900 border-l-4 border-primary pl-3">
                      经营类目
                    </h3>
                    {selectedTagIds.length > 0 && (
                      <span className="ml-auto text-[10px] bg-emerald-100 text-emerald-700 font-medium px-2 py-0.5 rounded-full">已设置</span>
                    )}
                  </div>
                  <div className="p-5 space-y-4">
                    <div className="flex items-start gap-2 rounded-lg bg-blue-50 border border-blue-200 px-3 py-2.5">
                      <Info className="h-3.5 w-3.5 text-blue-600 mt-0.5 shrink-0" />
                      <p className="text-[11px] text-blue-700 leading-relaxed">
                        类目标签决定您的店铺出现在首页「餐厅」频道哪些筛选分类下，<strong>直接影响曝光量和排名</strong>。建议选择 1-3 个最能代表您店铺的类目。
                      </p>
                    </div>
                    {availableMerchantTags.length === 0 ? (
                      <p className="text-xs text-muted-foreground text-center py-4">暂无可选类目，请联系平台添加</p>
                    ) : (
                      <div className="flex flex-wrap gap-2">
                        {availableMerchantTags.map(tag => {
                          const selected = selectedTagIds.includes(tag.id);
                          return (
                            <button
                              key={tag.id}
                              type="button"
                              onClick={() => {
                                if (selected) {
                                  setSelectedTagIds(selectedTagIds.filter(id => id !== tag.id));
                                } else {
                                  if (selectedTagIds.length >= 5) {
                                    toast.warning("最多选 5 个类目");
                                    return;
                                  }
                                  setSelectedTagIds([...selectedTagIds, tag.id]);
                                }
                              }}
                              className={cn(
                                "inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium border transition-all",
                                selected
                                  ? "bg-primary text-primary-foreground border-primary"
                                  : "bg-white text-slate-600 border-slate-200 hover:border-primary/60 hover:text-primary"
                              )}
                            >
                              <Tag className="h-3 w-3" />
                              {tag.name}
                            </button>
                          );
                        })}
                      </div>
                    )}
                    <div className="pt-1 flex justify-between items-center">
                      <span className="text-[11px] text-muted-foreground">已选 {selectedTagIds.length}/5</span>
                      <button
                        type="button"
                        onClick={handleSaveMerchantTags}
                        disabled={savingTags}
                        className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-primary text-primary-foreground hover:bg-primary/90 disabled:opacity-50 transition-colors"
                      >
                        {savingTags ? <Loader2 className="h-3 w-3 animate-spin" /> : <Save className="h-3 w-3" />}
                        保存类目
                      </button>
                    </div>
                  </div>
                </div>

                <div className="bg-white rounded-xl border shadow-sm">
                  <div className="p-4 border-b">
                    <h3 className="text-sm font-semibold text-slate-900 border-l-4 border-primary pl-3">
                      店铺 Logo
                    </h3>
                  </div>
                  <div className="p-6 flex flex-col items-center gap-6">
                    <div className="relative w-32 h-32 rounded-xl border-2 border-dashed border-slate-200 flex items-center justify-center overflow-hidden group">
                      {profile?.logo_url ? (
                        <Image 
                          src={getMediaUrl(profile.logo_url)} 
                          alt="店铺 Logo" 
                          width={128}
                          height={128}
                          className="w-full h-full object-cover"
                        />
                      ) : (
                        <ImageIcon className="h-10 w-10 text-slate-300" />
                      )}
                      <label className="absolute inset-0 bg-black/50 text-white flex flex-col items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer">
                        <ImageIcon className="h-6 w-6 mb-1" />
                        <span className="text-[10px]">更换图片</span>
                        <input type="file" className="hidden" accept="image/*" onChange={handleLogoUpload} />
                      </label>
                    </div>
                    <div className="text-center space-y-1">
                      <p className="text-sm font-medium">建议尺寸 800x800 px</p>
                      <p className="text-xs text-muted-foreground">支持 JPG, PNG 格式，小于 2MB</p>
                    </div>
                  </div>
                </div>

                <div className="bg-white rounded-xl border shadow-sm overflow-hidden">
                  <div className="p-4 border-b">
                    <h3 className="text-sm font-semibold text-slate-900 border-l-4 border-primary pl-3">
                      快捷状态
                    </h3>
                  </div>
                  <div className="p-6 space-y-6">
                    <div className="flex items-center justify-between">
                      <div className="space-y-1">
                        <p className="text-sm font-medium">当前营业状态</p>
                        <p className="text-xs text-muted-foreground">
                          {profile?.is_open ? "客人们现在可以看到并下单" : "店铺目前暂不接受新订单"}
                        </p>
                      </div>
                      <Switch 
                        checked={profile?.is_open} 
                        onCheckedChange={handleToggleOpen}
                      />
                    </div>
                    <div className={cn(
                      "p-3 rounded-lg flex items-center gap-3",
                      profile?.is_open ? "bg-emerald-50 text-emerald-700" : "bg-rose-50 text-rose-700"
                    )}>
                      {profile?.is_open ? <CheckCircle2 className="h-5 w-5" /> : <AlertCircle className="h-5 w-5" />}
                      <span className="text-sm font-medium">{profile?.is_open ? "营业中" : "打烊中"}</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </TabsContent>

          <TabsContent value="business" className="space-y-6 m-0">
            <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
              <div className="lg:col-span-2 space-y-6">
                <Card>
                  <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4 border-b">
                    <div>
                      <CardTitle className="text-sm font-semibold border-l-4 border-primary pl-3">每周常规营业时间</CardTitle>
                      <CardDescription className="pl-3 mt-1">设置每周一至周日的固定营业时间段</CardDescription>
                    </div>
                    <Button variant="outline" size="sm" onClick={() => {
                      const firstDay = businessHours.find(h => !h.is_closed && !h.special_date);
                      if (!firstDay) {
                        toast.error("请先设置至少一个营业的时段");
                        return;
                      }
                      const next = businessHours.map(h => h.special_date ? h : {
                         ...h, 
                         open_time: firstDay.open_time, 
                         close_time: firstDay.close_time, 
                         is_closed: firstDay.is_closed 
                      });
                      setBusinessHours(next);
                      toast.success("已同步至所有工作日 (未保存)");
                    }}>
                      同步到所有天
                    </Button>
                  </CardHeader>
                  <CardContent className="p-0">
                    <div className="divide-y divide-slate-100">
                      {[1, 2, 3, 4, 5, 6, 0].map((dayNum) => {
                        const daySlots = businessHours.filter(h => h.day_of_week === dayNum && !h.special_date);
                        const dayName = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"][dayNum];
                        const isClosed = daySlots.some(s => s.is_closed) || daySlots.length === 0;

                        return (
                          <div key={dayNum} className="p-6 flex flex-col md:flex-row gap-6 hover:bg-slate-50/50 transition-colors">
                            <div className="md:w-32 flex flex-row md:flex-col items-center md:items-start justify-between md:justify-start gap-2">
                              <span className="font-bold text-slate-900">{dayName}</span>
                              <div className="flex items-center gap-2">
                                <Switch 
                                  checked={!isClosed}
                                  onCheckedChange={(val) => {
                                    if (!val) {
                                      // 设置为休息
                                      const otherDays = businessHours.filter(h => h.day_of_week !== dayNum || h.special_date);
                                      setBusinessHours([...otherDays, {
                                        day_of_week: dayNum,
                                        day_name: dayName,
                                        open_time: "09:00",
                                        close_time: "21:00",
                                        is_closed: true
                                      }]);
                                    } else {
                                      // 设置为营业
                                      const otherDays = businessHours.filter(h => h.day_of_week !== dayNum || h.special_date);
                                      setBusinessHours([...otherDays, {
                                        day_of_week: dayNum,
                                        day_name: dayName,
                                        open_time: "09:00",
                                        close_time: "21:00",
                                        is_closed: false
                                      }]);
                                    }
                                  }}
                                />
                                <Badge variant={isClosed ? "secondary" : "default"} className="text-[10px] px-1.5 py-0 h-5">
                                  {isClosed ? "休息" : "营业"}
                                </Badge>
                              </div>
                            </div>

                            <div className="flex-1 space-y-3">
                              {!isClosed && daySlots.map((slot, sIdx) => (
                                <div key={sIdx} className="flex items-center gap-3 animate-in fade-in slide-in-from-left-2">
                                  <div className="flex items-center bg-white border rounded-md px-2 h-9">
                                    <Clock className="h-3 w-3 text-slate-400 mr-2" />
                                    <input 
                                      type="time" 
                                      className="border-none bg-transparent text-sm focus:ring-0 w-24 p-0"
                                      value={slot.open_time}
                                      onChange={(e) => {
                                        const next = [...businessHours];
                                        const globalIdx = businessHours.indexOf(slot);
                                        next[globalIdx] = { ...slot, open_time: e.target.value };
                                        setBusinessHours(next);
                                      }}
                                    />
                                  </div>
                                  <span className="text-slate-400">至</span>
                                  <div className="flex items-center bg-white border rounded-md px-2 h-9">
                                    <Clock className="h-3 w-3 text-slate-400 mr-2" />
                                    <input 
                                      type="time" 
                                      className="border-none bg-transparent text-sm focus:ring-0 w-24 p-0"
                                      value={slot.close_time}
                                      onChange={(e) => {
                                        const next = [...businessHours];
                                        const globalIdx = businessHours.indexOf(slot);
                                        next[globalIdx] = { ...slot, close_time: e.target.value };
                                        setBusinessHours(next);
                                      }}
                                    />
                                  </div>
                                  {daySlots.length > 1 && (
                                    <Button 
                                      variant="ghost" 
                                      size="icon" 
                                      className="h-8 w-8 text-rose-500"
                                      onClick={() => {
                                        setBusinessHours(businessHours.filter(h => h !== slot));
                                      }}
                                    >
                                      <Trash2 className="h-4 w-4" />
                                    </Button>
                                  )}
                                </div>
                              ))}
                              {!isClosed && (
                                <Button 
                                  variant="ghost" 
                                  size="sm" 
                                  className="text-primary hover:text-primary/80 hover:bg-primary/5 h-8 px-2"
                                  onClick={() => {
                                    setBusinessHours([...businessHours, {
                                      day_of_week: dayNum,
                                      day_name: dayName,
                                      open_time: "17:00",
                                      close_time: "21:00",
                                      is_closed: false
                                    }]);
                                  }}
                                >
                                  <Plus className="h-3 w-3 mr-1" />
                                  添加时段
                                </Button>
                              )}
                              {isClosed && <p className="text-sm text-slate-400 italic">本日全天不营业</p>}
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </CardContent>
                  <div className="p-6 border-t flex justify-end gap-3 bg-slate-50/30">
                    <Button variant="outline" onClick={fetchData}>重置更改</Button>
                    <Button onClick={handleSaveBusinessHours} disabled={saving}>
                      {saving ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : <Save className="h-4 w-4 mr-2" />}
                      保存常规时间
                    </Button>
                  </div>
                </Card>
              </div>

              <div className="space-y-6">
                <Card>
                  <CardHeader className="border-b">
                    <CardTitle className="text-sm font-semibold border-l-4 border-amber-500 pl-3">特殊日期 (节假日/调休)</CardTitle>
                    <CardDescription className="pl-3 mt-1">为特定日期单独配置营业时间，优先级高于常规时间</CardDescription>
                  </CardHeader>
                  <CardContent className="p-6 space-y-4">
                    <div className="space-y-4">
                      {businessHours.filter(h => h.special_date).map((hour, idx) => (
                        <Card key={idx} className="border-amber-100 bg-amber-50/30 overflow-hidden">
                          <div className="p-3 bg-amber-100/50 flex items-center justify-between">
                            <span className="text-xs font-bold text-amber-900">{hour.special_date}</span>
                            <Button 
                              variant="ghost" 
                              size="icon" 
                              className="h-6 w-6 text-amber-700 hover:bg-amber-200"
                              onClick={() => setBusinessHours(businessHours.filter(h => h !== hour))}
                            >
                              <XCircle className="h-4 w-4" />
                            </Button>
                          </div>
                          <div className="p-4 space-y-3">
                            <div className="flex items-center justify-between">
                              <Label className="text-xs font-medium">是否营业</Label>
                              <Switch 
                                checked={!hour.is_closed} 
                                onCheckedChange={(val) => {
                                  const next = [...businessHours];
                                  const globalIdx = businessHours.indexOf(hour);
                                  next[globalIdx] = { ...hour, is_closed: !val };
                                  setBusinessHours(next);
                                }}
                              />
                            </div>
                            {!hour.is_closed && (
                              <div className="flex items-center gap-2">
                                <Input 
                                  type="time" 
                                  className="h-8 text-xs px-2" 
                                  value={hour.open_time}
                                  onChange={(e) => {
                                    const next = [...businessHours];
                                    const globalIdx = businessHours.indexOf(hour);
                                    next[globalIdx] = { ...hour, open_time: e.target.value };
                                    setBusinessHours(next);
                                  }}
                                />
                                <span className="text-[10px] text-slate-400">-</span>
                                <Input 
                                  type="time" 
                                  className="h-8 text-xs px-2" 
                                  value={hour.close_time}
                                  onChange={(e) => {
                                    const next = [...businessHours];
                                    const globalIdx = businessHours.indexOf(hour);
                                    next[globalIdx] = { ...hour, close_time: e.target.value };
                                    setBusinessHours(next);
                                  }}
                                />
                              </div>
                            )}
                            {hour.is_closed && <p className="text-xs text-amber-700 italic">本日全天不营业</p>}
                          </div>
                        </Card>
                      ))}

                      <Button 
                        variant="outline" 
                        className="w-full border-dashed border-2 h-12"
                        onClick={() => {
                          const date = prompt("请输入日期 (YYYY-MM-DD)", new Date().toISOString().split('T')[0]);
                          if (date && /^\d{4}-\d{2}-\d{2}$/.test(date)) {
                            const d = new Date(date);
                            setBusinessHours([...businessHours, {
                              day_of_week: d.getDay(),
                              day_name: ["周日", "周一", "周二", "周三", "周四", "周五", "周六"][d.getDay()],
                              open_time: "09:00",
                              close_time: "21:00",
                              is_closed: false,
                              special_date: date
                            }]);
                          } else if (date) {
                            toast.error("日期格式不正确");
                          }
                        }}
                      >
                        <Plus className="h-4 w-4 mr-2" />
                        添加特殊日期
                      </Button>
                    </div>
                  </CardContent>
                </Card>

                <div className="bg-primary/5 rounded-xl border border-primary/20 p-6 space-y-4">
                  <div className="flex items-center gap-3 text-primary">
                    <div className="w-8 h-8 rounded-full bg-primary/10 flex items-center justify-center">
                      <AlertCircle className="h-4 w-4" />
                    </div>
                    <span className="text-sm font-bold">后台管理小贴士</span>
                  </div>
                  <ul className="text-xs text-slate-600 space-y-2 leading-relaxed list-disc pl-4">
                    <li>特殊日期的设置会覆盖对应日期的常规营业时间。</li>
                    <li>如果某个时间段重叠，系统将以营业的时段为准。</li>
                    <li>设置修改后，请务必点击“保存设置”以同步到云端。</li>
                    <li>更改营业时间可能会影响到已有的外卖订单和预订。</li>
                  </ul>
                </div>
              </div>
            </div>
          </TabsContent>

          <TabsContent value="devices" className="space-y-6 m-0">
            <div className="bg-white rounded-xl border shadow-sm h-full flex flex-col">
              <div className="p-4 border-b flex items-center justify-between">
                <h3 className="text-sm font-semibold text-slate-900 border-l-4 border-primary pl-3">
                  云打印机列表
                </h3>
                <Button size="sm" onClick={handleAddPrinter}>
                  <Plus className="h-4 w-4 mr-2" />
                  添加打印机
                </Button>
              </div>
              <ScrollArea className="flex-1 p-6">
                <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
                  {printers.map((printer) => (
                    <div 
                      key={printer.id}
                      className="p-4 rounded-xl border border-slate-200 bg-white hover:border-primary/50 transition-all group"
                    >
                      <div className="flex items-start justify-between mb-4">
                        <div className="h-10 w-10 bg-slate-100 rounded-lg flex items-center justify-center">
                          <Printer className="h-5 w-5 text-slate-500" />
                        </div>
                        <Switch 
                          checked={printer.is_active} 
                          onCheckedChange={(val) => handleTogglePrinter(printer.id, val)}
                        />
                      </div>
                      <div className="space-y-1">
                        <h4 className="font-bold text-slate-900">{printer.printer_name}</h4>
                        <p className="text-xs text-muted-foreground">SN: {printer.printer_sn}</p>
                        <Badge variant="outline" className="text-[10px] mt-2">
                          {printer.printer_type === 'feieyun' ? '飞鹅云' : printer.printer_type === 'yilianyun' ? '易联云' : '其他'}
                        </Badge>
                      </div>
                      <div className="mt-4 pt-4 border-t flex flex-wrap gap-2">
                        {printer.print_takeout && <Badge variant="secondary" className="text-[10px] bg-blue-50 text-blue-600 border-none">外卖</Badge>}
                        {printer.print_dine_in && <Badge variant="secondary" className="text-[10px] bg-emerald-50 text-emerald-600 border-none">堂食</Badge>}
                        {printer.print_reservation && <Badge variant="secondary" className="text-[10px] bg-amber-50 text-amber-600 border-none">预订</Badge>}
                      </div>
                      <div className="mt-4 flex gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
                        <Button variant="ghost" size="sm" className="flex-1 h-8 text-xs" onClick={() => handleEditPrinter(printer)}>
                          <Edit className="h-3 w-3 mr-1" />
                          编辑
                        </Button>
                        <Button 
                          variant="ghost" 
                          size="sm" 
                          className="flex-1 h-8 text-xs text-rose-600 hover:text-rose-700 hover:bg-rose-50"
                          onClick={() => setDeletePrinterConfirm(printer.id)}
                        >
                          <Trash2 className="h-3 w-3 mr-1" />
                          删除
                        </Button>
                      </div>
                    </div>
                  ))}
                  {printers.length === 0 && (
                    <div className="col-span-full py-16 flex flex-col items-center justify-center text-muted-foreground border-2 border-dashed rounded-xl">
                      <div className="w-12 h-12 bg-slate-100 rounded-full flex items-center justify-center mb-3">
                        <Printer className="w-6 h-6 text-slate-400" />
                      </div>
                      <p className="text-sm">暂未配置打印机</p>
                      <Button variant="link" size="sm" onClick={handleAddPrinter}>立即添加</Button>
                    </div>
                  )}
                </div>
              </ScrollArea>
            </div>
          </TabsContent>

          <TabsContent value="display" className="space-y-6 m-0">
            <div className="bg-white rounded-xl border shadow-sm">
              <div className="p-4 border-b">
                <h3 className="text-sm font-semibold text-slate-900 border-l-4 border-primary pl-3">
                  系统展示与提醒配置
                </h3>
              </div>
              <div className="p-8 max-w-4xl">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-12">
                  <section className="space-y-6">
                    <div className="flex flex-col gap-2 mb-6">
                      <h4 className="text-sm font-bold flex items-center gap-2">
                        <Printer className="h-4 w-4" /> 云打印设置
                      </h4>
                      <p className="text-xs text-muted-foreground">控制系统是否自动向已绑定的打印机发送小票</p>
                    </div>
                    
                    <div className="space-y-4">
                      <div className="flex items-center justify-between p-3 rounded-lg border border-slate-100 bg-slate-50/30">
                        <Label htmlFor="enable-print" className="font-medium">全局自动打印</Label>
                        <Switch 
                          id="enable-print" 
                          checked={displayConfig?.enable_print} 
                          onCheckedChange={val => displayConfig && setDisplayConfig({...displayConfig, enable_print: val})}
                        />
                      </div>
                      
                      <div className="pl-6 space-y-4 border-l-2 border-slate-100 ml-1">
                        <div className="flex items-center justify-between">
                          <Label className="text-sm font-normal text-slate-600">打印外卖订单</Label>
                          <Switch 
                            disabled={!displayConfig?.enable_print}
                            checked={displayConfig?.print_takeout} 
                            onCheckedChange={val => displayConfig && setDisplayConfig({...displayConfig, print_takeout: val})}
                          />
                        </div>
                        <div className="flex items-center justify-between">
                          <Label className="text-sm font-normal text-slate-600">打印堂食订单</Label>
                          <Switch 
                            disabled={!displayConfig?.enable_print}
                            checked={displayConfig?.print_dine_in} 
                            onCheckedChange={val => displayConfig && setDisplayConfig({...displayConfig, print_dine_in: val})}
                          />
                        </div>
                        <div className="flex items-center justify-between">
                          <Label className="text-sm font-normal text-slate-600">打印预订小票</Label>
                          <Switch 
                            disabled={!displayConfig?.enable_print}
                            checked={displayConfig?.print_reservation} 
                            onCheckedChange={val => displayConfig && setDisplayConfig({...displayConfig, print_reservation: val})}
                          />
                        </div>
                      </div>
                    </div>
                  </section>

                  <section className="space-y-6">
                    <div className="space-y-6">
                      <div className="flex flex-col gap-2">
                        <h4 className="text-sm font-bold flex items-center gap-2">
                          <ImageIcon className="h-4 w-4" /> KDS 厨显系统
                        </h4>
                        <p className="text-xs text-muted-foreground">配置后可在平板电脑等大屏展示后厨待做订单</p>
                      </div>
                      <div className="space-y-4">
                        <div className="flex items-center justify-between p-3 rounded-lg border border-slate-100 bg-slate-50/30">
                          <Label htmlFor="enable-kds" className="font-medium">启用 KDS 系统</Label>
                          <Switch 
                            id="enable-kds"
                            checked={displayConfig?.enable_kds} 
                            onCheckedChange={val => displayConfig && setDisplayConfig({...displayConfig, enable_kds: val})}
                          />
                        </div>
                        {displayConfig?.enable_kds && (
                          <div className="space-y-2 animate-in fade-in slide-in-from-top-2">
                            <Label className="text-xs">KDS 控制台访问地址 (URL)</Label>
                            <Input 
                              placeholder="https://yourapp.com/kds" 
                              value={displayConfig?.kds_url || ""}
                              onChange={e => setDisplayConfig({...displayConfig, kds_url: e.target.value})}
                            />
                            <p className="text-[10px] text-muted-foreground">开启后，后台将同步订单数据至此地址</p>
                          </div>
                        )}
                      </div>
                    </div>

                    <div className="space-y-6 pt-6 border-t">
                      <div className="flex flex-col gap-2">
                        <h4 className="text-sm font-bold flex items-center gap-2">
                          < ImageIcon className="h-4 w-4" /> 语音提醒
                        </h4>
                        <p className="text-xs text-muted-foreground">新订单到达时通过浏览器/桌面端发声</p>
                      </div>
                      <div className="space-y-4">
                        <div className="flex items-center justify-between p-3 rounded-lg border border-slate-100 bg-slate-50/30">
                          <Label htmlFor="enable-voice" className="font-medium">开启新订单配音</Label>
                          <Switch 
                            id="enable-voice"
                            checked={displayConfig?.enable_voice} 
                            onCheckedChange={val => displayConfig && setDisplayConfig({...displayConfig, enable_voice: val})}
                          />
                        </div>
                      </div>
                    </div>
                  </section>
                </div>

                <div className="mt-12 pt-8 border-t flex justify-end">
                  <Button onClick={handleSaveDisplayConfig} disabled={saving} size="lg" className="px-12 rounded-full">
                    {saving ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : <Save className="h-4 w-4 mr-2" />}
                    保存全局配置
                  </Button>
                </div>
              </div>
            </div>
          </TabsContent>
          <TabsContent value="group" className="space-y-6 m-0">
            <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
              <div className="md:col-span-2 space-y-6">
                {currentGroup ? (
                  <div className="bg-white rounded-xl border shadow-sm p-6 text-center space-y-4">
                    <div className="mx-auto w-16 h-16 bg-primary/10 rounded-full flex items-center justify-center">
                      <Building2 className="h-8 w-8 text-primary" />
                    </div>
                    <div>
                      <h3 className="text-xl font-bold">{currentGroup.name}</h3>
                      <p className="text-muted-foreground">{currentGroup.address}</p>
                    </div>
                    <Badge variant="outline" className="bg-emerald-100 text-emerald-700 hover:bg-emerald-100 px-4 py-1">
                      已加入集团
                    </Badge>
                    <div className="pt-4 text-xs text-muted-foreground">
                      如需退出集团，请联系集团管理员或系统运营。
                    </div>
                  </div>
                ) : (
                  <>
                    <div className="bg-white rounded-xl border shadow-sm p-6 space-y-4">
                      <div className="flex flex-col gap-2 mb-4">
                        <h3 className="text-lg font-bold">搜索并加入集团</h3>
                        <p className="text-sm text-muted-foreground">
                          加入集团可以统一管理菜品库、享受集团优惠活动及更高级的数据分析。
                        </p>
                      </div>
                      
                      <form onSubmit={handleSearchGroups} className="flex gap-2">
                        <Input 
                          placeholder="输入集团名称关键词..." 
                          value={groupKeyword}
                          onChange={e => setGroupKeyword(e.target.value)}
                        />
                        <Button type="submit" disabled={searchingGroups}>
                          {searchingGroups ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : <RefreshCw className="h-4 w-4 mr-2" />}
                          搜索
                        </Button>
                      </form>
                    </div>

                    <div className="space-y-4">
                      {groupResults.map(group => (
                        <div key={group.id} className="bg-white rounded-xl border shadow-sm p-4 flex items-center justify-between">
                          <div className="space-y-1">
                            <h4 className="font-bold">{group.name}</h4>
                            <div className="flex items-center gap-4 text-xs text-muted-foreground">
                              <span className="flex items-center gap-1"><MapPin className="h-3 w-3" /> {group.address}</span>
                              <span className="flex items-center gap-1"><Phone className="h-3 w-3" /> {group.contact_phone}</span>
                            </div>
                          </div>
                          <Button size="sm" onClick={() => setJoinReasonModal(group)}>
                            申请加入
                          </Button>
                        </div>
                      ))}
                      {groupKeyword && !searchingGroups && groupResults.length === 0 && (
                        <div className="py-12 text-center text-muted-foreground bg-slate-50 border-2 border-dashed rounded-xl">
                          未搜到相关集团
                        </div>
                      )}
                    </div>
                  </>
                )}
              </div>

              <div className="space-y-6">
                <div className="bg-white rounded-xl border shadow-sm p-6 space-y-4">
                  <h4 className="text-sm font-bold flex items-center gap-2">
                    <AlertCircle className="h-4 w-4 text-primary" /> 加入须知
                  </h4>
                  <ul className="text-xs space-y-3 text-muted-foreground list-disc pl-4">
                    <li>加入后，您的菜品分类可能由集团统一管理。</li>
                    <li>部分营销活动可能由集团强制推行或参与。</li>
                    <li>集团管理员将拥有查看您店铺流水和经营数据的权限。</li>
                    <li>一旦加入，解除绑定需由集团端发起或联系平台客服。</li>
                  </ul>
                </div>
              </div>
            </div>
          </TabsContent>
        </Tabs>
      </PageContent>

      <Dialog open={!!joinReasonModal} onOpenChange={(open) => !open && setJoinReasonModal(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>申请加入集团</DialogTitle>
            <DialogDescription>
              申请加入 <strong>{joinReasonModal?.name}</strong>。请填写申请理由，方便集团快速审核。
            </DialogDescription>
          </DialogHeader>
          <div className="py-4">
            <Textarea 
              placeholder="例如：希望统一品牌运营，使用集团配送资源..."
              value={joinReason}
              onChange={e => setJoinReason(e.target.value)}
              className="min-h-25"
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setJoinReasonModal(null)}>取消</Button>
            <Button onClick={handleJoinGroup} disabled={saving}>
              {saving && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
              确认申请
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={printerModalOpen} onOpenChange={setPrinterModalOpen}>
        <DialogContent className="sm:max-w-106.25">
          <DialogHeader>
            <DialogTitle>{editingPrinter?.id ? "编辑打印机" : "添加打印机"}</DialogTitle>
            <DialogDescription>
              请输入打印机品牌商提供的设备信息进行连接。目前支持飞鹅云和易联云。
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="space-y-2">
              <Label>打印机名称</Label>
              <Input 
                value={editingPrinter?.printer_name || ""} 
                onChange={e => setEditingPrinter({...editingPrinter, printer_name: e.target.value})}
                placeholder="例如：后厨小票机" 
              />
            </div>
            {!editingPrinter?.id && (
              <>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label>连接协议</Label>
                    <Select 
                      value={editingPrinter?.printer_type} 
                      onValueChange={(val) =>
                        setEditingPrinter({
                          ...editingPrinter,
                          printer_type: val as CloudPrinter["printer_type"],
                        })
                      }
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="feieyun">飞鹅云</SelectItem>
                        <SelectItem value="yilianyun">易联云</SelectItem>
                        <SelectItem value="other">其他</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <Label>状态</Label>
                    <div className="flex items-center h-10 gap-2">
                      <Switch 
                        checked={editingPrinter?.is_active} 
                        onCheckedChange={val => setEditingPrinter({...editingPrinter, is_active: val})} 
                      />
                      <span className="text-xs">已启用</span>
                    </div>
                  </div>
                </div>
                <div className="space-y-2">
                  <Label>打印机序列号 (SN)</Label>
                  <Input 
                    value={editingPrinter?.printer_sn || ""} 
                    onChange={e => setEditingPrinter({...editingPrinter, printer_sn: e.target.value})}
                    placeholder="设备底部标签上的 SN 码" 
                  />
                </div>
                <div className="space-y-2">
                  <Label>打印机密钥 (Key)</Label>
                  <Input 
                    value={editingPrinter?.printer_key || ""} 
                    onChange={e => setEditingPrinter({...editingPrinter, printer_key: e.target.value})}
                    type="password"
                    placeholder="厂商提供的设备 Key" 
                  />
                </div>
              </>
            )}
            <div className="space-y-3 pt-2">
              <Label className="text-xs">打印场景</Label>
              <div className="flex flex-wrap gap-4">
                <div className="flex items-center gap-2">
                  <Switch 
                    id="p-takeout" 
                    checked={editingPrinter?.print_takeout} 
                    onCheckedChange={val => setEditingPrinter({...editingPrinter, print_takeout: val})}
                  />
                  <Label htmlFor="p-takeout" className="font-normal">外卖</Label>
                </div>
                <div className="flex items-center gap-2">
                  <Switch 
                    id="p-dinein" 
                    checked={editingPrinter?.print_dine_in} 
                    onCheckedChange={val => setEditingPrinter({...editingPrinter, print_dine_in: val})}
                  />
                  <Label htmlFor="p-dinein" className="font-normal">堂食</Label>
                </div>
                <div className="flex items-center gap-2">
                  <Switch 
                    id="p-res" 
                    checked={editingPrinter?.print_reservation} 
                    onCheckedChange={val => setEditingPrinter({...editingPrinter, print_reservation: val})}
                  />
                  <Label htmlFor="p-res" className="font-normal">预订</Label>
                </div>
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPrinterModalOpen(false)}>取消</Button>
            <Button onClick={handleSavePrinter} disabled={saving}>
              {saving ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : <Save className="h-4 w-4 mr-2" />}
              {editingPrinter?.id ? "保存修改" : "确认添加"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog 
        open={!!deletePrinterConfirm}
        onOpenChange={(open) => !open && setDeletePrinterConfirm(null)}
        title="确认删除打印机？"
        description="删除后，该打印机将不再自动打印相关场景的小票，且无法撤销。"
        confirmText="确认删除"
        variant="destructive"
        onConfirm={handleDeletePrinter}
      />
    </PageShell>
  );
}
