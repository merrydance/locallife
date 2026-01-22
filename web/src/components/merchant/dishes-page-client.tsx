"use client";

import React, { useState, useEffect, useCallback, useRef } from "react";
import { Plus, Search, MoreHorizontal, Edit, Trash2, ChevronRight, Upload, GripVertical, Check, X, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { PageShell, PageHeader, PageContent } from "@/components/merchant/layout/page-shell";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Input } from "@/components/ui/input";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { Separator } from "@/components/ui/separator";
import { ScrollArea } from "@/components/ui/scroll-area";
import { 
  Sheet, 
  SheetContent, 
  SheetHeader, 
  SheetTitle, 
  SheetFooter,
  SheetDescription
} from "@/components/ui/sheet";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { 
  apiGet,
  apiPost,
  apiPut,
  apiDelete,
  apiPatch,
  apiUpload,
  formatAmount,
  getMediaUrl,
  formatImageUrl 
} from "@/lib/api";
import { 
  DishResponse, 
  DishCategory, 
  TagInfo, 
  CustomizationGroup,
  CustomizationOption,
  CreateDishRequest 
} from "@/types/dish";
import { cn } from "@/lib/utils";

export function DishesPageClient() {
  // --- State ---
  const [categories, setCategories] = useState<DishCategory[]>([]);
  const [dishes, setDishes] = useState<DishResponse[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeCategoryId, setActiveCategoryId] = useState<number | "all">("all");
  const [searchKeyword, setSearchKeyword] = useState("");
  
  // Selection / Editing
  const [isEditing, setIsEditing] = useState(false);
  const [editDish, setEditDish] = useState<Partial<DishResponse>>({});
  const [saving, setSaving] = useState(false);
  
  // Multi-select
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const [isMultiSelectMode, setIsMultiSelectMode] = useState(false);

  // Tags & Customizations
  const [availableTags, setAvailableTags] = useState<TagInfo[]>([]);
  const [selectedTagIds, setSelectedTagIds] = useState<number[]>([]);
  const [customizationGroups, setCustomizationGroups] = useState<Partial<CustomizationGroup>[]>([]);
  const [availableCustomizationTags, setAvailableCustomizationTags] = useState<TagInfo[]>([]);

  // UI Refs
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [uploadingImage, setUploadingImage] = useState(false);

  // Category Manager
  const [isAddingCategory, setIsAddingCategory] = useState(false);
  const [newCatName, setNewCatName] = useState("");

  // Confirm Dialog States
  const [deleteDishDialog, setDeleteDishDialog] = useState<{ open: boolean; id: number | null }>({ open: false, id: null });
  const [deleteCategoryDialog, setDeleteCategoryDialog] = useState<{ open: boolean; id: number | null; name: string }>({ open: false, id: null, name: "" });

  // --- Effects ---
  const loadCategories = useCallback(async () => {
    try {
      const res = await apiGet<{ categories: DishCategory[] }>("/dishes/categories");
      setCategories(res.categories || []);
    } catch (err) {
      console.error("Failed to load categories", err);
    }
  }, []);

  const loadDishes = useCallback(async () => {
    setLoading(true);
    try {
      const res = await apiGet<{ dishes: DishResponse[] }>("/dishes", {
        page_id: 1,
        page_size: 50,
        ...(activeCategoryId !== "all" ? { category_id: activeCategoryId } : {})
      });
      setDishes(res.dishes || []);
    } catch (err) {
      console.error("Failed to load dishes", err);
    } finally {
      setLoading(false);
    }
  }, [activeCategoryId]);

  const loadTags = useCallback(async () => {
    try {
      const res = await apiGet<{ tags: TagInfo[] }>("/tags", { type: "dish" });
      setAvailableTags(res.tags || []);
      const res2 = await apiGet<{ tags: TagInfo[] }>("/tags", { type: "customization" });
      setAvailableCustomizationTags(res2.tags || []);
    } catch (err) {
      console.error("Failed to load tags", err);
    }
  }, []);

  useEffect(() => {
    loadCategories();
    loadTags();
  }, [loadCategories, loadTags]);

  useEffect(() => {
    loadDishes();
  }, [loadDishes]);

  // --- Handlers ---
  const handleAddDish = () => {
    setEditDish({
      name: "",
      price: 0,
      description: "",
      is_online: true,
      is_available: true,
      category_id: activeCategoryId === "all" ? undefined : activeCategoryId,
      image_url: ""
    });
    setSelectedTagIds([]);
    setCustomizationGroups([]);
    setIsEditing(true);
  };

  const handleEditDish = async (dish: DishResponse) => {
    try {
      const detail = await apiGet<DishResponse>(`/dishes/${dish.id}`);
      setEditDish(detail);
      setSelectedTagIds(detail.tags?.map(t => t.id) || []);
      setCustomizationGroups(detail.customization_groups || []);
      setIsEditing(true);
    } catch (err) {
      console.error("Failed to load dish details", err);
      // Fallback
      setEditDish(dish);
      setSelectedTagIds([]);
      setCustomizationGroups([]);
      setIsEditing(true);
    }
  };

  const handleSaveDish = async () => {
    if (!editDish.name?.trim()) return;
    if (editDish.price === undefined || editDish.price < 0) return;

    setSaving(true);
    try {
      const payload: CreateDishRequest = {
        name: editDish.name,
        description: editDish.description || "",
        price: editDish.price,
        member_price: editDish.member_price,
        category_id: editDish.category_id,
        is_online: editDish.is_online,
        is_available: editDish.is_available,
        image_url: editDish.image_url,
        tag_ids: selectedTagIds,
        customization_groups: customizationGroups.map((g, idx) => ({
          name: g.name || "",
          is_required: !!g.is_required,
          sort_order: g.sort_order ?? idx,
          options: (g.options || []).map((o, oidx) => ({
            tag_id: o.tag_id,
            extra_price: o.extra_price || 0,
            sort_order: o.sort_order ?? oidx
          }))
        }))
      };

      if (editDish.id) {
        await apiPut(`/dishes/${editDish.id}`, payload as any);
        // Backend updateDish doesn't include customizations, update them separately
        await apiPut(`/dishes/${editDish.id}/customizations`, { 
          groups: payload.customization_groups 
        });
      } else {
        await apiPost("/dishes", payload as any);
      }
      
      setIsEditing(false);
      loadDishes();
    } catch (err) {
      console.error("Failed to save dish", err);
    } finally {
      setSaving(false);
    }
  };

  const handleDeleteDish = async (id: number) => {
    setDeleteDishDialog({ open: true, id });
  };

  const confirmDeleteDish = async () => {
    if (!deleteDishDialog.id) return;
    try {
      await apiDelete(`/dishes/${deleteDishDialog.id}`);
      toast.success("菜品已删除");
      loadDishes();
    } catch (err) {
      toast.error("删除失败");
      console.error("Failed to delete dish", err);
    }
  };

  const handleImageUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    setUploadingImage(true);
    try {
      const res = await apiUpload<{ image_url: string }>("/dishes/images/upload", file);
      setEditDish(prev => ({ ...prev, image_url: res.image_url }));
    } catch (err) {
      console.error("Upload failed", err);
    } finally {
      setUploadingImage(false);
    }
  };

  // --- Customization Helpers ---
  const addGroup = () => {
    setCustomizationGroups([...customizationGroups, { name: "", is_required: false, options: [] }]);
  };

  const removeGroup = (index: number) => {
    setCustomizationGroups(customizationGroups.filter((_, i) => i !== index));
  };

  const addOption = (groupIndex: number, tagId: number) => {
    const tag = availableCustomizationTags.find(t => t.id === tagId);
    if (!tag) return;

    const newGroups = [...customizationGroups];
    const group = { ...newGroups[groupIndex] };
    if (!group.options) group.options = [];
    
    if (group.options.find(o => o.tag_id === tagId)) return;

    group.options = [...group.options, { tag_id: tagId, tag_name: tag.name, extra_price: 0, sort_order: group.options.length, id: 0 }];
    newGroups[groupIndex] = group;
    setCustomizationGroups(newGroups);
  };

  const removeOption = (groupIndex: number, optionIndex: number) => {
    const newGroups = [...customizationGroups];
    const group = { ...newGroups[groupIndex] };
    group.options = group.options?.filter((_, i) => i !== optionIndex);
    newGroups[groupIndex] = group;
    setCustomizationGroups(newGroups);
  };

  const updateOptionPrice = (groupIndex: number, optionIndex: number, price: number) => {
    const newGroups = [...customizationGroups];
    const group = { ...newGroups[groupIndex] };
    if (!group.options) return;
    const option = { ...group.options[optionIndex], extra_price: price };
    group.options = [...group.options];
    group.options[optionIndex] = option;
    newGroups[groupIndex] = group;
    setCustomizationGroups(newGroups);
  };

  // --- Rendering ---
  const filteredDishes = dishes.filter(dish => 
    dish.name.toLowerCase().includes(searchKeyword.toLowerCase())
  );

  return (
    <PageShell>
      <PageHeader
        title="菜品管理"
        description="管理您的菜单，设置价格、描述和上架状态。"
        actions={
          <div className="flex items-center gap-2">
            {isMultiSelectMode ? (
              <>
                <Button variant="outline" size="sm" onClick={() => handleBatchToggleVisibility?.(true)}>批量上架</Button>
                <Button variant="outline" size="sm" onClick={() => handleBatchToggleVisibility?.(false)}>批量下架</Button>
                <Button variant="ghost" size="sm" onClick={() => { setIsMultiSelectMode(false); setSelectedIds([]); }}>取消</Button>
              </>
            ) : (
              <>
                <Button variant="outline" size="sm" onClick={() => setIsMultiSelectMode(true)}>批量操作</Button>
                <Button size="sm" onClick={handleAddDish}><Plus className="mr-2 h-4 w-4" />新增菜品</Button>
              </>
            )}
          </div>
        }
      />

      <PageContent>
        <div className="flex gap-6 h-[calc(100vh-12rem)]">
          {/* Categories Sidebar */}
          <div className="w-60 flex flex-col bg-white rounded-xl border shadow-sm">
            {/* 分类面板头部 */}
            <div className="p-4 border-b">
              <div className="flex items-center justify-between">
                <h3 className="font-medium">菜品分类</h3>
                <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => setIsAddingCategory(true)}><Plus className="h-4 w-4" /></Button>
              </div>
            </div>
            {/* 分类列表 */}
            <ScrollArea className="flex-1 p-2">
              <div className="space-y-1">
                <div 
                  onClick={() => setActiveCategoryId("all")}
                  className={cn(
                    "px-3 py-2 rounded-lg cursor-pointer transition-all flex items-center justify-between",
                    activeCategoryId === "all" 
                      ? "bg-primary/5 border border-primary text-primary" 
                      : "hover:bg-slate-50 border border-transparent"
                  )}
                >
                  <span className="font-medium">全部菜品</span>
                  <Badge variant="outline" className="h-5 text-xs">{dishes.length}</Badge>
                </div>
                {categories.map(cat => (
                  <div 
                    key={cat.id} 
                    onClick={() => setActiveCategoryId(cat.id)}
                    className={cn(
                      "px-3 py-2 rounded-lg cursor-pointer transition-all flex items-center justify-between group",
                      activeCategoryId === cat.id 
                        ? "bg-primary/5 border border-primary text-primary" 
                        : "hover:bg-slate-50 border border-transparent"
                    )}
                  >
                    <span className="font-medium truncate">{cat.name}</span>
                    <Button 
                      variant="ghost" 
                      size="icon" 
                      className="h-6 w-6 opacity-0 group-hover:opacity-100 transition-opacity hover:bg-destructive/10 hover:text-destructive" 
                      onClick={(e) => { e.stopPropagation(); handleDeleteCategory(cat.id, cat.name); }}
                    >
                      <Trash2 className="h-3 w-3" />
                    </Button>
                  </div>
                ))}
              </div>
              {isAddingCategory && (
                <div className="mt-3 p-3 border rounded-lg space-y-2 bg-slate-50">
                  <Input placeholder="新分类名称" value={newCatName} onChange={(e) => setNewCatName(e.target.value)} className="h-8 text-sm bg-white" autoFocus />
                  <div className="flex gap-2">
                    <Button size="sm" className="flex-1 h-7 text-xs" onClick={handleCreateCategory}>确认</Button>
                    <Button size="sm" variant="outline" className="flex-1 h-7 text-xs" onClick={() => setIsAddingCategory(false)}>取消</Button>
                  </div>
                </div>
              )}
            </ScrollArea>
          </div>

          {/* Main Dish View */}
          <div className="flex-1 flex flex-col bg-white rounded-xl border shadow-sm">
            {/* 搜索栏 */}
            <div className="p-4 border-b">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input placeholder="搜索菜品名称..." className="pl-9 bg-slate-50 border-slate-200" value={searchKeyword} onChange={(e) => setSearchKeyword(e.target.value)} />
              </div>
            </div>
            <ScrollArea className="flex-1 p-4">
              {loading ? (
                <div className="flex items-center justify-center h-40 text-muted-foreground">加载中...</div>
              ) : filteredDishes.length === 0 ? (
                <div className="flex items-center justify-center h-40 text-muted-foreground italic">暂无菜品</div>
              ) : (
                <div className="grid grid-cols-1 md:grid-cols-2 2xl:grid-cols-3 gap-4 pb-6">
                  {filteredDishes.map(dish => (
                    <Card 
                      key={dish.id} 
                      className={cn(
                        "flex flex-row h-28 items-stretch overflow-hidden p-0 gap-0 transition-all hover:shadow-md cursor-pointer border-muted/60 group relative",
                        !dish.is_online && "opacity-80 grayscale-[0.1] bg-muted/20"
                      )}
                      onClick={() => isMultiSelectMode ? setSelectedIds(prev => prev.includes(dish.id) ? prev.filter(id => id !== dish.id) : [...prev, dish.id]) : handleEditDish(dish)}
                    >
                      {/* Left: Image (Flush to 3 sides: top, bottom, left) */}
                      <div className="relative w-32 shrink-0 bg-muted border-r border-muted/50 overflow-hidden">
                        {dish.image_url ? (
                          <img 
                            src={formatImageUrl(getMediaUrl(dish.image_url))} 
                            alt={dish.name} 
                            className="w-full h-full object-cover transition-transform duration-300 group-hover:scale-105" 
                          />
                        ) : (
                          <div className="w-full h-full flex flex-col items-center justify-center gap-1 text-muted-foreground/30">
                            <Upload className="h-6 w-6 stroke-[1.5]" />
                            <span className="text-[10px]">暂无图</span>
                          </div>
                        )}
                        
                        {!dish.is_online && (
                          <div className="absolute inset-0 bg-background/50 backdrop-blur-[1px] flex items-center justify-center">
                            <Badge variant="secondary" className="bg-background/90 text-[10px] py-0 h-4 scale-90 border-muted-foreground/20">已下架</Badge>
                          </div>
                        )}
                      </div>
                      
                      {/* Right: Content Area (using CardContent but overriding its default padding) */}
                      <CardContent className="flex flex-col flex-1 p-3 px-3 py-3 justify-between min-w-0 bg-card gap-0">
                        <div className="space-y-1 overflow-hidden">
                          <div className="flex items-center justify-between gap-2">
                            <h4 className="font-bold text-sm truncate flex-1">{dish.name}</h4>
                            {!dish.is_available && (
                              <Badge variant="destructive" className="h-4 px-1 text-[9px] font-normal shrink-0">缺货</Badge>
                            )}
                          </div>
                          
                          <p className="text-[11px] text-muted-foreground line-clamp-1">
                            {dish.description || "暂无菜品描述"}
                          </p>
                          
                          <div className="flex flex-wrap gap-1 mt-1">
                            {dish.tags?.slice(0, 2).map(tag => (
                              <Badge key={tag.id} variant="outline" className="text-[9px] h-3.5 px-1 font-normal border-muted/50 bg-muted/10 opacity-80">{tag.name}</Badge>
                            ))}
                          </div>
                        </div>
                        
                        <div className="flex items-end justify-between mt-auto">
                          <div className="flex flex-col">
                            <span className="text-primary font-bold text-base leading-none">¥{formatAmount(dish.price)}</span>
                            {dish.member_price && (
                              <span className="text-[9px] text-muted-foreground mt-0.5">会员 ¥{formatAmount(dish.member_price)}</span>
                            )}
                          </div>
                          
                          <div className="flex gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity translate-x-1">
                            <Button variant="ghost" size="icon" className="h-7 w-7 hover:bg-primary/10 hover:text-primary rounded-full" onClick={(e) => { e.stopPropagation(); handleEditDish(dish); }}>
                              <Edit className="h-3.5 w-3.5" />
                            </Button>
                            <Button variant="ghost" size="icon" className="h-7 w-7 hover:bg-destructive/10 hover:text-destructive rounded-full" onClick={(e) => { e.stopPropagation(); handleDeleteDish(dish.id); }}>
                              <Trash2 className="h-3.5 w-3.5" />
                            </Button>
                          </div>
                        </div>
                      </CardContent>

                      {/* Card Top-Right Checkbox */}
                      {isMultiSelectMode && (
                        <div className="absolute top-2 right-2 z-10 bg-background/95 backdrop-blur-sm p-1.5 rounded-md border shadow-sm ring-1 ring-primary/10" onClick={(e) => e.stopPropagation()}>
                          <Checkbox 
                            checked={selectedIds.includes(dish.id)} 
                            onCheckedChange={() => setSelectedIds(prev => prev.includes(dish.id) ? prev.filter(id => id !== dish.id) : [...prev, dish.id])} 
                          />
                        </div>
                      )}
                    </Card>
                  ))}
                </div>
              )}
            </ScrollArea>
          </div>
        </div>
      </PageContent>

      <Sheet open={isEditing} onOpenChange={setIsEditing}>
        <SheetContent className="sm:max-w-2xl overflow-y-auto">
          <SheetHeader className="mb-6">
            <SheetTitle>{editDish.id ? "编辑菜品" : "新增菜品"}</SheetTitle>
            <SheetDescription>管理菜品的基础信息、属性标签和定制规格。</SheetDescription>
          </SheetHeader>

          <div className="space-y-8 py-4 pb-20">
            {/* Image Section */}
            <div className="space-y-3">
              <Label>菜品图片</Label>
              <div className="flex gap-4 items-start">
                <div 
                  className="w-32 h-32 rounded-lg border-2 border-dashed border-muted flex flex-col items-center justify-center relative bg-muted/20 hover:bg-muted/30 transition-colors cursor-pointer overflow-hidden"
                  onClick={() => fileInputRef.current?.click()}
                >
                  {editDish.image_url ? (
                    <img src={formatImageUrl(getMediaUrl(editDish.image_url))} className="w-full h-full object-cover" />
                  ) : (
                    <>
                      {uploadingImage ? <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" /> : <Upload className="h-8 w-8 text-muted-foreground mb-1" />}
                      <span className="text-[10px] text-muted-foreground text-center px-2">点击上传图片</span>
                    </>
                  )}
                </div>
                {editDish.image_url && (
                  <Button variant="ghost" size="sm" className="text-destructive h-8 px-2" onClick={() => setEditDish({...editDish, image_url: ""})}><Trash2 className="h-4 w-4 mr-1"/>移除</Button>
                )}
                <input type="file" className="hidden" ref={fileInputRef} accept="image/*" onChange={handleImageUpload} />
              </div>
            </div>

            {/* Basic Info */}
            <div className="grid gap-4">
              <div className="grid gap-2">
                <Label>菜品名称 *</Label>
                <Input value={editDish.name} onChange={(e) => setEditDish({...editDish, name: e.target.value})} placeholder="例如: 招牌红烧肉" />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div className="grid gap-2">
                  <Label>价格 (¥) *</Label>
                  <Input type="number" step="0.01" value={editDish.price !== undefined ? editDish.price / 100 : ""} onChange={(e) => {
                    const val = parseFloat(e.target.value);
                    setEditDish({...editDish, price: isNaN(val) ? 0 : Math.round(val * 100)});
                  }} placeholder="0.00" />
                </div>
                <div className="grid gap-2">
                  <Label>会员价 (¥)</Label>
                  <Input type="number" step="0.01" value={editDish.member_price !== undefined ? editDish.member_price / 100 : ""} onChange={(e) => {
                    const val = parseFloat(e.target.value);
                    setEditDish({...editDish, member_price: isNaN(val) ? undefined : Math.round(val * 100)});
                  }} placeholder="如果不设置则不启用" />
                </div>
              </div>

              <div className="grid gap-2">
                <Label>所属分类</Label>
                <Select value={editDish.category_id?.toString()} onValueChange={(val) => setEditDish({...editDish, category_id: parseInt(val)})}>
                  <SelectTrigger><SelectValue placeholder="选择分类" /></SelectTrigger>
                  <SelectContent>{categories.map(cat => <SelectItem key={cat.id} value={cat.id.toString()}>{cat.name}</SelectItem>)}</SelectContent>
                </Select>
              </div>

              <div className="grid gap-2">
                <Label>菜品描述</Label>
                <Textarea value={editDish.description} onChange={(e) => setEditDish({...editDish, description: e.target.value})} placeholder="向顾客介绍您的美味佳肴..." rows={3} />
              </div>
            </div>

            <Separator />

            {/* Tags Section */}
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <Label className="text-base">属性标签</Label>
                <span className="text-xs text-muted-foreground">如：辣度、口味等</span>
              </div>
              <div className="flex flex-wrap gap-2">
                {availableTags.map(tag => (
                  <Button
                    key={tag.id}
                    variant={selectedTagIds.includes(tag.id) ? "default" : "outline"}
                    size="sm"
                    className="rounded-full h-8"
                    onClick={() => setSelectedTagIds(prev => prev.includes(tag.id) ? prev.filter(id => id !== tag.id) : [...prev, tag.id])}
                  >
                    {tag.name}
                  </Button>
                ))}
              </div>
            </div>

            <Separator />

            {/* Customizations Section */}
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <Label className="text-base">规格/加料定制</Label>
                <Button variant="outline" size="sm" onClick={addGroup}><Plus className="h-4 w-4 mr-1" />添加分组</Button>
              </div>
              
              {customizationGroups.length === 0 ? (
                <div className="text-sm text-muted-foreground italic border rounded-lg p-6 text-center bg-muted/10">暂无定制选项。可以添加如“冷热”、“加料”等分组。</div>
              ) : (
                <div className="space-y-4">
                  {customizationGroups.map((group, gidx) => (
                    <div key={gidx} className="border rounded-lg p-4 space-y-4 relative bg-muted/5">
                      <Button variant="ghost" size="icon" className="h-6 w-6 absolute top-2 right-2 text-muted-foreground hover:text-destructive" onClick={() => removeGroup(gidx)}><X className="h-4 w-4" /></Button>
                      
                      <div className="grid grid-cols-[1fr_auto] gap-4 pr-6">
                        <div className="space-y-2">
                          <Label className="text-xs">分组名称</Label>
                          <Input value={group.name} onChange={(e) => {
                            const newGroups = [...customizationGroups];
                            newGroups[gidx] = { ...group, name: e.target.value };
                            setCustomizationGroups(newGroups);
                          }} placeholder="例如: 辣度, 加料" className="h-8" />
                        </div>
                        <div className="space-y-2">
                          <Label className="text-xs">必选</Label>
                          <div className="flex items-center h-8"><Checkbox checked={group.is_required} onCheckedChange={(val) => {
                            const newGroups = [...customizationGroups];
                            newGroups[gidx] = { ...group, is_required: !!val };
                            setCustomizationGroups(newGroups);
                          }} /></div>
                        </div>
                      </div>

                      <div className="space-y-2">
                        <Label className="text-xs">选项内容</Label>
                        <div className="space-y-2">
                          {group.options?.map((opt, oidx) => (
                            <div key={oidx} className="flex items-center gap-2 bg-background p-2 rounded border group/opt">
                              <span className="text-sm font-medium flex-1 pl-1">{opt.tag_name}</span>
                              <div className="flex items-center gap-2">
                                <span className="text-xs text-muted-foreground">+¥</span>
                                <Input 
                                  type="number" 
                                  step="0.01" 
                                  className="w-20 h-7 text-xs" 
                                  value={opt.extra_price / 100} 
                                  onChange={(e) => updateOptionPrice(gidx, oidx, Math.round(parseFloat(e.target.value) * 100))}
                                />
                                <Button variant="ghost" size="icon" className="h-6 w-6 opacity-0 group-hover/opt:opacity-100" onClick={() => removeOption(gidx, oidx)}><X className="h-3 w-3" /></Button>
                              </div>
                            </div>
                          ))}
                          
                          <div className="flex flex-wrap gap-1 mt-2">
                            <Select onValueChange={(val) => addOption(gidx, parseInt(val))}>
                              <SelectTrigger className="h-7 w-auto min-w-24 text-[10px]"><SelectValue placeholder="+ 添加选项" /></SelectTrigger>
                              <SelectContent>
                                {availableCustomizationTags.map(tag => <SelectItem key={tag.id} value={tag.id.toString()}>{tag.name}</SelectItem>)}
                              </SelectContent>
                            </Select>
                          </div>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>

            <Separator />

            {/* Availability */}
            <div className="grid grid-cols-2 gap-6 pb-4">
              <div className="flex items-center space-x-2">
                <Checkbox id="is_online" checked={editDish.is_online} onCheckedChange={(val) => setEditDish({...editDish, is_online: !!val})} />
                <Label htmlFor="is_online" className="cursor-pointer">显示在菜单上 (上架)</Label>
              </div>
              <div className="flex items-center space-x-2">
                <Checkbox id="is_available" checked={editDish.is_available} onCheckedChange={(val) => setEditDish({...editDish, is_available: !!val})} />
                <Label htmlFor="is_available" className="cursor-pointer">当前可售 (有货)</Label>
              </div>
            </div>
          </div>

          <SheetFooter className="fixed bottom-0 right-0 left-0 bg-background border-t p-4 z-10 sm:relative sm:border-t-0 sm:bg-transparent sm:p-0">
            <Button className="w-full flex-1" onClick={handleSaveDish} disabled={saving || uploadingImage}>
              {saving ? <><Loader2 className="mr-2 h-4 w-4 animate-spin" /> 保存中...</> : "完成并保存"}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      {/* Confirm Dialogs */}
      <ConfirmDialog
        open={deleteDishDialog.open}
        onOpenChange={(open) => setDeleteDishDialog({ open, id: open ? deleteDishDialog.id : null })}
        title="删除菜品"
        description="确定要删除此菜品吗？此操作不可撤销。"
        confirmText="删除"
        variant="destructive"
        onConfirm={confirmDeleteDish}
      />
      <ConfirmDialog
        open={deleteCategoryDialog.open}
        onOpenChange={(open) => setDeleteCategoryDialog({ open, id: open ? deleteCategoryDialog.id : null, name: open ? deleteCategoryDialog.name : "" })}
        title="删除分类"
        description={`确定要删除分类「${deleteCategoryDialog.name}」吗？菜品将变为未分类。`}
        confirmText="删除"
        variant="destructive"
        onConfirm={async () => {
          if (!deleteCategoryDialog.id) return;
          try {
            await apiDelete(`/dishes/categories/${deleteCategoryDialog.id}`);
            if (activeCategoryId === deleteCategoryDialog.id) setActiveCategoryId("all");
            toast.success("分类已删除");
            loadCategories();
            loadDishes();
          } catch (err) {
            toast.error("删除失败");
            console.error(err);
          }
        }}
      />
    </PageShell>
  );

  // --- Helpers for Category Manager ---
  async function handleCreateCategory() {
    if (!newCatName.trim()) return;
    try {
      await apiPost("/dishes/categories", { name: newCatName });
      setNewCatName("");
      setIsAddingCategory(false);
      loadCategories();
    } catch (err) { console.error(err); }
  }

  async function handleDeleteCategory(id: number, name: string) {
    setDeleteCategoryDialog({ open: true, id, name });
  }

  async function handleBatchToggleVisibility(is_online: boolean) {
    if (selectedIds.length === 0) return;
    try {
      await apiPatch("/dishes/batch/status", { dish_ids: selectedIds, is_online });
      setSelectedIds([]);
      setIsMultiSelectMode(false);
      loadDishes();
    } catch (err) { console.error(err); }
  }
}
