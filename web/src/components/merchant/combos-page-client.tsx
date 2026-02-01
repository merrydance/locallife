"use client";

import { useState, useMemo, useEffect } from "react";
import Image from "next/image";
import { 
  Search, 
  Plus, 
  Trash2, 
  Box, 
  CheckCircle2,
  XCircle,
  RefreshCw,
  Utensils
} from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { ScrollArea } from "@/components/ui/scroll-area";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { apiGet, apiPost, apiPut, apiDelete, formatAmount, getMediaUrl, formatImageUrl } from "@/lib/api";
import { cn } from "@/lib/utils";
import { PageShell, PageHeader, PageContent } from "@/components/merchant/layout/page-shell";

import type { ComboSetResponse, ComboDishInfo, CreateComboRequest } from "@/types/combo";
import type { DishResponse, TagInfo } from "@/types/dish";

interface CombosPageClientProps {
  initialData: ComboSetResponse[];
}

export function CombosPageClient({ initialData }: CombosPageClientProps) {
  const [combos, setCombos] = useState<ComboSetResponse[]>(initialData);
  const [searchQuery, setSearchQuery] = useState("");
  
  // Selection & Editing
  const [selectedCombo, setSelectedCombo] = useState<ComboSetResponse | null>(null);
  const [isAdding, setIsAdding] = useState(false);
  const [saving, setSaving] = useState(false);

  // Form Data
  const [formData, setFormData] = useState<Partial<ComboSetResponse>>({});
  const [formDishes, setFormDishes] = useState<ComboDishInfo[]>([]);
  const [formTagIds, setFormTagIds] = useState<number[]>([]);

  // Resources
  const [allDishes, setAllDishes] = useState<DishResponse[]>([]);
  const [availableTags, setAvailableTags] = useState<TagInfo[]>([]);
  
  // Dish Picker State
  const [isDishPickerOpen, setIsDishPickerOpen] = useState(false);
  const [dishPickerSearch, setDishPickerSearch] = useState("");

  // Delete Confirm Dialog
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  const loadCombos = async () => {
    try {
      const response = await apiGet<{ combo_sets: ComboSetResponse[] }>("/combos", { page_id: 1, page_size: 50 });
      setCombos(response.combo_sets || []);
    } catch (error) {
      console.error("Failed to load combos", error);
    }
  };

  const loadResources = async () => {
    try {
      const [dishesRes, tagsRes] = await Promise.all([
        apiGet<{ dishes: DishResponse[] }>("/dishes", { page_id: 1, page_size: 50 }),
        apiGet<{ tags: TagInfo[] }>("/tags", { type: "dish" }) // Reusing dish tags as per miniprogram logic
      ]);
      setAllDishes(dishesRes.dishes || []);
      setAvailableTags(tagsRes.tags || []);
    } catch (error) {
      console.error("Failed to load resources", error);
    }
  };

  useEffect(() => {
    loadCombos();
    loadResources();
  }, []);

  const filteredCombos = useMemo(() => {
    if (!searchQuery) return combos;
    return combos.filter(c => c.name.toLowerCase().includes(searchQuery.toLowerCase()));
  }, [combos, searchQuery]);

  // Select Combo handler
  const handleSelectCombo = async (combo: ComboSetResponse) => {
    try {
      // Load full details
      const detail = await apiGet<ComboSetResponse>(`/combos/${combo.id}`);
      setSelectedCombo(detail);
      
      // Initialize form
      setIsAdding(false);
      setFormData(detail);
      setFormDishes(detail.dishes || []);
      setFormTagIds(detail.tags?.map(t => t.id) || []);
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "加载套餐详情失败";
      toast.error(message);
    }
  };

  // Add Combo handler
  const handleAddCombo = () => {
    setIsAdding(true);
    setSelectedCombo(null);
    setFormData({
      name: "",
      description: "",
      combo_price: 0,
      is_online: true,
      is_available: true,
    });
    setFormDishes([]);
    setFormTagIds([]);
  };

  // Dish Picker Helpers
  const filteredDishes = useMemo(() => {
    return allDishes.filter(d => 
      d.name.toLowerCase().includes(dishPickerSearch.toLowerCase())
    );
  }, [allDishes, dishPickerSearch]);

  const toggleDishInForm = (dish: DishResponse) => {
    setFormDishes(prev => {
      const exists = prev.find(d => d.dish_id === dish.id);
      if (exists) {
        return prev.filter(d => d.dish_id !== dish.id);
      } else {
        return [...prev, {
          dish_id: dish.id,
          dish_name: dish.name,
          dish_image_url: dish.image_url,
          dish_price: dish.price,
          quantity: 1
        }];
      }
    });
  };

  const updateDishQuantity = (dishId: number, delta: number) => {
    setFormDishes(prev => prev.map(d => {
      if (d.dish_id === dishId) {
        return { ...d, quantity: Math.max(1, d.quantity + delta) };
      }
      return d;
    }));
  };

  // Save Logic
  const handleSave = async () => {
    if (!formData.name?.trim()) {
      toast.error("请输入套餐名称");
      return;
    }
    if ((formData.combo_price || 0) <= 0) {
      toast.error("请输入有效价格");
      return;
    }

    setSaving(true);
    try {
      const payload: CreateComboRequest = {
        name: formData.name!,
        description: formData.description || "",
        combo_price: formData.combo_price!,
        is_online: formData.is_online,
        is_available: formData.is_available,
        dishes: formDishes.map(d => ({ dish_id: d.dish_id, quantity: d.quantity })),
        tag_ids: formTagIds,
      };

      if (isAdding) {
        await apiPost("/combos", payload);
        toast.success("套餐创建成功");
      } else if (selectedCombo) {
        await apiPut(`/combos/${selectedCombo.id}`, payload);
        toast.success("套餐更新成功");
      }
      
      // Refresh list
      loadCombos();
      
      // Reset editor if needed (optional, keeping selection for now)
      if (isAdding) {
        setIsAdding(false);
        setSelectedCombo(null);
        setFormData({});
      }
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "保存失败";
      toast.error(message);
    } finally {
      setSaving(false);
    }
  };

  // Delete Logic
  const handleDeleteClick = () => {
    if (!selectedCombo) return;
    setDeleteDialogOpen(true);
  };

  const handleDeleteConfirm = async () => {
    if (!selectedCombo) return;
    try {
      await apiDelete(`/combos/${selectedCombo.id}`);
      toast.success("套餐已删除");
      setSelectedCombo(null);
      setIsAdding(false);
      loadCombos();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "删除失败";
      toast.error(message);
    }
  };

  // Calculations
  const originalPrice = useMemo(() => {
    return formDishes.reduce((sum, item) => sum + (item.dish_price * item.quantity), 0);
  }, [formDishes]);

  const discountRate = useMemo(() => {
    if (!originalPrice || !formData.combo_price) return 0;
    return (((originalPrice - formData.combo_price) / originalPrice) * 100).toFixed(0);
  }, [originalPrice, formData.combo_price]);

  return (
    <PageShell>
      <PageHeader 
        title="套餐管理" 
        description="创建和管理超值套餐组合"
      />

      <PageContent>
        <div className="flex h-[calc(100vh-12rem)] gap-6">
          {/* Left Panel: Combo List */}
          <div className="w-1/3 min-w-80 flex flex-col bg-white rounded-xl border shadow-sm">
            <div className="p-4 border-b space-y-4">
              <div className="flex items-center justify-between">
                <h3 className="font-medium">套餐列表 ({combos.length})</h3>
                <Button size="sm" onClick={handleAddCombo}>
                  <Plus className="h-4 w-4 mr-2" />
                  新建套餐
                </Button>
              </div>
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input 
                  placeholder="搜索套餐..." 
                  className="pl-9 bg-slate-50 border-slate-200"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                />
              </div>
            </div>

            <ScrollArea className="flex-1 p-2">
              <div className="space-y-2">
                {filteredCombos.map(combo => (
                  <div 
                    key={combo.id}
                    onClick={() => handleSelectCombo(combo)}
                    className={cn(
                      "p-4 rounded-lg border transition-all cursor-pointer hover:border-primary/50 hover:bg-slate-50",
                      selectedCombo?.id === combo.id && !isAdding ? "border-primary bg-primary/5 ring-1 ring-primary" : "border-slate-100"
                    )}
                  >
                    <div className="flex justify-between items-start mb-2">
                      <span className="font-medium text-slate-900 line-clamp-1">{combo.name}</span>
                      <Badge variant={combo.is_online ? "default" : "secondary"} className="text-xs h-5 px-1.5">
                        {combo.is_online ? "在售" : "下架"}
                      </Badge>
                    </div>
                    <div className="flex justify-between items-baseline">
                      <span className="text-lg font-bold text-primary">
                        <span className="text-xs font-normal">¥</span>
                        {formatAmount(combo.combo_price)}
                      </span>
                      <span className="text-xs text-muted-foreground">ID: {combo.id}</span>
                    </div>
                  </div>
                ))}
                
                {filteredCombos.length === 0 && (
                  <div className="text-center py-10 text-muted-foreground text-sm">
                    暂无套餐数据
                  </div>
                )}
              </div>
            </ScrollArea>
          </div>

          {/* Right Panel: Editor */}
          <div className="flex-1 bg-white rounded-xl border shadow-sm flex flex-col">
            {!selectedCombo && !isAdding ? (
              <div className="flex-1 flex flex-col items-center justify-center text-muted-foreground">
                <div className="w-16 h-16 bg-slate-100 rounded-full flex items-center justify-center mb-4">
                  <Box className="w-8 h-8 text-slate-400" />
                </div>
                <p>选择左侧套餐查看详情，或点击新建</p>
              </div>
            ) : (
              <>
                {/* Editor Header */}
                <div className="flex items-center justify-between p-4 border-b">
                  <div className="flex items-center gap-4">
                    <h2 className="text-lg font-semibold">
                      {isAdding ? "新建套餐" : "编辑套餐"}
                    </h2>
                    {!isAdding && selectedCombo && (
                      <div className="flex gap-2">
                        <Button variant="outline" size="sm" onClick={() => handleSelectCombo(selectedCombo)}>
                          <RefreshCw className="h-3 w-3 mr-1" />
                          重置
                        </Button>
                        <Button variant="destructive" size="sm" onClick={handleDeleteClick}>
                          <Trash2 className="h-3 w-3 mr-1" />
                          删除
                        </Button>
                      </div>
                    )}
                  </div>
                  <Button onClick={handleSave} disabled={saving}>
                    {saving && <RefreshCw className="mr-2 h-4 w-4 animate-spin" />}
                    保存更改
                  </Button>
                </div>

                {/* Editor Content */}
                <ScrollArea className="flex-1">
                  <div className="p-6 space-y-8 max-w-3xl mx-auto">
                    
                    {/* Basic Info */}
                    <section className="space-y-4">
                      <h3 className="text-sm font-semibold text-slate-900 border-l-4 border-primary pl-3">基本信息</h3>
                      <div className="grid gap-4">
                        <div className="grid gap-2">
                          <Label>套餐名称 <span className="text-destructive">*</span></Label>
                          <Input 
                            placeholder="例如：双人豪华烤肉餐" 
                            value={formData.name || ""}
                            onChange={(e) => setFormData(p => ({ ...p, name: e.target.value }))}
                          />
                        </div>
                        <div className="grid gap-2">
                          <Label>套餐描述</Label>
                          <Textarea 
                            placeholder="简要描述套餐特色，吸引顾客..." 
                            className="h-20 resize-none"
                            value={formData.description || ""}
                            onChange={(e) => setFormData(p => ({ ...p, description: e.target.value }))}
                          />
                        </div>
                      </div>
                    </section>

                    {/* Pricing */}
                    <section className="space-y-4">
                      <h3 className="text-sm font-semibold text-slate-900 border-l-4 border-primary pl-3">价格与状态</h3>
                      <div className="grid grid-cols-2 gap-6">
                        <div className="grid gap-2">
                          <Label>套餐售价 (元) <span className="text-destructive">*</span></Label>
                          <div className="relative">
                            <span className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">¥</span>
                            <Input 
                              type="number"
                              className="pl-7"
                              placeholder="0.00"
                              value={formData.combo_price ? (formData.combo_price / 100).toString() : ""}
                              onChange={(e) => {
                                  const val = parseFloat(e.target.value);
                                  setFormData(p => ({ ...p, combo_price: isNaN(val) ? 0 : Math.round(val * 100) }));
                              }}
                            />
                          </div>
                          {Number(discountRate) > 0 && (
                            <div className="text-xs text-emerald-600 font-medium mt-1">
                              比原价节省 {discountRate}% (原价 ¥{formatAmount(originalPrice)})
                            </div>
                          )}
                        </div>
                        
                        <div className="grid gap-2">
                          <Label>售卖状态</Label>
                          <div className="flex items-center gap-2 h-10">
                            <Switch 
                              id="combo-online"
                              checked={formData.is_online}
                              onCheckedChange={(c) => setFormData(p => ({ ...p, is_online: c }))}
                            />
                            <Label htmlFor="combo-online" className="cursor-pointer font-normal">
                               {formData.is_online ? "上架销售" : "暂不售卖"}
                            </Label>
                          </div>
                        </div>
                      </div>
                    </section>

                    {/* Dishes */}
                    <section className="space-y-4">
                      <div className="flex items-center justify-between">
                        <h3 className="text-sm font-semibold text-slate-900 border-l-4 border-primary pl-3">套餐内容</h3>
                        <Button variant="outline" size="sm" onClick={() => setIsDishPickerOpen(true)}>
                          <Plus className="h-4 w-4 mr-2" />
                          添加菜品
                        </Button>
                      </div>

                      <div className="rounded-lg border bg-slate-50/50">
                        {formDishes.length === 0 ? (
                          <div className="p-8 text-center text-muted-foreground text-sm">
                            暂未添加任何菜品
                          </div>
                        ) : (
                          <div className="divide-y">
                            {formDishes.map((item) => (
                              <div key={item.dish_id} className="p-3 flex items-center justify-between group bg-white">
                                <div className="flex items-center gap-3">
                                  <div className="h-12 w-12 rounded bg-slate-100 flex items-center justify-center shrink-0 overflow-hidden border border-slate-200">
                                     {item.dish_image_url ? (
                                        <Image 
                                          src={formatImageUrl(getMediaUrl(item.dish_image_url))} 
                                          alt={item.dish_name} 
                                          width={48}
                                          height={48}
                                          className="h-full w-full object-cover"
                                        />
                                     ) : (
                                        <Utensils className="h-5 w-5 text-slate-400" />
                                     )}
                                  </div>
                                  <div className="min-w-0">
                                    <div className="font-medium truncate">{item.dish_name}</div>
                                    <div className="text-xs text-muted-foreground">单价: ¥{formatAmount(item.dish_price)}</div>
                                  </div>
                                </div>
                                <div className="flex items-center gap-4">
                                  <div className="flex items-center border rounded-md h-8 bg-white">
                                    <button 
                                      className="px-2.5 h-full hover:bg-slate-100 disabled:opacity-50"
                                      onClick={() => updateDishQuantity(item.dish_id, -1)}
                                      disabled={item.quantity <= 1}
                                    >-</button>
                                    <span className="w-8 text-center text-sm font-medium">{item.quantity}</span>
                                    <button 
                                      className="px-2.5 h-full hover:bg-slate-100"
                                      onClick={() => updateDishQuantity(item.dish_id, 1)}
                                    >+</button>
                                  </div>
                                  <Button 
                                    variant="ghost" 
                                    size="icon" 
                                    className="h-8 w-8 text-muted-foreground hover:text-destructive"
                                    onClick={() => setFormDishes(prev => prev.filter(d => d.dish_id !== item.dish_id))}
                                  >
                                    <XCircle className="h-4 w-4" />
                                  </Button>
                                </div>
                              </div>
                            ))}
                          </div>
                        )}
                        
                        {formDishes.length > 0 && (
                          <div className="p-3 bg-slate-100 text-right text-sm font-medium border-t">
                            原价总计：¥{formatAmount(originalPrice)}
                          </div>
                        )}
                      </div>
                    </section>

                    {/* Tags */}
                    <section className="space-y-4">
                       <h3 className="text-sm font-semibold text-slate-900 border-l-4 border-primary pl-3">属性标签</h3>
                       <div className="flex flex-wrap gap-2">
                          {availableTags.map(tag => {
                            const isSelected = formTagIds.includes(tag.id);
                            return (
                              <Badge 
                                key={tag.id}
                                variant={isSelected ? "default" : "outline"}
                                className={cn(
                                  "cursor-pointer px-3 py-1.5 transition-all text-sm font-normal",
                                  isSelected ? "border-primary" : "text-muted-foreground hover:bg-slate-100"
                                )}
                                onClick={() => {
                                  setFormTagIds(prev => 
                                    isSelected ? prev.filter(id => id !== tag.id) : [...prev, tag.id]
                                  );
                                }}
                              >
                                {tag.name}
                                {isSelected && <CheckCircle2 className="ml-1.5 h-3.5 w-3.5" />}
                              </Badge>
                            );
                          })}
                          {availableTags.length === 0 && (
                            <span className="text-sm text-muted-foreground">暂无标签可用 (请在菜品管理中添加)</span>
                          )}
                       </div>
                    </section>

                  </div>
                </ScrollArea>
              </>
            )}
          </div>
        </div>
      </PageContent>
      {/* Dish Picker Overlay */}
      {isDishPickerOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
          <div className="bg-white rounded-xl shadow-xl w-full max-w-lg max-h-[80vh] flex flex-col">
            <div className="p-4 border-b flex items-center justify-between">
              <h3 className="font-semibold text-lg">选择包含菜品</h3>
              <Button variant="ghost" size="icon" onClick={() => setIsDishPickerOpen(false)}>
                <XCircle className="h-5 w-5" />
              </Button>
            </div>
            
            <div className="p-3 border-b bg-slate-50">
               <Input 
                 placeholder="搜索菜品..." 
                 value={dishPickerSearch}
                 onChange={(e) => setDishPickerSearch(e.target.value)}
                 className="bg-white"
               />
            </div>

            <ScrollArea className="flex-1">
              <div className="divide-y">
                {filteredDishes.map(dish => {
                  const isSelected = formDishes.some(d => d.dish_id === dish.id);
                  return (
                    <div 
                      key={dish.id} 
                      className={cn(
                        "p-3 flex items-center justify-between hover:bg-slate-50 cursor-pointer",
                        isSelected && "bg-primary/5"
                      )}
                      onClick={() => toggleDishInForm(dish)}
                    >
                      <div className="flex items-center gap-3">
                        <div className="h-10 w-10 rounded bg-slate-100 flex items-center justify-center shrink-0 overflow-hidden border border-slate-200">
                           {dish.image_url ? (
                              <Image 
                                src={formatImageUrl(getMediaUrl(dish.image_url))} 
                                alt={dish.name} 
                                width={40}
                                height={40}
                                className="h-full w-full object-cover"
                              />
                           ) : (
                              <Utensils className="h-5 w-5 text-slate-400" />
                           )}
                        </div>
                        <div>
                          <div className="font-medium text-slate-800">{dish.name}</div>
                          <div className="text-xs text-muted-foreground">¥{formatAmount(dish.price)}</div>
                        </div>
                      </div>
                      {isSelected && <CheckCircle2 className="h-5 w-5 text-primary" />}
                    </div>
                  );
                })}
                {filteredDishes.length === 0 && (
                  <div className="py-8 text-center text-muted-foreground">未找到菜品</div>
                )}
              </div>
            </ScrollArea>

            <div className="p-4 border-t flex justify-end bg-slate-50 rounded-b-xl">
              <Button onClick={() => setIsDishPickerOpen(false)}>
                完成选择 ({formDishes.length})
              </Button>
            </div>
          </div>
        </div>
      )}
      {/* Delete Confirm Dialog */}
      <ConfirmDialog
        open={deleteDialogOpen}
        onOpenChange={setDeleteDialogOpen}
        title="删除套餐"
        description={`确定要删除套餐 "${selectedCombo?.name}" 吗？此操作不可撤销。`}
        confirmText="删除"
        variant="destructive"
        onConfirm={handleDeleteConfirm}
      />
    </PageShell>
  );
}
