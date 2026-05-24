"use client";

import { useState, useMemo, useEffect, useCallback } from "react";
import {
  Truck,
  Search,
  Plus,
  Trash2,
  CheckCircle2,
  XCircle,
  RefreshCw,
  Calendar,
  ArrowLeft,
  Info
} from "lucide-react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { apiGet, apiPost, apiPatch, apiDelete, formatAmount } from "@/lib/api";
import { cn } from "@/lib/utils";
import { PageShell, PageHeader, PageContent } from "@/components/merchant/layout/page-shell";
import { useMerchantSession } from "@/components/providers/merchant-session-provider";

import type { DeliveryPromotionResponse } from "@/types/delivery";

export function DeliveryPageClient() {
  const session = useMerchantSession();
  const router = useRouter();
  const [promotions, setPromotions] = useState<DeliveryPromotionResponse[]>([]);
  const [loading, setLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  
  // Selection & Editing
  const [selectedPromo, setSelectedPromo] = useState<DeliveryPromotionResponse | null>(null);
  const [isAdding, setIsAdding] = useState(false);
  const [saving, setSaving] = useState(false);

  // Form Data
  const [formData, setFormData] = useState<Partial<DeliveryPromotionResponse>>({});
  
  // Delete Confirm Dialog
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  const loadPromotions = useCallback(async () => {
    if (!session?.merchant?.id) return;
    setLoading(true);
    try {
      const response = await apiGet<DeliveryPromotionResponse[]>(`/delivery-fee/merchants/${session.merchant.id}/promotions`);
      setPromotions(response || []);
      
      // 同步更新当前选中的数据
      if (selectedPromo) {
        const updated = response.find(p => p.id === selectedPromo.id);
        if (updated) {
          setSelectedPromo(updated);
        }
      }
    } catch (error) {
      console.error("Failed to load delivery promotions", error);
    } finally {
      setLoading(false);
    }
  }, [session?.merchant?.id, selectedPromo]);

  useEffect(() => {
    loadPromotions();
  }, [loadPromotions]);

  const filteredPromos = useMemo(() => {
    if (!searchQuery) return promotions;
    return promotions.filter(p => p.name.toLowerCase().includes(searchQuery.toLowerCase()));
  }, [promotions, searchQuery]);

  // Select Promo handler
  const handleSelectPromo = (promo: DeliveryPromotionResponse) => {
    setSelectedPromo(promo);
    setIsAdding(false);
    setFormData({
        ...promo,
        valid_from: promo.valid_from.slice(0, 10),
        valid_until: promo.valid_until.slice(0, 10),
    });
  };

  // Add Promo handler
  const handleAddPromo = () => {
    setIsAdding(true);
    setSelectedPromo(null);
    const today = new Date();
    const nextMonth = new Date();
    nextMonth.setMonth(nextMonth.getMonth() + 1);
    
    const initialData = {
      name: "",
      min_order_amount: 0,
      discount_amount: 0,
      valid_from: today.toISOString().slice(0, 10),
      valid_until: nextMonth.toISOString().slice(0, 10),
      is_active: true,
    };
    setFormData(initialData);
  };

  // Reset Logic
  const handleReset = () => {
    if (isAdding) {
      handleAddPromo();
    } else if (selectedPromo) {
      setFormData({
        ...selectedPromo,
        valid_from: selectedPromo.valid_from.slice(0, 10),
        valid_until: selectedPromo.valid_until.slice(0, 10),
      });
    }
    toast.info("表单已重置");
  };

  // Save Logic
  const handleSave = async () => {
    if (!session?.merchant?.id) return;
    if (!formData.name?.trim()) {
      toast.error("请输入活动名称");
      return;
    }
    if ((formData.discount_amount || 0) <= 0) {
      toast.error("请输入有效减免金额");
      return;
    }
    if (!formData.valid_from || !formData.valid_until) {
      toast.error("请选择有效期");
      return;
    }

    setSaving(true);
    try {
      const payload = {
        name: formData.name!,
        min_order_amount: formData.min_order_amount || 0,
        discount_amount: formData.discount_amount!,
        valid_from: formData.valid_from + "T00:00:00Z",
        valid_until: formData.valid_until + "T23:59:59Z",
      };

      if (isAdding) {
        await apiPost(`/delivery-fee/merchants/${session.merchant.id}/promotions`, payload);
        toast.success("运费满返活动创建成功");
        setIsAdding(false);
      } else if (selectedPromo) {
        await apiPatch(`/delivery-fee/merchants/${session.merchant.id}/promotions/${selectedPromo.id}`, {
          ...payload,
          is_active: formData.is_active
        });
        toast.success("活动更新成功");
      }
      
      loadPromotions();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "保存失败";
      toast.error(message);
    } finally {
      setSaving(false);
    }
  };

  // Delete Logic
  const handleDeleteClick = () => {
    if (!selectedPromo) return;
    setDeleteDialogOpen(true);
  };

  const handleDeleteConfirm = async () => {
    if (!selectedPromo || !session?.merchant?.id) return;
    try {
      await apiDelete(`/delivery-fee/merchants/${session.merchant.id}/promotions/${selectedPromo.id}`);
      toast.success("活动已删除");
      setSelectedPromo(null);
      setIsAdding(false);
      loadPromotions();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "删除失败";
      toast.error(message);
    }
  };

  return (
    <PageShell>
      <PageHeader 
        title="运费满返管理" 
        description="设置商户承担的运费优惠，激励客户湊单，提升客单价"
        actions={
          <Button variant="outline" size="sm" onClick={() => router.push("/merchant/marketing")}>
            <ArrowLeft className="mr-2 h-4 w-4" />
            返回营销中心
          </Button>
        }
      />

      <PageContent>
        <div className="flex h-[calc(100vh-12rem)] gap-6">
          {/* Left Panel: List */}
          <div className="w-1/3 min-w-80 flex flex-col bg-white rounded-xl border shadow-sm">
            <div className="p-4 border-b space-y-4">
              <div className="flex items-center justify-between">
                <h3 className="font-medium">活动列表 ({promotions.length})</h3>
                <Button size="sm" onClick={handleAddPromo}>
                  <Plus className="h-4 w-4 mr-2" />
                  新建活动
                </Button>
              </div>
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input 
                  placeholder="搜索活动名称..." 
                  className="pl-9 bg-slate-50 border-slate-200"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                />
              </div>
            </div>

            <ScrollArea className="flex-1 p-2">
              <div className="space-y-2">
                {filteredPromos.map(promo => {
                    const isExpired = new Date(promo.valid_until) < new Date();
                    return (
                        <div 
                          key={promo.id}
                          onClick={() => handleSelectPromo(promo)}
                          className={cn(
                            "p-4 rounded-lg border transition-all cursor-pointer hover:border-primary/50 hover:bg-slate-50 group",
                            selectedPromo?.id === promo.id && !isAdding ? "border-primary bg-primary/5 ring-1 ring-primary" : "border-slate-100"
                          )}
                        >
                          <div className="flex justify-between items-start mb-2">
                            <div className="flex flex-col gap-1">
                                <span className="font-medium text-slate-900 line-clamp-1 group-hover:text-primary transition-colors">{promo.name}</span>
                                <div className="flex items-center gap-2">
                                    <Badge variant={promo.is_active ? "default" : "secondary"} className="text-[10px] h-4 px-1">
                                        {promo.is_active ? "进行中" : "已停用"}
                                    </Badge>
                                    {isExpired && <Badge variant="outline" className="text-[10px] h-4 px-1 text-rose-500 border-rose-200 bg-rose-50">已过期</Badge>}
                                </div>
                            </div>
                            <div className="text-right">
                                <div className="text-lg font-bold text-blue-600">
                                    <span className="text-xs font-normal mr-0.5">返</span>
                                    {formatAmount(promo.discount_amount)}
                                </div>
                                <div className="text-[10px] text-muted-foreground">
                                    满{formatAmount(promo.min_order_amount)}可用
                                </div>
                            </div>
                          </div>
                          
                          <div className="mt-3 pt-3 border-t border-slate-50 flex items-center gap-1 text-[10px] text-muted-foreground">
                              <Calendar className="h-3 w-3" />
                              <span>{promo.valid_from.slice(0, 10)} 至 {promo.valid_until.slice(0, 10)}</span>
                          </div>
                        </div>
                    );
                })}
                
                {filteredPromos.length === 0 && !loading && (
                  <div className="text-center py-10 text-muted-foreground text-sm flex flex-col items-center gap-2">
                    <Truck className="h-8 w-8 text-slate-200" />
                    <span>暂无运费优惠活动</span>
                  </div>
                )}
                
                {loading && (
                    <div className="flex justify-center py-10">
                        <RefreshCw className="h-6 w-6 animate-spin text-muted-foreground" />
                    </div>
                )}
              </div>
            </ScrollArea>
          </div>

          {/* Right Panel: Editor */}
          <div className="flex-1 bg-white rounded-xl border shadow-sm flex flex-col">
            {!selectedPromo && !isAdding ? (
              <div className="flex-1 flex flex-col items-center justify-center text-muted-foreground p-12 text-center">
                <div className="w-20 h-20 bg-slate-50 rounded-full flex items-center justify-center mb-6 shadow-inner">
                  <Truck className="w-10 h-10 text-slate-300" />
                </div>
                <h4 className="text-slate-900 font-medium mb-2">运费满返助手</h4>
                <p className="max-w-xs text-sm">在这里您可以设置由商户承担的运费补贴。点击“新建活动”开始设置您的运费营销计划。</p>
              </div>
            ) : (
              <>
                {/* Editor Header */}
                <div className="flex items-center justify-between p-4 border-b bg-slate-50/50">
                  <div className="flex items-center gap-4">
                    <div className="size-8 rounded-lg bg-blue-50 flex items-center justify-center text-blue-600">
                        <Truck className="size-4" />
                    </div>
                    <h2 className="text-lg font-semibold">
                      {isAdding ? "新建运费活动" : "编辑运费活动"}
                    </h2>
                    <div className="flex gap-2 ml-4">
                      {isAdding ? (
                        <>
                          <Button 
                            variant="ghost" 
                            size="sm" 
                            className="h-8" 
                            onClick={() => {
                              setIsAdding(false);
                              setFormData({});
                            }}
                          >
                            <XCircle className="h-4 w-4 mr-1" />
                            取消
                          </Button>
                          <Button variant="ghost" size="sm" className="h-8" onClick={handleReset}>
                            <RefreshCw className="h-3 w-3 mr-1" />
                            重置
                          </Button>
                        </>
                      ) : (
                        <>
                          <Button 
                            variant="ghost" 
                            size="sm" 
                            className="h-8 text-muted-foreground" 
                            onClick={() => {
                              setSelectedPromo(null);
                              setFormData({});
                            }}
                          >
                            <XCircle className="h-4 w-4 mr-1" />
                            收起
                          </Button>
                          <Button variant="ghost" size="sm" className="h-8" onClick={handleReset}>
                            <RefreshCw className="h-3 w-3 mr-1" />
                            重置
                          </Button>
                          <Button variant="ghost" size="sm" className="h-8 text-rose-600 hover:text-rose-700 hover:bg-rose-50" onClick={handleDeleteClick}>
                            <Trash2 className="h-3 w-3 mr-1" />
                            删除
                          </Button>
                        </>
                      )}
                    </div>
                  </div>
                  <Button onClick={handleSave} disabled={saving} className="px-8 shadow-sm">
                    {saving ? <RefreshCw className="mr-2 h-4 w-4 animate-spin" /> : <CheckCircle2 className="mr-2 h-4 w-4" />}
                    保存发布
                  </Button>
                </div>

                {/* Editor Content */}
                <ScrollArea className="flex-1">
                  <div className="p-8 space-y-10 max-w-2xl mx-auto">
                    
                    {/* Basic Info */}
                    <section className="space-y-6">
                      <div className="flex items-center gap-2">
                        <div className="h-4 w-1 bg-blue-500 rounded-full"></div>
                        <h3 className="text-sm font-semibold text-slate-900">核心信息</h3>
                      </div>
                      
                      <div className="grid gap-6">
                        <div className="grid gap-2">
                          <Label className="text-slate-700">活动名称 <span className="text-rose-500">*</span></Label>
                          <Input 
                            placeholder="例如：代取费满30减5、新春运费免除" 
                            className="bg-slate-50/50 focus:bg-white transition-all shadow-sm"
                            value={formData.name || ""}
                            onChange={(e) => setFormData(p => ({ ...p, name: e.target.value }))}
                          />
                        </div>
                        
                        <div className="grid grid-cols-2 gap-6">
                            <div className="grid gap-2">
                                <Label className="text-slate-700">最低订单金额 (元) <span className="text-rose-500">*</span></Label>
                                <div className="relative">
                                    <span className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground text-sm font-medium">满</span>
                                    <Input 
                                        type="number"
                                        className="pl-8 bg-slate-50/50 focus:bg-white shadow-sm"
                                        placeholder="0.00"
                                        value={formData.min_order_amount ? (formData.min_order_amount / 100).toString() : ""}
                                        onChange={(e) => {
                                            const val = parseFloat(e.target.value);
                                            setFormData(p => ({ ...p, min_order_amount: isNaN(val) ? 0 : Math.round(val * 100) }));
                                        }}
                                    />
                                </div>
                            </div>

                            <div className="grid gap-2">
                                <Label className="text-slate-700">运费减免金额 (元) <span className="text-rose-500">*</span></Label>
                                <div className="relative">
                                    <span className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground text-sm font-medium">返</span>
                                    <Input 
                                        type="number"
                                        className="pl-8 bg-slate-50/50 focus:bg-white shadow-sm"
                                        placeholder="0.00"
                                        value={formData.discount_amount ? (formData.discount_amount / 100).toString() : ""}
                                        onChange={(e) => {
                                            const val = parseFloat(e.target.value);
                                            setFormData(p => ({ ...p, discount_amount: isNaN(val) ? 0 : Math.round(val * 100) }));
                                        }}
                                    />
                                </div>
                            </div>
                        </div>

                        <div className="grid gap-2">
                            <Label className="text-slate-700">状态控制</Label>
                            <div className="flex items-center gap-3 h-10 px-1">
                                <Switch 
                                    id="promo-active"
                                    checked={formData.is_active}
                                    onCheckedChange={(c) => setFormData(p => ({ ...p, is_active: c }))}
                                />
                                <Label htmlFor="promo-active" className="cursor-pointer font-medium text-sm">
                                    {formData.is_active ? "活动进行中" : "已下架停用"}
                                </Label>
                            </div>
                        </div>
                      </div>
                    </section>

                    <Separator className="opacity-50" />

                    {/* Validity */}
                    <section className="space-y-6">
                      <div className="flex items-center gap-2">
                        <div className="h-4 w-1 bg-blue-500 rounded-full"></div>
                        <h3 className="text-sm font-semibold text-slate-900">有效期设置</h3>
                      </div>
                      <div className="grid grid-cols-2 gap-6">
                         <div className="grid gap-2">
                            <Label className="text-slate-700">开始日期</Label>
                            <div className="relative">
                                <Calendar className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground pointer-events-none" />
                                <Input 
                                    type="date"
                                    className="pl-10 bg-slate-50/50 focus:bg-white shadow-sm"
                                    value={formData.valid_from || ""}
                                    onChange={(e) => setFormData(p => ({ ...p, valid_from: e.target.value }))}
                                />
                            </div>
                         </div>
                         <div className="grid gap-2">
                            <Label className="text-slate-700">结束日期</Label>
                            <div className="relative">
                                <Calendar className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground pointer-events-none" />
                                <Input 
                                    type="date"
                                    className="pl-10 bg-slate-50/50 focus:bg-white shadow-sm"
                                    value={formData.valid_until || ""}
                                    onChange={(e) => setFormData(p => ({ ...p, valid_until: e.target.value }))}
                                />
                            </div>
                         </div>
                      </div>
                    </section>

                    <Separator className="opacity-50" />

                    {/* Rules Hint */}
                    <div className="p-4 rounded-xl border bg-amber-50/50 flex gap-4 items-start">
                        <div className="p-2 rounded-lg bg-amber-100 text-amber-700 shrink-0">
                            <Info className="size-5" />
                        </div>
                        <div className="space-y-1">
                            <h4 className="text-sm font-semibold text-amber-900">营销提示</h4>
                            <p className="text-xs text-amber-800/80 leading-relaxed">
                                这里的“减免金额”将由商户账户承担。客户支付订单时，若满足条件，该金额将从订单总额中扣除。骑手收到的代取费由系统运费配置决定，不受此页面影响。
                            </p>
                        </div>
                    </div>
                    
                    <div className="h-20" /> {/* Spacer */}
                  </div>
                </ScrollArea>
              </>
            )}
          </div>
        </div>
      </PageContent>

      {/* Delete Confirm Dialog */}
      <ConfirmDialog
        open={deleteDialogOpen}
        onOpenChange={setDeleteDialogOpen}
        title="确认删除运费活动"
        description={`确定要删除 "${selectedPromo?.name}" 吗？此操作不可撤销。`}
        confirmText="确认删除"
        variant="destructive"
        onConfirm={handleDeleteConfirm}
      />
    </PageShell>
  );
}
