"use client";

import { useState, useMemo, useEffect } from "react";
import Image from "next/image";
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
import type { LucideIcon } from "lucide-react";
import { toast } from "sonner";
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
import { ConfirmDialog } from "@/components/ui/confirm-dialog";

import { PageShell, PageHeader, PageContent } from "@/components/merchant/layout/page-shell";
import { apiGet, apiPost, apiPatch, apiPut, apiDelete, getMediaUrl, getAuthToken } from "@/lib/api";
import { uploadMedia } from "@/lib/media";
import { cn } from "@/lib/utils";
import type { TableResponse, TableType, TableStatus, TableTag, CreateTableRequest, TableImageResponse } from "@/types/table";

const STATUS_MAP: Record<TableStatus, { label: string, variant: "default" | "secondary" | "destructive" | "outline", icon: LucideIcon }> = {
  available: { label: "空闲", variant: "secondary", icon: CheckCircle2 },
  occupied: { label: "占用", variant: "default", icon: Armchair },
  reserved: { label: "已预定", variant: "outline", icon: AlertCircle },
  disabled: { label: "停用", variant: "destructive", icon: XCircle },
};

const TYPE_MAP: Record<TableType, { label: string, icon: LucideIcon }> = {
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
  const [qrCodeLoading, setQrCodeLoading] = useState(false);

  // Image state
  const [tableImages, setTableImages] = useState<TableImageResponse[]>([]);
  const [pendingImages, setPendingImages] = useState<{ mediaId: number; previewUrl: string }[]>([]); // images queued before table creation

  // Confirm Dialog States
  const [deleteTableDialog, setDeleteTableDialog] = useState<{ open: boolean; id: number | null }>({ open: false, id: null });
  const [deleteImageDialog, setDeleteImageDialog] = useState<{ open: boolean; imageId: number | null }>({ open: false, imageId: null });
  const [deleteTagDialog, setDeleteTagDialog] = useState<{ open: boolean; tag: TableTag | null }>({ open: false, tag: null });

  const loadTables = async () => {
    setLoading(true);
    try {
      const response = await apiGet<{ tables: TableResponse[] }>("/tables");
      setTables(response.tables || []);
    } catch {
      toast.error("加载桌台失败");
    } finally {
      setLoading(false);
    }
  };

  const loadTags = async () => {
    try {
      const response = await apiGet<{ tags: TableTag[] }>("/tags", { type: "table" });
      setAvailableTags(response.tags || []);
    } catch {
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
    if (!e.target.files || !e.target.files[0]) return;
    
    const file = e.target.files[0];
    try {
      setLoading(true);
      const { mediaId, urls } = await uploadMedia(file, {
        businessType: "merchant",
        mediaCategory: "table",
      });
      const previewUrl = urls["card"] ?? urls["original"] ?? "";
      
      if (editingTable) {
        // 直接关联到已有桌台
        await apiPost(`/tables/${editingTable.id}/images`, { media_asset_id: mediaId });
        toast.success("图片上传成功");
        loadTableImages(editingTable.id);
      } else {
        // 尚未创建桶台，暂存 asset ID + 预览 URL
        setPendingImages(prev => [...(prev ?? []), { mediaId, previewUrl }]);
        toast.success("图片已暂存，保存桌台后将生效");
      }
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "上传失败";
      toast.error(message);
    } finally {
      setLoading(false);
      e.target.value = "";
    }
  };

  const handleDeleteImage = async (imageId: number) => {
    if (!editingTable) return;
    setDeleteImageDialog({ open: true, imageId });
  };

  const confirmDeleteImage = async () => {
    if (!editingTable || !deleteImageDialog.imageId) return;
    try {
      await apiDelete(`/tables/${editingTable.id}/images/${deleteImageDialog.imageId}`);
      toast.success("图片已删除");
      loadTableImages(editingTable.id);
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "删除失败";
      toast.error(message);
    }
  };

  const handleSetPrimaryImage = async (imageId: number) => {
    if (!editingTable) return;
    try {
      await apiPut(`/tables/${editingTable.id}/images/${imageId}/primary`, {});
      toast.success("已设为主图");
      loadTableImages(editingTable.id);
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "设置失败";
      toast.error(message);
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
    setPendingImages([]);
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
        const response = await apiPost<TableResponse>("/tables", data);
        
        // Handle pending images
        if (pendingImages && pendingImages.length > 0 && response.id) {
          toast.info(`正在关联 ${pendingImages.length} 张照片...`);
          for (const { mediaId } of pendingImages) {
            await apiPost(`/tables/${response.id}/images`, { media_asset_id: mediaId });
          }
        }
        
        toast.success("桌台已创建");
      }
      setIsSheetOpen(false);
      loadTables();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "保存失败";
      toast.error(message);
    } finally {
      setLoading(false);
    }
  };

  const handleDeleteTable = async (id: number) => {
    setDeleteTableDialog({ open: true, id });
  };

  const openQRCodeDialog = async (table: TableResponse) => {
    setQrCodeDialog({ open: true, table });
    setQrCodeLoading(true);
    try {
      const res = await apiGet<{ qr_code_url: string }>(`/tables/${table.id}/qrcode`);
      const qrCodeUrl = res?.qr_code_url || table.qr_code_url;
      const nextTable = qrCodeUrl ? { ...table, qr_code_url: qrCodeUrl } : table;
      setQrCodeDialog({ open: true, table: nextTable });
      setTables((prev) => prev.map((item) => (item.id === table.id ? { ...item, qr_code_url: qrCodeUrl } : item)));
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "获取二维码失败";
      toast.error(message);
    } finally {
      setQrCodeLoading(false);
    }
  };

  const confirmDeleteTable = async () => {
    if (!deleteTableDialog.id) return;
    try {
      await apiDelete(`/tables/${deleteTableDialog.id}`);
      toast.success("桌台已删除");
      loadTables();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "删除失败";
      toast.error(message);
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
          <div className="flex flex-col md:flex-row gap-4 items-center justify-between bg-white p-4 rounded-xl border shadow-sm">
            <div className="flex items-center gap-2 w-full md:w-auto">
              <div className="relative w-full md:w-80">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input 
                  placeholder="搜索桌号..." 
                  className="pl-9 bg-slate-50 border-slate-200"
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
                        <Button variant="ghost" size="icon" className="h-8 w-8 hover:bg-primary/10 hover:text-primary rounded-full" onClick={(e: React.MouseEvent) => { e.stopPropagation(); openQRCodeDialog(table); }}>
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
              className="border-dashed border-2 hover:border-primary/50 hover:bg-primary/5 transition-all cursor-pointer flex flex-col items-center justify-center p-6 h-full min-h-40"
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
        <SheetContent className="w-full sm:max-w-3xl p-0 flex flex-col h-full overflow-hidden">
          <SheetHeader className="p-6 border-b shrink-0 bg-white/80 backdrop-blur-md z-20">
            <SheetTitle className="text-xl font-bold">{editingTable ? "编辑桌台详情" : "创建新桌台"}</SheetTitle>
            <SheetDescription>
              填写桌台的基本信息、位置描述和配置详情，以便顾客自助点餐。
            </SheetDescription>
          </SheetHeader>

          <ScrollArea className="flex-1">
            <div className="space-y-10 py-8 px-8 mx-1">
              {/* Basic Info */}
              <div className="space-y-6">
                <div className="grid gap-2">
                  <Label htmlFor="table_no" className="text-sm font-semibold text-slate-700 uppercase tracking-wider">桌号/房号 *</Label>
                  <Input 
                    id="table_no" 
                    placeholder="如：A01，大厅-05，VIP包间-1" 
                    value={formData.table_no}
                    onChange={(e) => setFormData({...formData, table_no: e.target.value})}
                    className="h-11 text-base font-medium border-slate-200 focus:border-primary transition-colors"
                  />
                </div>

                <div className="grid grid-cols-2 gap-8">
                  <div className="grid gap-2">
                    <Label htmlFor="table_type" className="text-sm font-semibold text-slate-700 uppercase tracking-wider">桌台类型</Label>
                    <Select 
                      value={formData.table_type} 
                      onValueChange={(val: string) => setFormData({ ...formData, table_type: val as TableType })}
                    >
                      <SelectTrigger id="table_type" className="h-11 border-slate-200 bg-white">
                        <SelectValue placeholder="选择类型" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="table">大厅桌台</SelectItem>
                        <SelectItem value="room">包间</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="capacity" className="text-sm font-semibold text-slate-700 uppercase tracking-wider">最大容纳人数 *</Label>
                    <div className="relative">
                      <Users className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-slate-400" />
                      <Input 
                        id="capacity" 
                        type="number"
                        min={1}
                        max={100}
                        value={formData.capacity}
                        onChange={(e) => setFormData({...formData, capacity: parseInt(e.target.value) || 1})}
                        className="pl-10 h-11 border-slate-200 font-bold"
                      />
                    </div>
                  </div>
                </div>

                <div className="grid gap-2">
                  <Label htmlFor="minimum_spend" className="text-sm font-semibold text-slate-700 uppercase tracking-wider">最低消费额度 (¥)</Label>
                  <div className="relative">
                    <span className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 font-bold">¥</span>
                    <Input 
                      id="minimum_spend" 
                      type="number"
                      step="0.01"
                      placeholder="0.00 (不填则无最低消费)"
                      className="pl-7 h-11 text-base font-bold border-slate-200"
                      value={formData.minimum_spend !== undefined ? (formData.minimum_spend / 100).toString() : ""}
                      onChange={(e) => {
                        const val = e.target.value;
                        setFormData({...formData, minimum_spend: val === "" ? undefined : Math.round(parseFloat(val) * 100)});
                      }}
                    />
                  </div>
                </div>

                <div className="grid gap-2">
                  <Label htmlFor="description" className="text-sm font-semibold text-slate-700 uppercase tracking-wider">位置描述/备注</Label>
                  <Textarea 
                    id="description" 
                    placeholder="描述该桌台的具体位置，如：靠近落落地窗、江景视野、靠近过道等..." 
                    className="resize-none h-24 border-slate-200 focus:bg-slate-50/50"
                    value={formData.description}
                    onChange={(e) => setFormData({...formData, description: e.target.value})}
                  />
                </div>
              </div>

              <Separator className="bg-slate-100" />

              {/* Photos Section */}
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <div className="space-y-1">
                    <Label className="text-sm font-semibold text-slate-700 uppercase tracking-wider">实景照片</Label>
                    <p className="text-[10px] text-slate-400 font-medium">展示桌台或包间的真实环境</p>
                  </div>
                  <Button variant="outline" size="sm" className="h-8 border-primary text-primary hover:bg-primary/10" asChild>
                    <label className="cursor-pointer">
                      <ImagePlus className="h-3.5 w-3.5 mr-1.5" />上传照片
                      <input type="file" className="hidden" accept="image/*" onChange={handleUploadImage} disabled={loading} />
                    </label>
                  </Button>
                </div>

                <div className="grid grid-cols-2 gap-6">
                  {/* Existing Images */}
                  {tableImages.map((img) => (
                    <div key={img.id} className="relative aspect-4/3 group rounded-2xl overflow-hidden border border-slate-200 bg-slate-50 shadow-sm transition-all hover:border-primary/50">
                      <Image 
                        src={getMediaUrl(img.image_url)} 
                        alt="桌台实景" 
                        width={400}
                        height={300}
                        className="w-full h-full object-cover"
                      />
                      {img.is_primary && (
                        <div className="absolute top-3 left-3 bg-primary/90 backdrop-blur-sm text-white text-[9px] px-2 py-1 rounded shadow-sm font-bold uppercase tracking-wider">
                          主图
                        </div>
                      )}
                      <div className="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity flex flex-col items-center justify-center gap-3">
                        {!img.is_primary && (
                          <Button size="sm" variant="secondary" className="h-8 text-[11px] px-3 font-bold" onClick={() => handleSetPrimaryImage(img.id)}>
                            设为主图
                          </Button>
                        )}
                        <Button size="icon" variant="destructive" className="h-9 w-9 rounded-full shadow-lg" onClick={() => handleDeleteImage(img.id)}>
                          <Trash2 className="h-5 w-5" />
                        </Button>
                      </div>
                    </div>
                  ))}

                  {/* Pending Images (Creation mode) */}
                  {!editingTable && (pendingImages ?? []).map((item, idx) => (
                    <div key={`pending-${idx}`} className="relative aspect-4/3 rounded-2xl overflow-hidden border-2 border-primary/30 bg-slate-50 shadow-sm group">
                      <Image 
                        src={getMediaUrl(item.previewUrl)} 
                        alt="待保存图片" 
                        width={400}
                        height={300}
                        className="w-full h-full object-cover opacity-80"
                      />
                      <div className="absolute top-2 left-2 bg-primary/80 text-white text-[8px] px-1.5 py-0.5 rounded font-bold uppercase">
                        待保存
                      </div>
                      <div className="absolute inset-0 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity bg-black/20">
                        <Button 
                          size="icon" 
                          variant="destructive" 
                          className="h-8 w-8 rounded-full"
                          onClick={() => setPendingImages(prev => (prev ?? []).filter((_, i) => i !== idx))}
                        >
                          <X className="h-4 w-4" />
                        </Button>
                      </div>
                    </div>
                  ))}

                  <div 
                    className={cn(
                      "flex flex-col items-center justify-center border-2 border-dashed border-slate-200 rounded-2xl bg-slate-50/50 cursor-pointer hover:bg-slate-50 transition-colors",
                      (tableImages.length === 0 && pendingImages.length === 0) ? "col-span-2 py-16" : "aspect-4/3"
                    )}
                    onClick={() => {
                      const input = document.querySelector('input[type="file"]') as HTMLInputElement;
                      if (input) input.click();
                    }}
                  >
                    <div className="w-12 h-12 rounded-full bg-slate-100 flex items-center justify-center text-slate-400 mb-2">
                      <ImagePlus className="h-6 w-6" />
                    </div>
                    <span className="text-sm font-medium text-slate-500">点击上传桌台照片</span>
                  </div>
                </div>
              </div>

              <Separator className="bg-slate-100" />

              {/* Tags Section */}
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <div className="space-y-1">
                    <Label className="text-sm font-semibold text-slate-700 uppercase tracking-wider">桌台属性标签</Label>
                    <p className="text-[10px] text-slate-400 font-medium">标注桌台特色，如江景、空调、靠近门口等</p>
                  </div>
                  <Button variant="ghost" size="sm" className="h-8 text-xs font-bold text-primary hover:bg-primary/5" onClick={() => setIsTagDialogOpen(true)}>
                    管理标签库
                  </Button>
                </div>
                <div className="flex flex-wrap gap-2.5 pt-1">
                  {availableTags.length > 0 ? (
                    availableTags.map(tag => (
                      <Badge 
                        key={tag.id}
                        variant={selectedTagIds.includes(tag.id) ? "default" : "outline"}
                        className={cn(
                          "cursor-pointer px-4 py-2 h-auto font-medium transition-all text-xs",
                          selectedTagIds.includes(tag.id) 
                            ? "bg-primary text-white shadow-md shadow-primary/20 scale-105" 
                            : "hover:bg-slate-100 border-slate-200 bg-white"
                        )}
                        onClick={() => toggleTag(tag.id)}
                      >
                        {tag.name}
                        {selectedTagIds.includes(tag.id) && <CheckCircle2 className="ml-2 h-4 w-4 fill-white text-primary" />}
                      </Badge>
                    ))
                  ) : (
                    <p className="text-xs text-slate-400 italic">暂无可用标签，请点击上方管理标签添加</p>
                  )}
                </div>
              </div>
              

            </div>
          </ScrollArea>

          <SheetFooter className="p-6 border-t bg-white/80 backdrop-blur-md shrink-0 flex items-center justify-end gap-3 z-20">
            <Button variant="ghost" onClick={() => setIsSheetOpen(false)} disabled={loading}>取消操作</Button>
            <Button className="min-w-35 font-bold shadow-lg shadow-primary/20" onClick={handleSaveTable} disabled={loading}>
              {loading ? <><RefreshCw className="mr-2 h-4 w-4 animate-spin" /> 正在提交...</> : "保存设置并发布"}
            </Button>
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
              {qrCodeLoading ? (
                <div className="flex flex-col items-center justify-center text-muted-foreground gap-2">
                  <RefreshCw className="h-12 w-12 animate-spin opacity-50" />
                  <span className="text-xs">正在生成二维码...</span>
                </div>
              ) : qrCodeDialog.table?.qr_code_url ? (
                <Image 
                  src={getMediaUrl(qrCodeDialog.table.qr_code_url)} 
                  alt="桌台二维码" 
                  width={256}
                  height={256}
                  className="w-full h-full object-contain"
                />
              ) : (
                <div className="flex flex-col items-center justify-center text-muted-foreground gap-2">
                  <QrCode className="h-12 w-12 stroke-[1.5] opacity-20" />
                  <span className="text-xs">暂无二维码</span>
                </div>
              )}
            </div>
            <p className="text-xs text-center text-muted-foreground max-w-60">
              顾客扫描此二维码即可进入点餐页面进行自助下单
            </p>
          </div>
          <DialogFooter className="flex-col sm:flex-col gap-2">
            {qrCodeDialog.table?.qr_code_url ? (
              <Button className="w-full" asChild>
                <a href={getMediaUrl(qrCodeDialog.table.qr_code_url)} target="_blank" rel="noreferrer">
                  <Download className="h-4 w-4 mr-2" />
                  下载打印图片
                </a>
              </Button>
            ) : (
              <Button className="w-full" disabled>
                <Download className="h-4 w-4 mr-2" />
                暂无可下载二维码
              </Button>
            )}
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
                } catch {
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
                      onClick={() => setDeleteTagDialog({ open: true, tag })}
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

      {/* Confirm Dialogs */}
      <ConfirmDialog
        open={deleteTableDialog.open}
        onOpenChange={(open) => setDeleteTableDialog({ open, id: open ? deleteTableDialog.id : null })}
        title="删除桌台"
        description="确定要删除此桌台吗？此操作不可撤销。"
        confirmText="删除"
        variant="destructive"
        onConfirm={confirmDeleteTable}
      />
      <ConfirmDialog
        open={deleteImageDialog.open}
        onOpenChange={(open) => setDeleteImageDialog({ open, imageId: open ? deleteImageDialog.imageId : null })}
        title="删除图片"
        description="确定要删除这张图片吗？"
        confirmText="删除"
        variant="destructive"
        onConfirm={confirmDeleteImage}
      />
      <ConfirmDialog
        open={deleteTagDialog.open}
        onOpenChange={(open) => setDeleteTagDialog({ open, tag: open ? deleteTagDialog.tag : null })}
        title="删除标签"
        description={`确定要删除标签 "${deleteTagDialog.tag?.name}" 吗？`}
        confirmText="删除"
        variant="destructive"
        onConfirm={async () => {
          if (!deleteTagDialog.tag) return;
          try {
            await apiDelete(`/tags/${deleteTagDialog.tag.id}`);
            loadTags();
            toast.success("标签已删除");
          } catch {
            toast.error("删除失败");
          }
        }}
      />
    </PageShell>
  );
}
