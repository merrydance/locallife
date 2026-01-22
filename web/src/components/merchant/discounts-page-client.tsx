"use client";

import { useState, useMemo, useEffect, useCallback } from "react";
import {
  Tag,
  Search,
  Plus,
  Trash2,
  Edit,
  CheckCircle2,
  XCircle,
  RefreshCw,
  Clock,
  Calendar,
  AlertCircle,
  Info,
  ArrowLeft
} from "lucide-react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Checkbox } from "@/components/ui/checkbox";
import { apiGet, apiPost, apiPatch, apiDelete, formatAmount } from "@/lib/api";
import { cn } from "@/lib/utils";
import { PageShell, PageHeader, PageContent } from "@/components/merchant/layout/page-shell";
import { useMerchantSession } from "@/components/providers/merchant-session-provider";

import type { DiscountResponse, CreateDiscountRequest, UpdateDiscountRequest, ListDiscountsResponse } from "@/types/discount";

export function DiscountsPageClient() {
  const session = useMerchantSession();
  const router = useRouter();
  const [discounts, setDiscounts] = useState<DiscountResponse[]>([]);
  const [loading, setLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  
  // Selection & Editing
  const [selectedDiscount, setSelectedDiscount] = useState<DiscountResponse | null>(null);
  const [isAdding, setIsAdding] = useState(false);
  const [saving, setSaving] = useState(false);

  // Form Data
  const [formData, setFormData] = useState<Partial<DiscountResponse>>({});
  
  // Delete Confirm Dialog
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  const loadDiscounts = useCallback(async () => {
    if (!session?.merchant?.id) return;
    setLoading(true);
    try {
      const response = await apiGet<ListDiscountsResponse>(`/merchants/${session.merchant.id}/discounts`, { 
        page_id: 1, 
        page_size: 50 
      });
      const rules = response.rules || [];
      setDiscounts(rules);
      
      // 同步更新当前选中的数据，确保重置逻辑能拿到最新保存的值
      if (selectedDiscount) {
        const updated = rules.find(r => r.id === selectedDiscount.id);
        if (updated) {
          setSelectedDiscount(updated);
        }
      }
    } catch (error) {
      console.error("Failed to load discounts", error);
    } finally {
      setLoading(false);
    }
  }, [session?.merchant?.id, selectedDiscount?.id]);

  useEffect(() => {
    loadDiscounts();
  }, [loadDiscounts]);

  const filteredDiscounts = useMemo(() => {
    if (!searchQuery) return discounts;
    return discounts.filter(d => d.name.toLowerCase().includes(searchQuery.toLowerCase()));
  }, [discounts, searchQuery]);

  // Select Discount handler
  const handleSelectDiscount = (discount: DiscountResponse) => {
    setSelectedDiscount(discount);
    setIsAdding(false);
    setFormData({
        ...discount,
        valid_from: discount.valid_from.slice(0, 10),
        valid_until: discount.valid_until.slice(0, 10),
    });
  };

  // Add Discount handler
  const handleAddDiscount = () => {
    setIsAdding(true);
    setSelectedDiscount(null);
    const today = new Date();
    const nextMonth = new Date();
    nextMonth.setMonth(nextMonth.getMonth() + 1);
    
    const initialData = {
      name: "",
      description: "",
      min_order_amount: 0,
      discount_amount: 0,
      valid_from: today.toISOString().slice(0, 10),
      valid_until: nextMonth.toISOString().slice(0, 10),
      can_stack_with_voucher: false,
      can_stack_with_membership: true,
      is_active: true,
    };
    setFormData(initialData);
  };

  // Reset Logic
  const handleReset = () => {
    if (isAdding) {
      handleAddDiscount(); // 如果是新增，重置为初始状态
    } else if (selectedDiscount) {
      setFormData({
        ...selectedDiscount,
        valid_from: selectedDiscount.valid_from.slice(0, 10),
        valid_until: selectedDiscount.valid_until.slice(0, 10),
      });
    }
    toast.info("表单已重置");
  };

  // Save Logic
  const handleSave = async () => {
    if (!session?.merchant?.id) return;
    if (!formData.name?.trim()) {
      toast.error("请输入满减活动名称");
      return;
    }
    if ((formData.discount_amount || 0) <= 0) {
      toast.error("请输入有效优惠金额");
      return;
    }
    if ((formData.min_order_amount || 0) <= (formData.discount_amount || 0)) {
        toast.error("最低消费金额必须大于优惠金额");
        return;
    }
    if (!formData.valid_from || !formData.valid_until) {
      toast.error("请选择有效期");
      return;
    }
    if (formData.valid_until < formData.valid_from) {
      toast.error("结束日期应晚于开始日期");
      return;
    }

    setSaving(true);
    try {
      if (isAdding) {
        const payload: CreateDiscountRequest = {
          name: formData.name!,
          description: formData.description,
          min_order_amount: formData.min_order_amount!,
          discount_amount: formData.discount_amount!,
          can_stack_with_voucher: !!formData.can_stack_with_voucher,
          can_stack_with_membership: !!formData.can_stack_with_membership,
          valid_from: formData.valid_from + "T00:00:00Z",
          valid_until: formData.valid_until + "T23:59:59Z",
        };
        await apiPost(`/merchants/${session.merchant.id}/discounts`, payload as any);
        toast.success("满减活动创建成功");
        setIsAdding(false);
      } else if (selectedDiscount) {
        const payload: UpdateDiscountRequest = {
          id: selectedDiscount.id,
          name: formData.name,
          description: formData.description,
          min_order_amount: formData.min_order_amount,
          discount_amount: formData.discount_amount,
          can_stack_with_voucher: formData.can_stack_with_voucher,
          can_stack_with_membership: formData.can_stack_with_membership,
          valid_from: formData.valid_from + "T00:00:00Z",
          valid_until: formData.valid_until + "T23:59:59Z",
          is_active: formData.is_active,
        };
        await apiPatch(`/merchants/${session.merchant.id}/discounts/${selectedDiscount.id}`, payload as any);
        toast.success("满减活动更新成功");
      }
      
      loadDiscounts();
    } catch (error: any) {
      toast.error(error.message || "保存失败");
    } finally {
      setSaving(false);
    }
  };

  // Delete Logic
  const handleDeleteClick = () => {
    if (!selectedDiscount) return;
    setDeleteDialogOpen(true);
  };

  const handleDeleteConfirm = async () => {
    if (!selectedDiscount || !session?.merchant?.id) return;
    try {
      await apiDelete(`/merchants/${session.merchant.id}/discounts/${selectedDiscount.id}`);
      toast.success("满减活动已删除");
      setSelectedDiscount(null);
      setIsAdding(false);
      loadDiscounts();
    } catch (error: any) {
      toast.error(error.message || "删除失败");
    }
  };

  return (
    <PageShell>
      <PageHeader 
        title="限时满减管理" 
        description="创建和管理满减促销活动，提高客单价和订单量"
        actions={
          <Button variant="outline" size="sm" onClick={() => router.push("/merchant/marketing")}>
            <ArrowLeft className="mr-2 h-4 w-4" />
            返回营销中心
          </Button>
        }
      />

      <PageContent>
        <div className="flex h-[calc(100vh-12rem)] gap-6">
          {/* Left Panel: Discount List */}
          <div className="w-1/3 min-w-[320px] flex flex-col bg-white rounded-xl border shadow-sm">
            <div className="p-4 border-b space-y-4">
              <div className="flex items-center justify-between">
                <h3 className="font-medium">活动列表 ({discounts.length})</h3>
                <Button size="sm" onClick={handleAddDiscount}>
                  <Plus className="h-4 w-4 mr-2" />
                  新建活动
                </Button>
              </div>
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input 
                  placeholder="搜索名称..." 
                  className="pl-9 bg-slate-50 border-slate-200"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                />
              </div>
            </div>

            <ScrollArea className="flex-1 p-2">
              <div className="space-y-2">
                {filteredDiscounts.map(discount => {
                    const isExpired = new Date(discount.valid_until) < new Date();
                    
                    return (
                        <div 
                          key={discount.id}
                          onClick={() => handleSelectDiscount(discount)}
                          className={cn(
                            "p-4 rounded-lg border transition-all cursor-pointer hover:border-primary/50 hover:bg-slate-50 group",
                            selectedDiscount?.id === discount.id && !isAdding ? "border-primary bg-primary/5 ring-1 ring-primary" : "border-slate-100"
                          )}
                        >
                          <div className="flex justify-between items-start mb-2">
                            <div className="flex flex-col gap-1">
                                <span className="font-medium text-slate-900 line-clamp-1 group-hover:text-primary transition-colors">{discount.name}</span>
                                <div className="flex items-center gap-2">
                                    <Badge variant={discount.is_active ? "default" : "secondary"} className="text-[10px] h-4 px-1">
                                        {discount.is_active ? "进行中" : "已停用"}
                                    </Badge>
                                    {isExpired && <Badge variant="outline" className="text-[10px] h-4 px-1 text-rose-500 border-rose-200 bg-rose-50">已过期</Badge>}
                                </div>
                            </div>
                            <div className="text-right">
                                <div className="text-lg font-bold text-primary">
                                    <span className="text-xs font-normal mr-0.5">减</span>
                                    {formatAmount(discount.discount_amount)}
                                </div>
                                <div className="text-[10px] text-muted-foreground">
                                    满{formatAmount(discount.min_order_amount)}可用
                                </div>
                            </div>
                          </div>
                          
                          <div className="mt-3 pt-3 border-t border-slate-50 space-y-2">
                            <div className="flex items-center gap-4 text-[10px] text-muted-foreground">
                                <div className="flex items-center gap-1">
                                    <div className={cn("size-1.5 rounded-full", discount.can_stack_with_voucher ? "bg-green-500" : "bg-slate-300")} />
                                    <span>叠加代金券</span>
                                </div>
                                <div className="flex items-center gap-1">
                                    <div className={cn("size-1.5 rounded-full", discount.can_stack_with_membership ? "bg-green-500" : "bg-slate-300")} />
                                    <span>叠加强制会员</span>
                                </div>
                            </div>
                            <div className="flex items-center gap-1 text-[10px] text-muted-foreground">
                                <Calendar className="h-3 w-3" />
                                <span>{discount.valid_from.slice(0, 10)} 至 {discount.valid_until.slice(0, 10)}</span>
                            </div>
                          </div>
                        </div>
                    );
                })}
                
                {filteredDiscounts.length === 0 && !loading && (
                  <div className="text-center py-10 text-muted-foreground text-sm flex flex-col items-center gap-2">
                    <Tag className="h-8 w-8 text-slate-200" />
                    <span>暂无满减活动数据</span>
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
            {!selectedDiscount && !isAdding ? (
              <div className="flex-1 flex flex-col items-center justify-center text-muted-foreground p-12 text-center">
                <div className="w-20 h-20 bg-slate-50 rounded-full flex items-center justify-center mb-6 shadow-inner">
                  <Tag className="w-10 h-10 text-slate-300" />
                </div>
                <h4 className="text-slate-900 font-medium mb-2">欢迎管理满减活动</h4>
                <p className="max-w-xs text-sm">选择左侧的活动进行编辑，或者点击“新建活动”开始您的促销计划。</p>
              </div>
            ) : (
              <>
                {/* Editor Header */}
                <div className="flex items-center justify-between p-4 border-b bg-slate-50/50">
                  <div className="flex items-center gap-4">
                    <div className="size-8 rounded-lg bg-primary/10 flex items-center justify-center text-primary">
                        <Tag className="size-4" />
                    </div>
                    <h2 className="text-lg font-semibold">
                      {isAdding ? "新建满减活动" : "编辑满减活动"}
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
                      ) : selectedDiscount ? (
                        <>
                          <Button 
                            variant="ghost" 
                            size="sm" 
                            className="h-8 text-muted-foreground" 
                            onClick={() => {
                              setSelectedDiscount(null);
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
                      ) : null}
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
                        <div className="h-4 w-1 bg-primary rounded-full"></div>
                        <h3 className="text-sm font-semibold text-slate-900">核心信息</h3>
                      </div>
                      
                      <div className="grid gap-6">
                        <div className="grid gap-2">
                          <Label className="text-slate-700">满减名称 <span className="text-rose-500">*</span></Label>
                          <Input 
                            placeholder="例如：午间满减、周末满30减5" 
                            className="bg-slate-50/50 focus:bg-white transition-all shadow-sm"
                            value={formData.name || ""}
                            onChange={(e) => setFormData(p => ({ ...p, name: e.target.value }))}
                          />
                        </div>
                        
                        <div className="grid grid-cols-2 gap-6">
                            <div className="grid gap-2">
                                <Label className="text-slate-700">最低消费 (元) <span className="text-rose-500">*</span></Label>
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
                                <Label className="text-slate-700">减免金额 (元) <span className="text-rose-500">*</span></Label>
                                <div className="relative">
                                    <span className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground text-sm font-medium">减</span>
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
                                    id="discount-active"
                                    checked={formData.is_active}
                                    onCheckedChange={(c) => setFormData(p => ({ ...p, is_active: c }))}
                                />
                                <Label htmlFor="discount-active" className="cursor-pointer font-medium text-sm">
                                    {formData.is_active ? "处于活动中" : "已下架停用"}
                                </Label>
                            </div>
                        </div>
                      </div>
                    </section>

                    <Separator className="opacity-50" />

                    {/* Validity */}
                    <section className="space-y-6">
                      <div className="flex items-center gap-2">
                        <div className="h-4 w-1 bg-primary rounded-full"></div>
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

                    {/* Stacking Options */}
                    <section className="space-y-6">
                      <div className="flex items-center gap-2">
                        <div className="h-4 w-1 bg-primary rounded-full"></div>
                        <h3 className="text-sm font-semibold text-slate-900">叠加规则</h3>
                      </div>
                      
                      <div className="p-4 rounded-xl border bg-slate-50/30 space-y-4">
                        <div className="flex items-center space-x-3 py-1">
                            <Checkbox 
                                id="stack-voucher" 
                                checked={formData.can_stack_with_voucher}
                                onCheckedChange={(c) => setFormData(p => ({ ...p, can_stack_with_voucher: !!c }))}
                            />
                            <div className="grid gap-0.5 leading-none">
                                <label
                                    htmlFor="stack-voucher"
                                    className="text-sm font-medium cursor-pointer"
                                >
                                    可与代金券叠加使用
                                </label>
                                <p className="text-[11px] text-muted-foreground">勾选后，用户在结算时可同时使用代金券和满减活动</p>
                            </div>
                        </div>

                        <div className="flex items-center space-x-3 py-1">
                            <Checkbox 
                                id="stack-membership" 
                                checked={formData.can_stack_with_membership}
                                onCheckedChange={(c) => setFormData(p => ({ ...p, can_stack_with_membership: !!c }))}
                            />
                            <div className="grid gap-0.5 leading-none">
                                <label
                                    htmlFor="stack-membership"
                                    className="text-sm font-medium cursor-pointer"
                                >
                                    可与会员折扣叠加使用
                                </label>
                                <p className="text-[11px] text-muted-foreground">勾选后，满减金额将在会员折扣价基础上进一步扣减</p>
                            </div>
                        </div>
                      </div>
                    </section>

                    <Separator className="opacity-50" />

                    {/* Description */}
                    <section className="space-y-4">
                      <div className="flex items-center gap-2">
                        <div className="h-4 w-1 bg-primary rounded-full"></div>
                        <h3 className="text-sm font-semibold text-slate-900">活动描述</h3>
                      </div>
                      <div className="grid gap-2">
                        <Textarea 
                          placeholder="例如：本满减活动仅限正餐时段使用，不与特价套餐同享等备注信息..." 
                          className="min-h-[120px] bg-slate-50/50 focus:bg-white p-4 resize-none shadow-sm"
                          value={formData.description || ""}
                          onChange={(e) => setFormData(p => ({ ...p, description: e.target.value }))}
                        />
                      </div>
                    </section>
                    
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
        title="确认删除满减活动"
        description={`确定要删除 "${selectedDiscount?.name}" 吗？此操作不可撤销。`}
        confirmText="确认删除"
        variant="destructive"
        onConfirm={handleDeleteConfirm}
      />
    </PageShell>
  );
}
