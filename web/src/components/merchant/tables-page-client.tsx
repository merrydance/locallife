"use client";

import { useState, useMemo, useEffect } from "react";
import { 
  Search, 
  Plus, 
  Filter, 
  Trash2, 
  Edit, 
  QrCode, 
  Users, 
  Armchair, 
  Home, 
  CheckCircle2, 
  XCircle, 
  AlertCircle,
  Download,
  RefreshCw,
  ImagePlus,
  X
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { 
  Card, 
  CardContent, 
  CardHeader, 
  CardFooter
} from "@/components/ui/card";
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
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";

import { PageShell, PageHeader, PageContent } from "@/components/merchant/layout/page-shell";
import { apiGet, apiPost, apiPatch, apiPut, apiDelete, apiUpload, getMediaUrl, getAuthToken } from "@/lib/api";
import { cn } from "@/lib/utils";
import type { TableResponse, TableType, TableStatus, TableTag, CreateTableRequest, TableImageResponse } from "@/types/table";

// Simple toast mock since sonner is not installed
const toast = {
  success: (msg: string) => alert(msg),
  error: (msg: string) => alert("错误: " + msg),
};

const STATUS_MAP: Record<TableStatus, { label: string, variant: "default" | "secondary" | "destructive" | "outline", icon: any }> = {
  available: { label: "空闲", variant: "secondary", icon: CheckCircle2 },
  occupied: { label: "占用", variant: "default", icon: Armchair },
  reserved: { label: "已预定", variant: "outline", icon: AlertCircle },
  disabled: { label: "停用", variant: "destructive", icon: XCircle },
};

const TYPE_MAP: Record<TableType, { label: string, icon: any }> = {
  table: { label: "大厅桌台", icon: Armchair },
  room: { label: "包间", icon: Home },
};

interface TablesPageClientProps {
  initialData: TableResponse[];
}

export function TablesPageClient({ initialData }: TablesPageClientProps) {
  const [tables, setTables] = useState<TableResponse[]>(initialData);
  const [loading, setLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [activeType, setActiveType] = useState<string>("all");
  
  // Sheet state
  const [isSheetOpen, setIsSheetOpen] = useState(false);
  const [editingTable, setEditingTable] = useState<TableResponse | null>(null);
  const [formData, setFormData] = useState<Partial<CreateTableRequest>>({
    table_no: "",
    table_type: "table",
    capacity: 4,
    description: "",
  });

  // Tag manager state
  const [isTagDialogOpen, setIsTagDialogOpen] = useState(false);
  const [availableTags, setAvailableTags] = useState<TableTag[]>([]);
  const [selectedTagIds, setSelectedTagIds] = useState<number[]>([]);

  // QR Code state
  const [qrCodeDialog, setQrCodeDialog] = useState<{ open: boolean; table: TableResponse | null }>({
    open: false,
    table: null,
  });

  // Image state
  const [tableImages, setTableImages] = useState<TableImageResponse[]>([]);

  const loadTables = async () => {
    setLoading(true);
    try {
      const response = await apiGet<{ tables: TableResponse[] }>("/tables");
      setTables(response.tables || []);
    } catch (error) {
      toast.error("加载桌台失败");
    } finally {
      setLoading(false);
    }
  };

  const loadTags = async () => {
    try {
      const response = await apiGet<{ tags: any[] }>("/tags", { type: "table" });
      setAvailableTags(response.tags.map(t => ({ id: t.id, name: t.name })));
    } catch (error) {
      console.error("Failed to load tags");
    }
  };

  const loadTableImages = async (tableId: number) => {
    try {
      const token = getAuthToken();
      console.log(`[loadTableImages] Fetching images for tableId: ${tableId}, Token present: ${!!token}, Token length: ${token?.length}`);
      const response = await apiGet<{ images: TableImageResponse[] }>(`/tables/${tableId}/images`);
      console.log("[loadTableImages] Response:", response);
      setTableImages(response.images || []);
    } catch (error) {
      console.error("Failed to load table images. TableID:", tableId, "Error:", error);
      setTableImages([]);
    }
  };

  const handleUploadImage = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!e.target.files || !e.target.files[0] || !editingTable) return;
    
    const file = e.target.files[0];
    try {
      setLoading(true);
      // 1. Upload image to get URL
      const uploadRes = await apiUpload<{ image_url: string }>("/tables/images/upload", file);
      
      // 2. Add image to table
      await apiPost(`/tables/${editingTable.id}/images`, { image_url: uploadRes.image_url });
      
      toast.success("图片上传成功");
      loadTableImages(editingTable.id);
    } catch (error: any) {
      toast.error(error.message || "上传失败");
    } finally {
      setLoading(false);
      // Reset input
      e.target.value = "";
    }
  };

  const handleDeleteImage = async (imageId: number) => {
    if (!editingTable || !confirm("确定要删除这张图片吗？")) return;
    try {
      await apiDelete(`/tables/${editingTable.id}/images/${imageId}`);
      toast.success("图片已删除");
      loadTableImages(editingTable.id);
    } catch (error: any) {
      toast.error("删除失败");
    }
  };

  const handleSetPrimaryImage = async (imageId: number) => {
    if (!editingTable) return;
    try {
      await apiPut(`/tables/${editingTable.id}/images/${imageId}/primary`, {});
      toast.success("已设为主图");
      loadTableImages(editingTable.id);
    } catch (error: any) {
      toast.error("设置失败");
    }
  };

  useEffect(() => {
    loadTables();
    loadTags();
  }, []);

  const filteredTables = useMemo(() => {
    return tables.filter(table => {
      const matchesSearch = table.table_no.toLowerCase().includes(searchQuery.toLowerCase());
      const matchesType = activeType === "all" || table.table_type === activeType;
      return matchesSearch && matchesType;
    });
  }, [tables, searchQuery, activeType]);

  const handleAddTable = () => {
    setEditingTable(null);
    setFormData({
      table_no: "",
      table_type: "table",
      capacity: 4,
      description: "",
      tag_ids: [],
    });
    setSelectedTagIds([]);
    setTableImages([]);
    setIsSheetOpen(true);
  };

  const handleEditTable = (table: TableResponse) => {
    setEditingTable(table);
    setFormData({
      table_no: table.table_no,
      table_type: table.table_type,
      capacity: table.capacity,
      description: table.description || "",
      minimum_spend: table.minimum_spend,
      tag_ids: table.tags?.map(t => t.id) || [],
    });
    setSelectedTagIds(table.tags?.map(t => t.id) || []);
    setTableImages([]); // Clear first
    loadTableImages(table.id); // Load images
    setIsSheetOpen(true);
  };

  const handleSaveTable = async () => {
    if (!formData.table_no) {
      toast.error("请输入桌号");
      return;
    }

    try {
      setLoading(true);
      const data = { ...formData, tag_ids: selectedTagIds };
      if (editingTable) {
        await apiPatch(`/tables/${editingTable.id}`, data);
        toast.success("桌台已更新");
      } else {
        await apiPost("/tables", data);
        toast.success("桌台已添加");
      }
      setIsSheetOpen(false);
      loadTables();
    } catch (error: any) {
      toast.error(error.message || "保存失败");
    } finally {
      setLoading(false);
    }
  };

  const handleDeleteTable = async (id: number) => {
    if (!confirm("确定要删除此桌台吗？")) return;
    try {
      await apiDelete(`/tables/${id}`);
      toast.success("桌台已删除");
      loadTables();
    } catch (error: any) {
      toast.error(error.message || "删除失败");
    }
  };

  const handleUpdateStatus = async (id: number, status: TableStatus) => {
    try {
      await apiPatch(`/tables/${id}/status`, { status });
      toast.success("状态已更新");
      loadTables();
    } catch (error: any) {
      toast.error(error.message || "更新状态失败");
    }
  };

  const toggleTag = (tagId: number) => {
    setSelectedTagIds(prev => 
      prev.includes(tagId) ? prev.filter(id => id !== tagId) : [...prev, tagId]
    );
  };

  return (
    <PageShell>
      <PageHeader 
        title="桌台管理" 
        description="管理您的店铺桌台和包间，查看实时占用情况"
        actions={
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={loadTables} disabled={loading}>
              <RefreshCw className={cn("h-4 w-4 mr-2", loading && "animate-spin")} />
              刷新
            </Button>
            <Button onClick={handleAddTable}>
              <Plus className="h-4 w-4 mr-2" />
              添加桌台
            </Button>
          </div>
        }
      />

      <PageContent>
        <div className="space-y-4">
          {/* Filters and Search */}
          <div className="flex flex-col md:flex-row gap-4 items-center justify-between bg-card p-4 rounded-xl border border-muted/50 shadow-sm">
            <div className="flex items-center gap-2 w-full md:w-auto">
              <div className="relative w-full md:w-80">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input 
                  placeholder="搜索桌号..." 
                  className="pl-9 bg-muted/50 border-none focus-visible:ring-1"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                />
              </div>
              <Tabs value={activeType} onValueChange={setActiveType} className="hidden lg:block">
                <TabsList className="bg-muted/50">
                  <TabsTrigger value="all">全部</TabsTrigger>
                  <TabsTrigger value="table">大厅桌台</TabsTrigger>
                  <TabsTrigger value="room">包间</TabsTrigger>
                </TabsList>
              </Tabs>
            </div>

            <Select value={activeType} onValueChange={setActiveType}>
              <SelectTrigger className="w-full md:w-40 lg:hidden">
                <Filter className="h-4 w-4 mr-2" />
                <SelectValue placeholder="类型筛选" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部类型</SelectItem>
                <SelectItem value="table">大厅桌台</SelectItem>
                <SelectItem value="room">包间</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {/* Grid Layout */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5 gap-4">
            {filteredTables.map(table => {
              const statusInfo = STATUS_MAP[table.status] || STATUS_MAP.available;
              const typeInfo = TYPE_MAP[table.table_type] || TYPE_MAP.table;
              const StatusIcon = statusInfo.icon;
              const TypeIcon = typeInfo.icon;

              return (
                <Card 
                  key={table.id} 
                  className={cn(
                    "group relative overflow-hidden transition-all hover:shadow-lg cursor-pointer border-muted/60",
                    table.status === 'disabled' && "opacity-80 grayscale-[0.2] bg-muted/20"
                  )}
                  onClick={() => handleEditTable(table)}
                >
                  {/* Status Indicator Bar */}
                  <div className={cn(
                    "absolute top-0 left-0 right-0 h-1 rounded-t-md",
                    table.status === 'available' ? "bg-emerald-500" :
                    table.status === 'occupied' ? "bg-primary" :
                    table.status === 'reserved' ? "bg-amber-500" : "bg-muted-foreground"
                  )} />

                  <CardHeader className="p-4 pb-2">
                    <div className="flex items-start justify-between">
                      <div className="space-y-1">
                        <div className="flex items-center gap-2">
                          <h3 className="text-3xl font-bold tracking-tight">{table.table_no}</h3>
                          <Badge className={cn("text-[10px] px-1.5 h-4 font-normal text-white", table.status === 'available' ? "bg-emerald-500" : table.status === 'occupied' ? "bg-primary" : table.status === 'reserved' ? "bg-amber-500" : "bg-muted-foreground")}>
                            {statusInfo.label}
                          </Badge>
                        </div>
                        <div className="flex items-center text-sm text-muted-foreground gap-2">
                          <span className="flex items-center gap-1">
                            <TypeIcon className="h-3 w-3" />
                            {typeInfo.label}
                          </span>
                          <span>•</span>
                          <span className="flex items-center gap-1">
                            <Users className="h-3 w-3" />
                            {table.capacity}人
                          </span>
                        </div>
                      </div>
                      
                      <div className="flex gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity translate-x-1">
                        <Button variant="ghost" size="icon" className="h-8 w-8 hover:bg-primary/10 hover:text-primary rounded-full" onClick={(e: React.MouseEvent) => { e.stopPropagation(); setQrCodeDialog({ open: true, table }); }}>
                          <QrCode className="h-4 w-4" />
                        </Button>
                        <Button variant="ghost" size="icon" className="h-8 w-8 hover:bg-primary/10 hover:text-primary rounded-full" onClick={(e: React.MouseEvent) => { e.stopPropagation(); handleEditTable(table); }}>
                          <Edit className="h-4 w-4" />
                        </Button>
                        <Button variant="ghost" size="icon" className="h-8 w-8 hover:bg-destructive/10 hover:text-destructive rounded-full" onClick={(e: React.MouseEvent) => { e.stopPropagation(); handleDeleteTable(table.id); }}>
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                    </div>
                  </CardHeader>

                  <CardContent className="p-4 pt-2">
                    {table.tags && table.tags.length > 0 ? (
                      <div className="flex flex-wrap gap-1.5 mt-2">
                        {table.tags.slice(0, 3).map(tag => (
                          <Badge key={tag.id} variant="outline" className="text-xs h-4 border-muted/60 font-normal">
                            {tag.name}
                          </Badge>
                        ))}
                        {table.tags.length > 3 && (
                          <span className="text-[10px] text-muted-foreground ml-1">+{table.tags.length - 3}</span>
                        )}
                      </div>
                    ) : (
                      <div className="text-[10px] text-muted-foreground/60 italic mt-2">暂无标签</div>
                    )}
                    
                    {table.current_reservation && (
                      <div className="mt-3 p-2 bg-primary/5 rounded-lg border border-primary/10">
                         <div className="flex items-center justify-between text-sm font-medium text-primary">
                          <span>即将到访</span>
                          <span>{table.current_reservation.reservation_time}</span>
                        </div>
                         <div className="text-sm text-muted-foreground truncate mt-0.5">
                          {table.current_reservation.contact_name} ({table.current_reservation.guest_count}人)
                        </div>
                      </div>
                    )}
                  </CardContent>

                   <CardFooter className="p-4 pt-0 flex justify-between items-center text-sm text-muted-foreground">
                    <span className="flex items-center gap-1 opacity-60">
                      ID: {table.id}
                    </span>
                    {table.minimum_spend ? (
                      <span className="font-medium text-primary/80">
                        低消 ¥{table.minimum_spend / 100}
                      </span>
                    ) : null}
                  </CardFooter>
                </Card>
              );
            })}

            {/* Add New Card Placeholder */}
            <Card 
              className="border-dashed border-2 hover:border-primary/50 hover:bg-primary/5 transition-all cursor-pointer flex flex-col items-center justify-center p-6 h-full min-h-[160px]"
              onClick={handleAddTable}
            >
              <div className="h-10 w-10 rounded-full bg-muted flex items-center justify-center mb-3">
                <Plus className="h-6 w-6 text-muted-foreground" />
              </div>
              <p className="font-medium text-sm text-muted-foreground">添加桌台</p>
            </Card>
          </div>
          
          {filteredTables.length === 0 && searchQuery && (
            <div className="flex flex-col items-center justify-center py-20 text-center">
              <div className="h-16 w-16 bg-muted rounded-full flex items-center justify-center mb-4">
                <Search className="h-8 w-8 text-muted-foreground/40" />
              </div>
              <p className="text-lg font-medium">未找到相关桌台</p>
              <p className="text-muted-foreground">尝试更换搜索关键词或筛选条件</p>
              <Button variant="link" onClick={() => { setSearchQuery(""); setActiveType("all"); }} className="mt-2">
                清除所有筛选
              </Button>
            </div>
          )}
        </div>
      </PageContent>

      {/* Edit/Add Sheet */}
      <Sheet open={isSheetOpen} onOpenChange={setIsSheetOpen}>
        <SheetContent className="sm:max-w-md md:max-w-lg overflow-y-auto">
          <SheetHeader className="pb-4">
            <SheetTitle>{editingTable ? "编辑桌台" : "添加桌台"}</SheetTitle>
            <SheetDescription>
              填写桌台的基本信息、位置描述和配置详情
            </SheetDescription>
          </SheetHeader>

          <div className="space-y-6 py-4">
            <div className="space-y-4">
              <div className="grid gap-2">
                <Label htmlFor="table_no">桌号/房号 <span className="text-destructive">*</span></Label>
                <Input 
                  id="table_no" 
                  placeholder="如：A01，大厅-05，VIP包间-1" 
                  value={formData.table_no}
                  onChange={(e) => setFormData({...formData, table_no: e.target.value})}
                />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div className="grid gap-2">
                  <Label htmlFor="table_type">类型</Label>
                  <Select 
                    value={formData.table_type} 
                    onValueChange={(val: any) => setFormData({...formData, table_type: val})}
                  >
                    <SelectTrigger id="table_type">
                      <SelectValue placeholder="选择类型" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="table">大厅桌台</SelectItem>
                      <SelectItem value="room">包间</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="capacity">容纳人数 <span className="text-destructive">*</span></Label>
                  <Input 
                    id="capacity" 
                    type="number"
                    min={1}
                    max={100}
                    value={formData.capacity}
                    onChange={(e) => setFormData({...formData, capacity: parseInt(e.target.value) || 1})}
                  />
                </div>
              </div>

              <div className="grid gap-2">
                <Label htmlFor="minimum_spend">最低消费 (元)</Label>
                <div className="relative">
                  <span className="absolute left-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">¥</span>
                  <Input 
                    id="minimum_spend" 
                    type="number"
                    step="0.01"
                    placeholder="不填则无最低消费"
                    className="pl-7"
                    value={formData.minimum_spend !== undefined ? (formData.minimum_spend / 100).toString() : ""}
                    onChange={(e) => {
                      const val = e.target.value;
                      setFormData({...formData, minimum_spend: val === "" ? undefined : Math.round(parseFloat(val) * 100)});
                    }}
                  />
                </div>
              </div>

              <div className="grid gap-2">
                <Label htmlFor="description">描述/备注</Label>
                <Textarea 
                  id="description" 
                  placeholder="填写位置信息或特色描述，如：靠近窗户，江景位" 
                  className="resize-none h-24"
                  value={formData.description}
                  onChange={(e) => setFormData({...formData, description: e.target.value})}
                />
              </div>
            </div>

            <div className="space-y-3">
              <Label className="text-sm font-semibold">桌台图片</Label>
              {editingTable ? (
                <div className="grid grid-cols-3 gap-3">
                  {tableImages.map((img) => (
                    <div key={img.id} className="relative aspect-square group rounded-lg overflow-hidden border bg-muted/20">
                      <img 
                        src={getMediaUrl(img.image_url)} 
                        alt="Table" 
                        className="w-full h-full object-cover"
                      />
                      {img.is_primary && (
                        <div className="absolute top-0 right-0 bg-primary text-primary-foreground text-[10px] px-1.5 py-0.5 rounded-bl-lg">
                          主图
                        </div>
                      )}
                      <div className="absolute inset-0 bg-black/50 opacity-0 group-hover:opacity-100 transition-opacity flex flex-col items-center justify-center gap-2">
                        {!img.is_primary && (
                          <Button size="sm" variant="secondary" className="h-6 text-[10px] px-2" onClick={() => handleSetPrimaryImage(img.id)}>
                            设为主图
                          </Button>
                        )}
                        <Button size="icon" variant="destructive" className="h-7 w-7" onClick={() => handleDeleteImage(img.id)}>
                          <Trash2 className="h-3 w-3" />
                        </Button>
                      </div>
                    </div>
                  ))}
                  <label className="flex flex-col items-center justify-center border-2 border-dashed border-muted-foreground/25 rounded-lg aspect-square cursor-pointer hover:bg-muted/50 transition-colors">
                    <ImagePlus className="h-6 w-6 text-muted-foreground mb-1" />
                    <span className="text-[10px] text-muted-foreground">上传图片</span>
                    <input type="file" className="hidden" accept="image/*" onChange={handleUploadImage} disabled={loading} />
                  </label>
                </div>
              ) : (
                <div className="p-4 border border-dashed rounded-lg text-center text-sm text-muted-foreground bg-muted/20">
                  请先创建桌台，然后编辑以添加图片
                </div>
              )}
            </div>

            <Separator />

            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <Label className="text-sm font-semibold">桌台标签</Label>
                <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={() => setIsTagDialogOpen(true)}>
                  <Plus className="h-3 w-3 mr-1" />
                  管理标签
                </Button>
              </div>
              <div className="flex flex-wrap gap-2">
                {availableTags.length > 0 ? (
                  availableTags.map(tag => (
                    <Badge 
                      key={tag.id}
                      variant={selectedTagIds.includes(tag.id) ? "default" : "outline"}
                      className={cn(
                        "cursor-pointer px-3 py-1 transition-colors hover:bg-muted",
                        selectedTagIds.includes(tag.id) && "hover:bg-primary/90"
                      )}
                      onClick={() => toggleTag(tag.id)}
                    >
                      {tag.name}
                      {selectedTagIds.includes(tag.id) && <CheckCircle2 className="ml-1 h-3 w-3 fill-white text-primary" />}
                    </Badge>
                  ))
                ) : (
                  <p className="text-xs text-muted-foreground italic">暂无可用标签，点击管理标签添加</p>
                )}
              </div>
            </div>
            
            {editingTable && (
              <>
                <Separator />
                <div className="space-y-3">
                  <Label className="text-sm font-semibold text-destructive">危险操作</Label>
                  <Button variant="outline" className="w-full text-destructive hover:bg-destructive/10 border-destructive/20" onClick={() => handleDeleteTable(editingTable.id)}>
                    <Trash2 className="h-4 w-4 mr-2" />
                    删除此桌台
                  </Button>
                </div>
              </>
            )}
          </div>

          <SheetFooter className="absolute bottom-0 left-0 right-0 p-6 bg-background border-t">
            <div className="flex gap-2 w-full">
              <Button variant="outline" className="flex-1" onClick={() => setIsSheetOpen(false)}>取消</Button>
              <Button className="flex-1" onClick={handleSaveTable} disabled={loading}>
                {loading && <RefreshCw className="mr-2 h-4 w-4 animate-spin" />}
                保存桌台
              </Button>
            </div>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      {/* QR Code Dialog */}
      <Dialog open={qrCodeDialog.open} onOpenChange={(open) => setQrCodeDialog({ ...qrCodeDialog, open })}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>桌台二维码</DialogTitle>
            <DialogDescription>
              桌号：{qrCodeDialog.table?.table_no}
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col items-center justify-center py-6 gap-4">
            <div className="relative p-4 bg-white rounded-2xl shadow-inner border aspect-square w-64 flex items-center justify-center">
              {qrCodeDialog.table?.qr_code_url ? (
                <img 
                  src={getMediaUrl(qrCodeDialog.table.qr_code_url)} 
                  alt="QR Code" 
                  className="w-full h-full object-contain"
                />
              ) : (
                <div className="flex flex-col items-center justify-center text-muted-foreground gap-2">
                  <QrCode className="h-12 w-12 stroke-[1.5] opacity-20" />
                  <span className="text-xs">暂无二维码</span>
                </div>
              )}
            </div>
            <p className="text-xs text-center text-muted-foreground max-w-[240px]">
              顾客扫描此二维码即可进入点餐页面进行自助下单
            </p>
          </div>
          <DialogFooter className="flex-col sm:flex-col gap-2">
            <Button className="w-full">
              <Download className="h-4 w-4 mr-2" />
              下载打印图片
            </Button>
            <Button variant="ghost" className="w-full" onClick={() => setQrCodeDialog({ open: false, table: null })}>
              关闭
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      
      {/* Tag Management Dialog */}
      <Dialog open={isTagDialogOpen} onOpenChange={setIsTagDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>管理桌台标签</DialogTitle>
            <DialogDescription>
              添加或移除桌台属性标签，如“空调位”、“景观位”等
            </DialogDescription>
          </DialogHeader>
          
          <div className="space-y-4 py-4">
            <div className="flex gap-2">
              <Input placeholder="输入新标签名称..." id="new-tag-name" />
              <Button size="sm" onClick={async () => {
                const input = document.getElementById("new-tag-name") as HTMLInputElement;
                if (!input.value) return;
                try {
                  await apiPost("/tags", { name: input.value, type: "table" });
                  input.value = "";
                  loadTags();
                  toast.success("标签已添加");
                } catch (error) {
                  toast.error("添加失败");
                }
              }}>添加</Button>
            </div>
            
            <ScrollArea className="h-60 rounded-md border p-4">
              <div className="space-y-2">
                {availableTags.map(tag => (
                  <div key={tag.id} className="flex items-center justify-between p-2 rounded-lg hover:bg-muted/50 group">
                    <span className="text-sm font-medium">{tag.name}</span>
                    <Button 
                      variant="ghost" 
                      size="icon" 
                      className="h-8 w-8 text-muted-foreground hover:text-destructive opacity-0 group-hover:opacity-100 transition-opacity"
                      onClick={async () => {
                        if (!confirm(`确定要删除标签 "${tag.name}" 吗？`)) return;
                        try {
                          await apiDelete(`/tags/${tag.id}`);
                          loadTags();
                        } catch (error) {
                          toast.error("删除失败");
                        }
                      }}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                ))}
                {availableTags.length === 0 && (
                  <p className="text-center text-sm text-muted-foreground py-10 italic">暂无标签</p>
                )}
              </div>
            </ScrollArea>
          </div>
          <DialogFooter>
            <Button onClick={() => setIsTagDialogOpen(false)}>完成</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </PageShell>
  );
}
