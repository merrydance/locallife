"use client";

import React, { useState, useEffect, useCallback, useMemo } from "react";
import Image from "next/image";
import { 
  Search, 
  RefreshCw, 
  Save, 
  Infinity, 
  AlertCircle,
  CheckCircle2,
  Package,
  PackageCheck,
  PackageX,
  Loader2
} from "lucide-react";
import { toast } from "sonner";
import { PageShell, PageHeader, PageContent } from "@/components/merchant/layout/page-shell";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  apiGet,
  apiPost,
  apiPut,
  formatAmount,
  getMediaUrl,
  formatImageUrl
} from "@/lib/api";
import { DishResponse, DishCategory } from "@/types/dish";
import { InventoryItem, InventoryStats, ListInventoryResponse } from "@/types/inventory";
import { cn } from "@/lib/utils";

// 辅助函数：格式化今日日期为 YYYY-MM-DD
function getTodayDateString() {
  const now = new Date();
  const year = now.getFullYear();
  const month = String(now.getMonth() + 1).padStart(2, "0");
  const day = String(now.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

export function InventoryPageClient() {
  // --- 状态 ---
  const [categories, setCategories] = useState<DishCategory[]>([]);
  const [dishes, setDishes] = useState<DishResponse[]>([]);
  const [inventories, setInventories] = useState<Record<number, InventoryItem>>({});
  const [stats, setStats] = useState<InventoryStats | null>(null);
  
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [activeCategoryId, setActiveCategoryId] = useState<number | "all">("all");
  const [searchKeyword, setSearchKeyword] = useState("");
  const [selectedDate, setSelectedDate] = useState(getTodayDateString());

  // 修改追踪：dishId -> totalQuantity
  const [pendingChanges, setPendingChanges] = useState<Record<number, number>>({});

  // --- 数据加载 ---
  const loadData = useCallback(async (date: string) => {
    setLoading(true);
    try {
      // 1. 加载分类
      const catRes = await apiGet<{ categories: DishCategory[] }>("/dishes/categories");
      setCategories(catRes.categories || []);

      const dishRes = await apiGet<{ dishes: DishResponse[] }>("/dishes", {
        page_id: 1,
        page_size: 50,
      });
      setDishes(dishRes.dishes || []);

      // 3. 加载选定日期的库存
      const invRes = await apiGet<ListInventoryResponse>("/inventory", { date });
      const invMap: Record<number, InventoryItem> = {};
      (invRes.inventories || []).forEach(item => {
        invMap[item.dish_id] = item;
      });
      setInventories(invMap);

      // 4. 加载统计
      const statsRes = await apiGet<InventoryStats>("/inventory/stats", { date });
      setStats(statsRes);

      // 清空待保存修改
      setPendingChanges({});
    } catch (err) {
      console.error("Failed to load inventory data", err);
      toast.error("加载数据失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadData(selectedDate);
  }, [loadData, selectedDate]);

  // --- 交互处理 ---
  const handleInventoryChange = (dishId: number, value: string) => {
    let quantity: number;
    if (value === "" || value === "-1") {
      quantity = -1;
    } else {
      quantity = parseInt(value, 10);
      if (isNaN(quantity) || quantity < -1) return;
    }

    const currentInv = inventories[dishId];
    const originalQuantity = currentInv ? currentInv.total_quantity : -1;

    if (quantity === originalQuantity) {
      const newChanges = { ...pendingChanges };
      delete newChanges[dishId];
      setPendingChanges(newChanges);
    } else {
      setPendingChanges(prev => ({
        ...prev,
        [dishId]: quantity
      }));
    }
  };

  const handleToggleUnlimited = (dishId: number, isUnlimited: boolean) => {
    handleInventoryChange(dishId, isUnlimited ? "-1" : "0");
  };

  const handleSaveChanges = async () => {
    const changeCount = Object.keys(pendingChanges).length;
    if (changeCount === 0) return;

    setSaving(true);
    let successCount = 0;
    let failCount = 0;

    try {
      // 本端后端目前没有提供批量更新接口，我们需要逐个更新（对齐小程序逻辑）
      // 虽然可以并发，但为了安全起见串行或小批量并行
      const promises = Object.entries(pendingChanges).map(async ([dishIdStr, quantity]) => {
        const dishId = parseInt(dishIdStr);
        try {
          // 对齐后端 api/inventory.go 的 updateDailyInventory (PUT /v1/inventory)
          // 逻辑是：如果不存在则报错 404，但小程序端用的是 setInventory (尝试 POST，失败则 PATCH)
          // 这里我们采用类似的策略，但 Web 端可以更智能，先看 inventories 中有没有
          const exists = !!inventories[dishId];
          
          if (exists) {
            await apiPut("/inventory", {
              dish_id: dishId,
              date: selectedDate,
              total_quantity: quantity
            });
          } else {
            // 如果当天没有库存记录，则创建
            await apiPost("/inventory", {
              dish_id: dishId,
              date: selectedDate,
              total_quantity: quantity
            });
          }
          successCount++;
        } catch (err) {
          console.error(`Failed to update dish ${dishId}`, err);
          failCount++;
        }
      });

      await Promise.all(promises);

      if (failCount === 0) {
        toast.success(`成功保存 ${successCount} 项修改`);
      } else {
        toast.warning(`保存完成：${successCount} 成功，${failCount} 失败`);
      }

      // 重新加载数据
      await loadData(selectedDate);
    } catch (err) {
      console.error("Failed to save changes", err);
      toast.error("保存修改失败");
    } finally {
      setSaving(false);
    }
  };

  const handleReset = () => {
    setPendingChanges({});
  };

  // --- 过滤逻辑 ---
  const filteredDishes = useMemo(() => {
    return dishes.filter(dish => {
      const matchesCategory = activeCategoryId === "all" || dish.category_id === activeCategoryId;
      const matchesSearch = dish.name.toLowerCase().includes(searchKeyword.toLowerCase());
      return matchesCategory && matchesSearch;
    });
  }, [dishes, activeCategoryId, searchKeyword]);

  // --- 渲染渲染 ---
  const hasChanges = Object.keys(pendingChanges).length > 0;

  return (
    <PageShell>
      <PageHeader
        title="库存管理"
        description="管理每日菜品可售数量。-1 表示无限库存。"
        actions={
          <div className="flex items-center gap-3">
            <div className="flex items-center bg-white border rounded-md px-3 h-9 shadow-sm">
              <span className="text-sm text-muted-foreground mr-2 font-medium">日期:</span>
              <input 
                type="date" 
                value={selectedDate} 
                onChange={(e) => setSelectedDate(e.target.value)}
                className="text-sm border-none bg-transparent focus:ring-0 outline-none h-full"
              />
            </div>
            {hasChanges && (
              <div className="flex items-center gap-2">
                <Button variant="outline" size="sm" onClick={handleReset}>
                  <RefreshCw className="h-4 w-4 mr-2" />
                  重置
                </Button>
                <Button size="sm" onClick={handleSaveChanges} disabled={saving}>
                  {saving ? (
                    <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                  ) : (
                    <Save className="h-4 w-4 mr-2" />
                  )}
                  保存 ({Object.keys(pendingChanges).length})
                </Button>
              </div>
            )}
          </div>
        }
      />

      <PageContent>
        <div className="flex flex-col lg:flex-row gap-6 h-[calc(100vh-12rem)]">
          {/* 左侧：分类 + 统计 */}
          <div className="lg:w-72 flex flex-col gap-4 shrink-0">
            {/* 统计卡片 */}
            <div className="bg-white rounded-xl border shadow-sm overflow-hidden">
              <div className="p-4 border-b">
                <h3 className="text-sm font-semibold text-slate-900 border-l-4 border-primary pl-3">
                  今日概览
                </h3>
              </div>
              <CardContent className="p-4 grid grid-cols-2 gap-x-4 gap-y-6">
                <div className="space-y-1">
                  <p className="text-[10px] text-muted-foreground font-medium">总菜品</p>
                  <div className="flex items-center gap-2">
                     <Package className="h-4 w-4 text-blue-500" />
                     <span className="text-lg font-bold tabular-nums">
                       {loading ? "..." : (stats?.total_dishes ?? 0)}
                     </span>
                  </div>
                </div>
                <div className="space-y-1">
                  <p className="text-[10px] text-muted-foreground font-medium">不限库存</p>
                  <div className="flex items-center gap-2">
                    <Infinity className="h-4 w-4 text-emerald-500" />
                    <span className="text-lg font-bold tabular-nums">
                      {loading ? "..." : (stats?.unlimited_dishes ?? 0)}
                    </span>
                  </div>
                </div>
                <div className="space-y-1 pt-3 border-t">
                  <p className="text-[10px] text-muted-foreground font-medium">可用菜品</p>
                  <div className="flex items-center gap-2">
                    <PackageCheck className="h-4 w-4 text-primary" />
                    <span className="text-lg font-bold tabular-nums">
                      {loading ? "..." : (stats?.available_dishes ?? 0)}
                    </span>
                  </div>
                </div>
                <div className="space-y-1 pt-3 border-t">
                  <p className="text-[10px] text-muted-foreground font-medium">已售罄</p>
                  <div className="flex items-center gap-2">
                    <PackageX className="h-4 w-4 text-rose-500" />
                    <span className="text-lg font-bold tabular-nums">
                      {loading ? "..." : (stats?.sold_out_dishes ?? 0)}
                    </span>
                  </div>
                </div>
              </CardContent>
            </div>

            {/* 分类列表 */}
            <div className="flex-1 flex flex-col bg-white rounded-xl border shadow-sm">
              <div className="p-4 border-b">
                <h3 className="text-sm font-semibold text-slate-900 border-l-4 border-primary pl-3">
                  选择分类
                </h3>
              </div>
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
                    <span className="text-sm font-medium">全部</span>
                    <Badge variant="outline" className="h-5 text-xs text-muted-foreground">{dishes.length}</Badge>
                  </div>
                  {categories.map(cat => (
                    <div 
                      key={cat.id} 
                      onClick={() => setActiveCategoryId(cat.id)}
                      className={cn(
                        "px-3 py-2 rounded-lg cursor-pointer transition-all flex items-center justify-between",
                        activeCategoryId === cat.id 
                          ? "bg-primary/5 border border-primary text-primary" 
                          : "hover:bg-slate-50 border border-transparent"
                      )}
                    >
                      <span className="text-sm font-medium truncate pr-2">{cat.name}</span>
                      <Badge variant="outline" className="h-5 text-xs text-muted-foreground">
                        {dishes.filter(d => d.category_id === cat.id).length}
                      </Badge>
                    </div>
                  ))}
                </div>
              </ScrollArea>
            </div>
          </div>

          {/* 右侧：主列表 */}
          <div className="flex-1 flex flex-col bg-white rounded-xl border shadow-sm overflow-hidden">
            {/* 搜索栏 */}
            <div className="p-4 border-b">
              <div className="relative max-w-md">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input 
                  placeholder="搜索菜品名称..." 
                  className="pl-9 bg-slate-50/50 border-slate-200 h-9" 
                  value={searchKeyword} 
                  onChange={(e) => setSearchKeyword(e.target.value)} 
                />
              </div>
            </div>

            {/* 表头 */}
            <div className="px-4 py-3 border-b bg-slate-50/50 flex items-center font-bold text-muted-foreground uppercase tracking-widest text-[10px]">
              {/* 镜像行结构的左侧 */}
              <div className="flex-1 flex items-center gap-4">
                <div className="w-12 shrink-0"></div> {/* 与头像对齐 */}
                <div>菜品信息</div>
              </div>

              {/* 镜像行结构的右侧 */}
              <div className="flex items-center gap-6 shrink-0 h-full">
                <div className="w-24 text-center">总库存设置</div>
                
                <div className="hidden md:flex gap-6 items-center">
                  <div className="w-16 text-center">已销售</div>
                  <div className="w-16 text-center">预留</div>
                  <div className="w-20 text-center uppercase">可用库存</div>
                </div>
                
                <div className="w-6"></div> {/* 状态图标位 */}
              </div>
            </div>

            {/* 列表内容 */}
            <ScrollArea className="flex-1">
              <div className="divide-y">
                {loading ? (
                  <div className="flex flex-col items-center justify-center h-64 text-muted-foreground gap-3">
                    <Loader2 className="h-8 w-8 animate-spin" />
                    <p className="text-sm">正在获取库存数据...</p>
                  </div>
                ) : filteredDishes.length === 0 ? (
                  <div className="flex flex-col items-center justify-center h-64 text-muted-foreground gap-2">
                    <Package className="h-10 w-10 opacity-20" />
                    <p className="text-sm">未找到符合条件的菜品</p>
                  </div>
                ) : (
                  filteredDishes.map(dish => {
                    const inv = inventories[dish.id];
                    const pendingValue = pendingChanges[dish.id];
                    const currentTotal = pendingValue !== undefined ? pendingValue : (inv ? inv.total_quantity : -1);
                    const isUnlimited = currentTotal === -1;
                    
                    const sold = inv?.sold_quantity || 0;
                    const reserved = inv?.reserved_quantity || 0;
                    
                    let available: string | number = "∞";
                    if (!isUnlimited) {
                      available = currentTotal - sold - reserved;
                      if (available < 0) available = 0;
                    }

                    return (
                      <div key={dish.id} className="p-4 hover:bg-slate-50/50 transition-colors group flex items-center">
                        {/* 菜品信息 */}
                        <div className="flex-1 flex items-center gap-4 min-w-0">
                          <div className="w-12 h-12 bg-slate-100 rounded-lg overflow-hidden shrink-0">
                             {dish.image_url ? (
                               <Image src={formatImageUrl(getMediaUrl(dish.image_url))} alt={dish.name} width={48} height={48} className="w-full h-full object-cover" />
                             ) : (
                               <div className="w-full h-full flex items-center justify-center text-slate-300">
                                 <Package className="h-6 w-6" />
                               </div>
                             )}
                          </div>
                          <div className="min-w-0 pr-4">
                            <h4 className="text-sm font-semibold text-slate-900 truncate">{dish.name}</h4>
                            <div className="flex items-center gap-2 mt-1">
                              <span className="text-sm font-bold text-primary">¥{formatAmount(dish.price)}</span>
                              <Badge variant="outline" className="text-[10px] h-4 px-1.5 opacity-60">
                                {categories.find(c => c.id === dish.category_id)?.name || "默认分类"}
                              </Badge>
                              {!dish.is_online && (
                                <Badge variant="secondary" className="text-[10px] h-4 px-1.5">已下架</Badge>
                              )}
                            </div>
                          </div>
                        </div>

                        {/* 库存编辑器 */}
                        <div className="flex items-center gap-6 shrink-0">
                          {/* 总数设置 */}
                          <div className="flex flex-col items-center gap-2 w-24">
                            <div className="flex items-center gap-2 h-9">
                              {isUnlimited ? (
                                <div className="flex items-center justify-center w-20 h-9 bg-emerald-50 text-emerald-600 rounded-md border border-emerald-100 font-medium text-sm">
                                  <Infinity className="h-4 w-4 mr-1" />
                                  无限
                                </div>
                              ) : (
                                <Input 
                                  type="number"
                                  value={currentTotal}
                                  onChange={(e) => handleInventoryChange(dish.id, e.target.value)}
                                  className={cn(
                                    "w-20 h-9 text-center font-bold px-1",
                                    pendingValue !== undefined && "border-primary ring-1 ring-primary/20 bg-primary/5"
                                  )}
                                  min="0"
                                />
                              )}
                            </div>
                            <div className="flex items-center gap-2">
                               <Switch 
                                 id={`unlimited-${dish.id}`} 
                                 checked={isUnlimited} 
                                 onCheckedChange={(checked) => handleToggleUnlimited(dish.id, checked)}
                                 className="scale-75"
                               />
                               <Label htmlFor={`unlimited-${dish.id}`} className="text-[10px] text-muted-foreground whitespace-nowrap cursor-pointer">
                                 不限库存
                               </Label>
                            </div>
                          </div>

                          {/* 业务数据 */}
                          <div className="hidden md:flex gap-6 items-center">
                            <div className="w-16 text-center text-sm font-medium text-slate-600">
                              {sold}
                            </div>
                            <div className="w-16 text-center text-sm font-medium text-slate-600">
                              {reserved}
                            </div>
                            <div className="w-20 text-center flex justify-center">
                              <Badge 
                                variant={available === 0 ? "destructive" : "outline"}
                                className={cn(
                                  "min-w-10 justify-center font-bold",
                                  available === "∞" ? "bg-emerald-50 text-emerald-600 border-emerald-200" :
                                  available === 0 ? "bg-rose-50 text-rose-600 border-rose-200" :
                                  "bg-slate-50 text-slate-600"
                                )}
                              >
                                {available}
                              </Badge>
                            </div>
                          </div>
                          
                          {/* 状态指示器 */}
                          <div className="w-6 flex justify-center">
                            {pendingValue !== undefined ? (
                              <RefreshCw className="h-4 w-4 text-primary animate-spin" />
                            ) : inv ? (
                              <CheckCircle2 className="h-4 w-4 text-emerald-500 opacity-40" />
                            ) : (
                              <AlertCircle className="h-4 w-4 text-amber-400 opacity-40" />
                            )}
                          </div>
                        </div>
                      </div>
                    );
                  })
                )}
              </div>
            </ScrollArea>
            
            {/* 批量操作提示 */}
            {hasChanges && (
              <div className="p-3 bg-primary/5 border-t border-primary/20 flex items-center justify-between px-6 animate-in slide-in-from-bottom-2">
                <div className="flex items-center gap-2 text-primary text-sm font-medium">
                  <RefreshCw className="h-4 w-4" />
                  <span>您有 {Object.keys(pendingChanges).length} 项修改尚未保存</span>
                </div>
                <div className="flex gap-3">
                   <Button variant="ghost" size="sm" className="text-primary hover:bg-primary/10" onClick={handleReset}>取消所有修改</Button>
                   <Button size="sm" onClick={handleSaveChanges} disabled={saving}>
                    {saving && <Loader2 className="h-3 w-3 mr-2 animate-spin" />}
                    立即提交所有修改
                   </Button>
                </div>
              </div>
            )}
          </div>
        </div>
      </PageContent>
    </PageShell>
  );
}
