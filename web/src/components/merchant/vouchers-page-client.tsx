"use client";

import { useState, useMemo, useEffect, useCallback } from "react";
import {
  Ticket,
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

import type { VoucherResponse, CreateVoucherRequest, UpdateVoucherRequest } from "@/types/voucher";

const ORDER_TYPE_OPTIONS = [
  { label: "堂食", value: "dine_in" },
  { label: "外卖", value: "takeout" },
  { label: "打包自取", value: "takeaway" },
  { label: "预订", value: "reservation" },
];

export function VouchersPageClient() {
  const session = useMerchantSession();
  const router = useRouter();
  const [vouchers, setVouchers] = useState<VoucherResponse[]>([]);
  const [loading, setLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  
  // Selection & Editing
  const [selectedVoucher, setSelectedVoucher] = useState<VoucherResponse | null>(null);
  const [isAdding, setIsAdding] = useState(false);
  const [saving, setSaving] = useState(false);

  // Form Data
  const [formData, setFormData] = useState<Partial<VoucherResponse>>({});
  
  // Delete Confirm Dialog
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  const loadVouchers = useCallback(async () => {
    if (!session?.merchant?.id) return;
    setLoading(true);
    try {
      const response = await apiGet<{ vouchers: VoucherResponse[] }>(`/merchants/${session.merchant.id}/vouchers`, { 
        page_id: 1, 
        page_size: 50 
      });
      setVouchers(response.vouchers || []);
    } catch (error) {
      console.error("Failed to load vouchers", error);
    } finally {
      setLoading(false);
    }
  }, [session?.merchant?.id]);

  useEffect(() => {
    loadVouchers();
  }, [loadVouchers]);

  const filteredVouchers = useMemo(() => {
    if (!searchQuery) return vouchers;
    return vouchers.filter(v => v.name.toLowerCase().includes(searchQuery.toLowerCase()));
  }, [vouchers, searchQuery]);

  // Select Voucher handler
  const handleSelectVoucher = (voucher: VoucherResponse) => {
    setSelectedVoucher(voucher);
    setIsAdding(false);
    setFormData({
        ...voucher,
        valid_from: voucher.valid_from.slice(0, 10),
        valid_until: voucher.valid_until.slice(0, 10),
    });
  };

  // Add Voucher handler
  const handleAddVoucher = () => {
    setIsAdding(true);
    setSelectedVoucher(null);
    const today = new Date();
    const nextMonth = new Date();
    nextMonth.setMonth(nextMonth.getMonth() + 1);
    
    setFormData({
      name: "",
      description: "",
      amount: 0,
      min_order_amount: 0,
      total_quantity: 100,
      valid_from: today.toISOString().slice(0, 10),
      valid_until: nextMonth.toISOString().slice(0, 10),
      is_active: true,
      allowed_order_types: ["dine_in", "takeout", "takeaway", "reservation"]
    });
  };

  // Save Logic
  const handleSave = async () => {
    if (!session?.merchant?.id) return;
    if (!formData.name?.trim()) {
      toast.error("请输入代金券名称");
      return;
    }
    if ((formData.amount || 0) <= 0) {
      toast.error("请输入有效优惠金额");
      return;
    }
    if ((formData.total_quantity || 0) <= 0) {
      toast.error("请输入有效发行数量");
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
    if (!formData.allowed_order_types || formData.allowed_order_types.length === 0) {
      toast.error("请至少选择一个适用场景");
      return;
    }

    setSaving(true);
    try {
      if (isAdding) {
        const payload: CreateVoucherRequest = {
          name: formData.name!,
          description: formData.description,
          amount: formData.amount!, // cents
          min_order_amount: formData.min_order_amount || 0,
          total_quantity: formData.total_quantity!,
          valid_from: formData.valid_from + "T00:00:00Z",
          valid_until: formData.valid_until + "T23:59:59Z",
          allowed_order_types: formData.allowed_order_types!,
        };
        await apiPost(`/merchants/${session.merchant.id}/vouchers`, payload as any);
        toast.success("代金券创建成功");
        setIsAdding(false);
      } else if (selectedVoucher) {
        const payload: UpdateVoucherRequest = {
          name: formData.name,
          description: formData.description,
          amount: formData.amount,
          min_order_amount: formData.min_order_amount,
          total_quantity: formData.total_quantity,
          valid_from: formData.valid_from + "T00:00:00Z",
          valid_until: formData.valid_until + "T23:59:59Z",
          is_active: formData.is_active,
          allowed_order_types: formData.allowed_order_types,
        };
        await apiPatch(`/merchants/${session.merchant.id}/vouchers/${selectedVoucher.id}`, payload as any);
        toast.success("代金券更新成功");
      }
      
      loadVouchers();
    } catch (error: any) {
      toast.error(error.message || "保存失败");
    } finally {
      setSaving(false);
    }
  };

  // Delete Logic
  const handleDeleteClick = () => {
    if (!selectedVoucher) return;
    setDeleteDialogOpen(true);
  };

  const handleDeleteConfirm = async () => {
    if (!selectedVoucher || !session?.merchant?.id) return;
    try {
      await apiDelete(`/merchants/${session.merchant.id}/vouchers/${selectedVoucher.id}`);
      toast.success("代金券已删除");
      setSelectedVoucher(null);
      setIsAdding(false);
      loadVouchers();
    } catch (error: any) {
      toast.error(error.message || "删除失败");
    }
  };

  const toggleOrderType = (type: string) => {
    setFormData(prev => {
        const current = prev.allowed_order_types || [];
        if (current.includes(type)) {
            return { ...prev, allowed_order_types: current.filter(t => t !== type) };
        } else {
            return { ...prev, allowed_order_types: [...current, type] };
        }
    });
  };

  return (
    <PageShell>
      <PageHeader 
        title="代金券管理" 
        description="创建和管理商户代金券，吸引客户到店或下单"
        actions={
          <Button variant="outline" size="sm" onClick={() => router.push("/merchant/marketing")}>
            <ArrowLeft className="mr-2 h-4 w-4" />
            返回营销中心
          </Button>
        }
      />

      <PageContent>
        <div className="flex h-[calc(100vh-12rem)] gap-6">
          {/* Left Panel: Voucher List */}
          <div className="w-1/3 min-w-[320px] flex flex-col bg-white rounded-xl border shadow-sm">
            <div className="p-4 border-b space-y-4">
              <div className="flex items-center justify-between">
                <h3 className="font-medium">代金券列表 ({vouchers.length})</h3>
                <Button size="sm" onClick={handleAddVoucher}>
                  <Plus className="h-4 w-4 mr-2" />
                  新建券
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
                {filteredVouchers.map(voucher => {
                    const isExpired = new Date(voucher.valid_until) < new Date();
                    const isSoldOut = voucher.claimed_quantity >= voucher.total_quantity;
                    
                    return (
                        <div 
                          key={voucher.id}
                          onClick={() => handleSelectVoucher(voucher)}
                          className={cn(
                            "p-4 rounded-lg border transition-all cursor-pointer hover:border-primary/50 hover:bg-slate-50 group",
                            selectedVoucher?.id === voucher.id && !isAdding ? "border-primary bg-primary/5 ring-1 ring-primary" : "border-slate-100"
                          )}
                        >
                          <div className="flex justify-between items-start mb-2">
                            <div className="flex flex-col gap-1">
                                <span className="font-medium text-slate-900 line-clamp-1 group-hover:text-primary transition-colors">{voucher.name}</span>
                                <div className="flex items-center gap-2">
                                    <Badge variant={voucher.is_active ? "default" : "secondary"} className="text-[10px] h-4 px-1">
                                        {voucher.is_active ? "进行中" : "已停用"}
                                    </Badge>
                                    {isExpired && <Badge variant="outline" className="text-[10px] h-4 px-1 text-rose-500 border-rose-200 bg-rose-50">已过期</Badge>}
                                    {isSoldOut && <Badge variant="outline" className="text-[10px] h-4 px-1 text-amber-500 border-amber-200 bg-amber-50">已领完</Badge>}
                                </div>
                            </div>
                            <span className="text-lg font-bold text-primary shrink-0">
                              <span className="text-xs font-normal mr-0.5">¥</span>
                              {formatAmount(voucher.amount)}
                            </span>
                          </div>
                          
                          <div className="space-y-2 mt-3">
                            <div className="flex justify-between text-[11px] text-muted-foreground">
                                <span>领取进度: {voucher.claimed_quantity}/{voucher.total_quantity}</span>
                                <span>使用: {voucher.used_quantity}</span>
                            </div>
                            <div className="w-full bg-slate-100 h-1 rounded-full overflow-hidden">
                                <div 
                                    className="bg-primary h-full transition-all" 
                                    style={{ width: `${Math.min(100, (voucher.claimed_quantity / voucher.total_quantity) * 100)}%` }}
                                />
                            </div>
                            <div className="flex items-center gap-1 text-[10px] text-muted-foreground pt-1">
                                <Calendar className="h-3 w-3" />
                                <span>{voucher.valid_from.slice(0, 10)} 至 {voucher.valid_until.slice(0, 10)}</span>
                            </div>
                          </div>
                        </div>
                    );
                })}
                
                {filteredVouchers.length === 0 && !loading && (
                  <div className="text-center py-10 text-muted-foreground text-sm flex flex-col items-center gap-2">
                    <Ticket className="h-8 w-8 text-slate-200" />
                    <span>暂无代金券数据</span>
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
            {!selectedVoucher && !isAdding ? (
              <div className="flex-1 flex flex-col items-center justify-center text-muted-foreground p-12 text-center">
                <div className="w-20 h-20 bg-slate-50 rounded-full flex items-center justify-center mb-6 shadow-inner">
                  <Ticket className="w-10 h-10 text-slate-300" />
                </div>
                <h4 className="text-slate-900 font-medium mb-2">欢迎管理代金券</h4>
                <p className="max-w-xs text-sm">选择左侧的代金券进行编辑，或者点击“新建券”开始创建您的营销活动。</p>
              </div>
            ) : (
              <>
                {/* Editor Header */}
                <div className="flex items-center justify-between p-4 border-b bg-slate-50/50">
                  <div className="flex items-center gap-4">
                    <div className="size-8 rounded-lg bg-primary/10 flex items-center justify-center text-primary">
                        <Ticket className="size-4" />
                    </div>
                    <h2 className="text-lg font-semibold">
                      {isAdding ? "新建代金券" : "编辑代金券"}
                    </h2>
                    <div className="flex gap-2 ml-4">
                      {isAdding ? (
                        <Button 
                          variant="ghost" 
                          size="sm" 
                          className="h-8" 
                          onClick={() => {
                            setIsAdding(false);
                            setFormData({});
                          }}
                        >
                          <XCircle className="h-3.3 w-3.5 mr-1" />
                          取消
                        </Button>
                      ) : selectedVoucher ? (
                        <>
                          <Button 
                            variant="ghost" 
                            size="sm" 
                            className="h-8 text-muted-foreground" 
                            onClick={() => {
                              setSelectedVoucher(null);
                              setFormData({});
                            }}
                          >
                            <XCircle className="h-4 w-4 mr-1" />
                            收起
                          </Button>
                          <Button variant="ghost" size="sm" className="h-8" onClick={() => handleSelectVoucher(selectedVoucher)}>
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
                          <Label className="text-slate-700">代金券名称 <span className="text-rose-500">*</span></Label>
                          <Input 
                            placeholder="例如：10元午餐通用券" 
                            className="bg-slate-50/50 focus:bg-white transition-all shadow-sm"
                            value={formData.name || ""}
                            onChange={(e) => setFormData(p => ({ ...p, name: e.target.value }))}
                          />
                        </div>
                        
                        <div className="grid grid-cols-2 gap-6">
                             <div className="grid gap-2">
                                <Label className="text-slate-700">面额 (元) <span className="text-rose-500">*</span></Label>
                                <div className="relative">
                                    <span className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground text-sm font-medium">¥</span>
                                    <Input 
                                        type="number"
                                        className="pl-7 bg-slate-50/50 focus:bg-white shadow-sm"
                                        placeholder="0.00"
                                        value={formData.amount ? (formData.amount / 100).toString() : ""}
                                        onChange={(e) => {
                                            const val = parseFloat(e.target.value);
                                            setFormData(p => ({ ...p, amount: isNaN(val) ? 0 : Math.round(val * 100) }));
                                        }}
                                        disabled={!isAdding}
                                    />
                                </div>
                                {!isAdding && <p className="text-[11px] text-amber-600 flex items-center gap-1"><AlertCircle className="size-3" /> 发布后不可修改金额</p>}
                            </div>

                            <div className="grid gap-2">
                                <Label className="text-slate-700">最低消费 (元)</Label>
                                <div className="relative">
                                    <span className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground text-sm font-medium">¥</span>
                                    <Input 
                                        type="number"
                                        className="pl-7 bg-slate-50/50 focus:bg-white shadow-sm"
                                        placeholder="0.00 (选填)"
                                        value={formData.min_order_amount ? (formData.min_order_amount / 100).toString() : ""}
                                        onChange={(e) => {
                                            const val = parseFloat(e.target.value);
                                            setFormData(p => ({ ...p, min_order_amount: isNaN(val) ? 0 : Math.round(val * 100) }));
                                        }}
                                        disabled={!isAdding}
                                    />
                                </div>
                                {!isAdding && <p className="text-[11px] text-amber-600 flex items-center gap-1"><AlertCircle className="size-3" /> 发布后不可修改门槛</p>}
                            </div>
                        </div>

                        <div className="grid grid-cols-2 gap-6">
                            <div className="grid gap-2">
                                <Label className="text-slate-700">发行总量 <span className="text-rose-500">*</span></Label>
                                <Input 
                                    type="number"
                                    className="bg-slate-50/50 focus:bg-white shadow-sm"
                                    placeholder="数量"
                                    value={formData.total_quantity?.toString() || ""}
                                    onChange={(e) => setFormData(p => ({ ...p, total_quantity: parseInt(e.target.value) || 0 }))}
                                />
                                <p className="text-[11px] text-muted-foreground flex items-center gap-1"><Info className="size-3" /> 已领取 {selectedVoucher?.claimed_quantity || 0}</p>
                            </div>
                            
                            <div className="grid gap-2">
                                <Label className="text-slate-700">状态控制</Label>
                                <div className="flex items-center gap-3 h-10 px-1">
                                    <Switch 
                                        id="voucher-active"
                                        checked={formData.is_active}
                                        onCheckedChange={(c) => setFormData(p => ({ ...p, is_active: c }))}
                                    />
                                    <Label htmlFor="voucher-active" className="cursor-pointer font-medium text-sm">
                                        {formData.is_active ? "处于发放中" : "已下架停发"}
                                    </Label>
                                </div>
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

                    {/* Usage Scenes */}
                    <section className="space-y-6">
                      <div className="flex items-center gap-2">
                        <div className="h-4 w-1 bg-primary rounded-full"></div>
                        <h3 className="text-sm font-semibold text-slate-900">适用范围</h3>
                      </div>
                      
                      <div className="p-4 rounded-xl border bg-slate-50/30 grid grid-cols-2 gap-y-4 gap-x-8">
                        {ORDER_TYPE_OPTIONS.map((option) => (
                           <div key={option.value} className="flex items-center space-x-3 py-1">
                                <Checkbox 
                                    id={`type-${option.value}`} 
                                    checked={formData.allowed_order_types?.includes(option.value)}
                                    onCheckedChange={() => toggleOrderType(option.value)}
                                />
                                <label
                                    htmlFor={`type-${option.value}`}
                                    className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70 cursor-pointer"
                                >
                                    {option.label}
                                </label>
                           </div>
                        ))}
                      </div>
                    </section>

                    <Separator className="opacity-50" />

                    {/* Description */}
                    <section className="space-y-4">
                      <div className="flex items-center gap-2">
                        <div className="h-4 w-1 bg-primary rounded-full"></div>
                        <h3 className="text-sm font-semibold text-slate-900">更多描述</h3>
                      </div>
                      <div className="grid gap-2">
                        <Textarea 
                          placeholder="例如代金券不可与其他优惠同享，或仅限某些节假日不可用等细则..." 
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
        title="确认删除代金券"
        description={`确定要删除 "${selectedVoucher?.name}" 吗？注意：删除会导致已领取但未使用的代金券也将无法使用。`}
        confirmText="确认删除"
        variant="destructive"
        onConfirm={handleDeleteConfirm}
      />
    </PageShell>
  );
}
