"use client";

import { useState, useMemo, useEffect, useCallback } from "react";
import Image from "next/image";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { 
  RefreshCw, 
  Calendar,
  Clock,
  Users,
  Utensils,
  LayoutGrid, 
  List as ListIcon, 
  Phone, 
  UserCheck
} from "lucide-react";
import { toast } from "sonner";
import { 
  EventReservationNew, 
  EventReservationUpdate
} from "@/lib/constants";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { 
  Dialog, 
  DialogContent, 
  DialogDescription, 
  DialogFooter, 
  DialogHeader, 
  DialogTitle 
} from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Separator } from "@/components/ui/separator";
import { 
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { PageShell, PageHeader, PageContent } from "@/components/merchant/layout/page-shell";
import { apiGet, apiPost, apiPut, formatDate } from "@/lib/api";
import { cn } from "@/lib/utils";
import type { ReservationResponse, ReservationStatsResponse } from "@/types/reservation";
import type { TableResponse } from "@/types/table";

interface ReservationsPageClientProps {
  stats: ReservationStatsResponse;
  date: string;
  totalCount: number;
}

interface BusinessHour {
  day_of_week: number;
  open_time: string;
  close_time: string;
  is_closed: boolean;
}

interface MerchantProfile {
    id: number;
    business_hours?: BusinessHour[];
}

export function ReservationsPageClient({
  stats: initialStats,
  date,
  totalCount: initialTotalCount,
}: ReservationsPageClientProps) {
  const router = useRouter();
  
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [tables, setTables] = useState<TableResponse[]>([]);
  const [tableLoading, setTableLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [isCancelConfirmOpen, setIsCancelConfirmOpen] = useState(false);
  const [merchant, setMerchant] = useState<MerchantProfile | null>(null);

  // 动态数据状态
  const [scheduleReservations, setScheduleReservations] = useState<ReservationResponse[]>([]);
  const [statsState, setStatsState] = useState<ReservationStatsResponse>(initialStats);
  const [totalCountState, setTotalCountState] = useState(initialTotalCount);
  const [viewMode, setViewMode] = useState<"grid" | "list">("grid");

  const [formData, setFormData] = useState({
    table_id: "",
    date: date || formatDate(new Date()),
    time: "18:00",
    guest_count: "2",
    contact_name: "",
    contact_phone: "",
    notes: "",
  });

  const matchedReservation = useMemo(() => {
    if (!formData.table_id || !formData.date || !formData.time) return null;
    const currentHour = parseInt(formData.time.split(":")[0]);
    const isLunchSlot = currentHour < 15;

    return scheduleReservations.find(r => {
      if (r.table_id !== parseInt(formData.table_id) || r.reservation_date !== formData.date) return false;
      if (['cancelled', 'no_show', 'completed'].includes(r.status)) return false;
      
      const resHour = parseInt(r.reservation_time.split(":")[0]);
      return isLunchSlot ? resHour < 15 : resHour >= 15;
    });
  }, [formData.table_id, formData.date, formData.time, scheduleReservations]);

  useEffect(() => {
    if (matchedReservation) {
      setFormData(f => ({
        ...f,
        time: matchedReservation.reservation_time,
        contact_name: matchedReservation.contact_name,
        contact_phone: matchedReservation.contact_phone,
        guest_count: matchedReservation.guest_count.toString(),
        notes: matchedReservation.notes || "",
      }));
    } else {
      setFormData(f => ({
        ...f,
        contact_name: "",
        contact_phone: "",
        guest_count: "2",
        notes: "",
      }));
    }
  }, [matchedReservation]);

  const next6Days = useMemo(() => {
    const days = [];
    const today = new Date();
    for (let i = 0; i < 6; i++) {
      const d = new Date(today);
      d.setDate(today.getDate() + i);
      days.push({
        dateStr: formatDate(d),
        label: i === 0 ? "今天" : i === 1 ? "明天" : formatDate(d).split("-").slice(1).join("/"),
        fullLabel: formatDate(d)
      });
    }
    return days;
  }, []);



  const loadMerchantProfile = useCallback(async () => {
    try {
      const profile = await apiGet<MerchantProfile>("/merchants/me");
      setMerchant(profile);
    } catch (error) {
      console.error("Failed to load merchant profile:", error);
    }
  }, []);

  const loadTables = useCallback(async () => {
    setTableLoading(true);
    try {
      const data = await apiGet<{ tables: TableResponse[] }>("/tables", { page_size: 100 });
      setTables(data.tables.filter(t => t.table_type === "room"));
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "加载桌台失败";
      toast.error(message);
    } finally {
      setTableLoading(false);
    }
  }, []);

  const loadScheduleReservations = useCallback(async () => {
    try {
      const data = await apiGet<{ reservations: ReservationResponse[], total?: number }>("/reservations/merchant", {
        page_id: 1,
        page_size: 200 
      });
      setScheduleReservations(data.reservations || []);
      if (data.total !== undefined) {
        setTotalCountState(data.total);
      }
    } catch (error) {
      console.error("Failed to load schedule reservations:", error);
    }
  }, []);

  const loadStats = useCallback(async () => {
    try {
      const newStats = await apiGet<ReservationStatsResponse>("/reservations/merchant/stats");
      setStatsState(newStats);
    } catch (error) {
      console.error("Failed to load stats:", error);
    }
  }, []);

  useEffect(() => {
    loadTables();
    loadScheduleReservations();
    loadMerchantProfile();
    loadStats();
  }, [loadTables, loadScheduleReservations, loadMerchantProfile, loadStats]);

  const refreshAll = useCallback(async () => {
    await Promise.all([loadTables(), loadScheduleReservations(), loadStats(), loadMerchantProfile()]);
  }, [loadTables, loadScheduleReservations, loadStats, loadMerchantProfile]);

  // 1. 监听 WebSocket 实时消息
  useEffect(() => {
    const handleRealtimeMessage = (event: Event) => {
      const customEvent = event as CustomEvent;
      const msg = customEvent.detail;
      
      if (!msg) return;

      if (msg.type === EventReservationNew) {
        const newRes = msg.data?.reservation as ReservationResponse;
        toast.info(`收到新预订：${newRes?.contact_name || '新客人'} (${newRes?.guest_count}人) - ${newRes?.reservation_time}`, {
          duration: 5000,
          action: {
            label: "查看",
            onClick: () => setViewMode('list')
          }
        });
        refreshAll();
      } else if (msg.type === EventReservationUpdate) {
         // 预订状态变更（如用户取消）
         refreshAll();
      }
    };

    window.addEventListener("merchant-realtime", handleRealtimeMessage);
    return () => {
      window.removeEventListener("merchant-realtime", handleRealtimeMessage);
    };
  }, [refreshAll]);

  const timeSlots = useMemo(() => {
    if (!merchant || !merchant.business_hours || !formData.date) {
      // Fallback if no data
      return { 
        lunch: ["11:00", "11:30", "12:00", "12:30", "13:00", "13:30"],
        dinner: ["17:00", "17:30", "18:00", "18:30", "19:00", "19:30", "20:00", "20:30"]
      };
    }
    
    const dayOfWeek = new Date(formData.date).getDay();
    const hours = merchant.business_hours.find(h => h.day_of_week === dayOfWeek);

    if (!hours || hours.is_closed) {
      return { lunch: [], dinner: [] };
    }

    const slots: string[] = [];
    const [openH, openM] = hours.open_time.split(':').map(Number);
    const [closeH, closeM] = hours.close_time.split(':').map(Number);
    
    let currentInMinutes = openH * 60 + openM;
    const closeInMinutes = closeH * 60 + closeM;
    
    while (currentInMinutes <= closeInMinutes - 30) { // 至少预留30分钟
      const h = Math.floor(currentInMinutes / 60);
      const m = currentInMinutes % 60;
      const timeStr = `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}`;
      slots.push(timeStr);
      currentInMinutes += 30;
    }

    if (slots.length === 0) return { lunch: [], dinner: [] };

    // 简单以 16:00 分界午晚市
    const lunch = slots.filter(t => parseInt(t.split(':')[0]) < 16);
    const dinner = slots.filter(t => parseInt(t.split(':')[0]) >= 16);
    
    return { lunch, dinner };
  }, [merchant, formData.date]);

  const openCreateDialog = (tableId: number, targetDate?: string, preferredTime?: string) => {
    const targetDay = targetDate || formData.date;
    const isTargetingLunch = preferredTime 
      ? parseInt(preferredTime.split(":")[0]) < 15 
      : parseInt(formData.time.split(":")[0]) < 15;

    const existingRes = scheduleReservations.find(r => 
      r.table_id === tableId && 
      r.reservation_date === targetDay && 
      !['cancelled', 'no_show', 'completed'].includes(r.status) &&
      (isTargetingLunch ? parseInt(r.reservation_time.split(":")[0]) < 15 : parseInt(r.reservation_time.split(":")[0]) >= 15)
    );

    setFormData(f => ({ 
      ...f, 
      table_id: tableId.toString(),
      date: targetDay,
      time: existingRes ? existingRes.reservation_time : (preferredTime || (isTargetingLunch ? "11:00" : "17:30"))
    }));
    setIsCreateOpen(true);
  };

  const handleCreate = async () => {
    if (!formData.table_id || !formData.contact_name || !formData.contact_phone) {
      toast.error("请填写完整信息");
      return;
    }
    setCreating(true);
    try {
      await apiPost("/reservations/merchant/create", {
        ...formData,
        table_id: parseInt(formData.table_id),
        guest_count: parseInt(formData.guest_count),
        source: "merchant"
      });
      setFormData(f => ({
        ...f,
        contact_name: "",
        contact_phone: "",
        guest_count: "2",
        table_id: "", // Reset table selection
        notes: "",
      }));
      toast.success("预订成功");
      setIsCreateOpen(false);
      await refreshAll();
      router.refresh();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "创建失败";
      toast.error(message);
    } finally {
      setCreating(false);
    }
  };

  const handleUpdate = async () => {
    if (!matchedReservation) return;
    setCreating(true);
    try {
      await apiPut(`/reservations/${matchedReservation.id}/update`, {
        ...formData,
        table_id: parseInt(formData.table_id),
        guest_count: parseInt(formData.guest_count),
      });
      toast.success("修改成功");
      setIsCreateOpen(false);
      await refreshAll();
      router.refresh();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "修改失败";
      toast.error(message);
    } finally {
      setCreating(false);
    }
  };

  const handleCancelReservation = async () => {
    if (!matchedReservation) return;
    setActionLoading(true);
    try {
      await apiPost(`/reservations/${matchedReservation.id}/cancel`, { reason: "商户手动取消" });
      toast.success("已取消预订");
      setIsCreateOpen(false);
      setIsCancelConfirmOpen(false);
      await refreshAll();
      router.refresh();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "取消失败";
      toast.error(message);
    } finally {
      setActionLoading(false);
    }
  };

  const handleCheckIn = async (reservationId: number, tableId: number) => {
    setActionLoading(true);
    try {
      await apiPost("/dining-sessions/open", {
        table_id: tableId,
        reservation_id: reservationId
      });
      toast.success("客人签到成功，已开台");
      await refreshAll();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "签到失败";
      toast.error(message);
    } finally {
      setActionLoading(false);
    }
  };

  const TableCard = ({ table }: { table: TableResponse }) => {
    return (
      <Card className={cn(
        "group relative overflow-hidden transition-all hover:shadow-lg border-muted/60 flex flex-col",
        table.status === 'disabled' && "opacity-80 grayscale-[0.2] bg-muted/20"
      )}>
        <div className={cn(
          "absolute top-0 left-0 right-0 h-1.5",
          table.status === 'available' ? "bg-emerald-500" :
          table.status === 'occupied' ? "bg-primary" :
          table.status === 'reserved' ? "bg-amber-500" : "bg-muted-foreground"
        )} />

        <CardHeader className="p-3 pb-1 shrink-0">
          <div className="flex items-start justify-between">
            <div className="space-y-0.5">
              <div className="flex items-center gap-2">
                <h3 className="text-xl font-bold tracking-tight">{table.table_no}</h3>
                <Badge className={cn(
                  "text-[9px] px-1 h-3.5 font-normal text-white border-0", 
                  table.status === 'available' ? "bg-emerald-500" : 
                  table.status === 'occupied' ? "bg-primary" : 
                  table.status === 'reserved' ? "bg-amber-500" : "bg-muted-foreground"
                )}>
                  {table.status === 'available' ? '空闲' : table.status === 'occupied' ? '用餐中' : table.status === 'reserved' ? '已预定' : '停用'}
                </Badge>
              </div>
              <div className="flex items-center text-[10px] text-muted-foreground gap-2">
                <span className="flex items-center gap-1"><Users className="h-3 w-3" /> {table.capacity} 人</span>
              </div>
            </div>
          </div>
          <div className="flex flex-wrap gap-1 mt-1.5">
             {table.tags?.map(tag => (
               <Badge key={tag.id} variant="secondary" className="text-[10px] h-4 bg-muted text-muted-foreground px-1.5 font-normal">
                 {tag.name}
               </Badge>
             ))}
          </div>
        </CardHeader>

        <CardContent className="p-3 pt-1 flex flex-col">
          <Separator className="my-1.5 opacity-50" />
          <div className="mt-0.5">
             <div className="text-[11px] font-semibold text-muted-foreground mb-2 flex items-center justify-between">
                <span>接单预报 (6 日内)</span>
                <span className="text-[8px] font-normal opacity-60">点击时段详情</span>
             </div>
             <div className="grid grid-cols-3 gap-2">
                {next6Days.map(day => {
                   const dayRes = scheduleReservations.filter(r => 
                     r.table_id === table.id && 
                     r.reservation_date === day.dateStr && 
                     !['cancelled', 'no_show', 'completed'].includes(r.status)
                   );
                   const hasLunch = dayRes.some(r => parseInt(r.reservation_time.split(":")[0]) < 15);
                   const hasDinner = dayRes.some(r => parseInt(r.reservation_time.split(":")[0]) >= 15);

                   return (
                     <div key={day.dateStr} className={cn(
                       "flex flex-col items-center justify-center py-2 rounded-xl border transition-all bg-emerald-50/30 border-emerald-100/50 group/day",
                       (hasLunch || hasDinner) && "bg-amber-50 border-amber-200"
                     )}>
                       <span className="text-xs font-bold leading-none mb-2 text-slate-700">{day.label}</span>
                       <div className="flex justify-between w-full px-2">
                          <div className="flex flex-col items-center gap-1.5 cursor-pointer hover:scale-110 active:scale-95 transition-transform" onClick={() => openCreateDialog(table.id, day.dateStr, "11:00")}>
                            <span className="text-[9px] font-black text-slate-400 leading-none">午</span>
                            <div className={cn("h-2.5 w-2.5 rounded-full shadow-sm ring-2 ring-transparent transition-all", hasLunch ? "bg-amber-500 animate-pulse ring-amber-100" : "bg-emerald-400")} />
                          </div>
                          <div className="flex flex-col items-center gap-1.5 cursor-pointer hover:scale-110 active:scale-95 transition-transform" onClick={() => openCreateDialog(table.id, day.dateStr, "17:30")}>
                            <span className="text-[9px] font-black text-slate-400 leading-none">晚</span>
                            <div className={cn("h-2.5 w-2.5 rounded-full shadow-sm ring-2 ring-transparent transition-all", hasDinner ? "bg-amber-500 animate-pulse ring-amber-100" : "bg-emerald-400")} />
                          </div>
                       </div>
                     </div>
                   );
                })}
             </div>
          </div>
          <div className="mt-3">
             {table.current_reservation && (
                <div className="p-1.5 bg-primary/5 rounded-lg border border-primary/10">
                   <div className="flex items-center justify-between text-[11px] font-bold text-primary mb-0.5">
                    <span className="flex items-center gap-1"><Clock className="h-2.5 w-2.5" /> {table.current_reservation.reservation_time} 到访</span>
                   </div>
                   <div className="text-[10px] text-muted-foreground flex items-center justify-between">
                    <span className="truncate max-w-15">{table.current_reservation.contact_name}</span>
                    <span className="flex items-center gap-1 shrink-0"><Users className="h-2.5 w-2.5" /> {table.current_reservation.guest_count}人</span>
                  </div>
                </div>
             )}
          </div>
        </CardContent>
      </Card>
    );
  };

  return (
    <PageShell>
      <PageHeader 
        title="预订管理" 
        description="管理包间预订、签到及菜品预备状态"
        actions={
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" className="h-9 px-4 border-slate-200" asChild>
              <Link href="/merchant/reservations/dishes">预订备菜清单</Link>
            </Button>
            <Button variant="outline" size="sm" className="h-9 px-4 border-slate-200" onClick={refreshAll}>
              <RefreshCw className="h-4 w-4 mr-2" />刷新
            </Button>
          </div>
        }
      />
      <PageContent>
        <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4 mb-6">
          <Card className="bg-white"><div className="p-4"><div className="text-xs text-muted-foreground mb-1 font-medium">待确认支付</div><div className="text-2xl font-bold text-blue-600">{statsState.paid_count}</div></div></Card>
          <Card className="bg-white"><div className="p-4"><div className="text-xs text-muted-foreground mb-1 font-medium">已确认预约</div><div className="text-2xl font-bold text-emerald-600">{statsState.confirmed_count}</div></div></Card>
          <Card className="bg-white"><div className="p-4"><div className="text-xs text-muted-foreground mb-1 font-medium">正在用餐</div><div className="text-2xl font-bold text-purple-600">{statsState.checked_in_count || 0}</div></div></Card>
          <Card className="bg-white"><div className="p-4"><div className="text-xs text-muted-foreground mb-1 font-medium">今日已结账</div><div className="text-2xl font-bold text-slate-600">{statsState.completed_count}</div></div></Card>
          <Card className="bg-white"><div className="p-4"><div className="text-xs text-muted-foreground mb-1 font-medium">未到店次数</div><div className="text-2xl font-bold text-rose-700">{statsState.no_show_count}</div></div></Card>
          <Card className="bg-primary/5 border-primary/20"><div className="p-4"><div className="text-xs text-primary/70 mb-1 font-semibold uppercase tracking-wider">总预订量</div><div className="text-2xl font-black text-primary">{totalCountState}</div></div></Card>
        </div>

        <div className="bg-white rounded-2xl border shadow-xl p-6 min-h-[calc(100vh-16rem)] overflow-y-auto">
          <div className="flex items-center justify-between mb-8">
             <div className="flex items-center gap-6">
               <h3 className="text-xl font-black text-slate-900 flex items-center gap-3"><Utensils className="h-6 w-6 text-primary" /> 房间/桌位状态看板</h3>
               <div className="flex items-center gap-4 text-[10px] font-bold uppercase tracking-widest text-muted-foreground bg-slate-50 px-4 py-2 rounded-full border border-slate-100 shadow-sm">
                  <span className="flex items-center gap-2"><span className="h-2 w-2 rounded-full bg-emerald-500"></span> 空闲</span>
                  <span className="flex items-center gap-2"><span className="h-2 w-2 rounded-full bg-primary"></span> 用餐中</span>
                  <span className="flex items-center gap-2"><span className="h-2 w-2 rounded-full bg-amber-500"></span> 已预定</span>
               </div>
             </div>
             
             <div className="flex bg-slate-100 p-1 rounded-lg">
                <Button 
                  variant={viewMode === 'grid' ? 'secondary' : 'ghost'} 
                  size="sm" 
                  className={cn("h-8 px-3 rounded-md transition-all", viewMode === 'grid' && "bg-white shadow-sm font-bold text-primary")}
                  onClick={() => setViewMode('grid')}
                >
                  <LayoutGrid className="h-4 w-4 mr-1.5" /> 房态视图
                </Button>
                <Button 
                  variant={viewMode === 'list' ? 'secondary' : 'ghost'} 
                  size="sm" 
                  className={cn("h-8 px-3 rounded-md transition-all", viewMode === 'list' && "bg-white shadow-sm font-bold text-primary")}
                  onClick={() => setViewMode('list')}
                >
                  <ListIcon className="h-4 w-4 mr-1.5" /> 预订列表
                </Button>
             </div>
          </div>

          {tableLoading ? (
             <div className="flex flex-col items-center justify-center py-32 gap-6">
               <RefreshCw className="h-12 w-12 text-primary animate-spin opacity-40" />
               <p className="text-sm font-black text-slate-400 tracking-tighter uppercase">加载中...</p>
             </div>
          ) : viewMode === 'grid' ? (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5 gap-8">
              {tables.map(table => <TableCard key={table.id} table={table} />)}
            </div>
          ) : (
            <div className="space-y-3">
              {scheduleReservations.length === 0 ? (
                 <div className="text-center py-20 text-muted-foreground bg-slate-50 rounded-xl border border-dashed">
                    <Calendar className="h-10 w-10 mx-auto mb-2 text-slate-300" />
                    <p>暂无预订记录</p>
                 </div>
              ) : (
                scheduleReservations.map(res => (
                  <div key={res.id} className="bg-white border rounded-xl p-4 transition-all hover:shadow-md hover:border-primary/30 group">
                     <div className="flex items-start justify-between">
                        <div className="flex items-start gap-4">
                           <div className="h-12 w-12 rounded-xl bg-primary/5 text-primary flex flex-col items-center justify-center border border-primary/10">
                              <span className="text-[10px] font-bold uppercase leading-none">{res.reservation_date.slice(5).replace('-','/')}</span>
                              <span className="text-lg font-black leading-none mt-0.5">{res.reservation_time}</span>
                           </div>
                           <div>
                              <div className="flex items-center gap-2 mb-1">
                                 <h4 className="font-bold text-lg text-slate-900">{res.contact_name}</h4>
                                 <Badge variant="outline" className="text-[10px] h-5 px-1.5 font-normal text-slate-500 bg-slate-50">
                                   {res.guest_count} 人
                                 </Badge>
                                 <Badge className={cn(
                                   "text-[10px] h-5 px-1.5 font-bold border-0",
                                   res.status === 'confirmed' || res.status === 'paid' ? "bg-emerald-100 text-emerald-700" :
                                   res.status === 'pending' ? "bg-amber-100 text-amber-700" :
                                   res.status === 'cancelled' || res.status === 'no_show' ? "bg-slate-100 text-slate-500 line-through" :
                                   "bg-blue-100 text-blue-700"
                                 )}>
                                   {res.status === 'pending' ? '待支付' :
                                    res.status === 'paid' ? '已支付' :
                                    res.status === 'confirmed' ? '已确认' :
                                    res.status === 'checked_in' ? '已入座' :
                                    res.status === 'completed' ? '已完成' :
                                    res.status === 'cancelled' ? '已取消' : '未到店'}
                                 </Badge>
                              </div>
                              <div className="flex items-center gap-3 text-xs text-muted-foreground font-medium">
                                 <span className="flex items-center gap-1"><Users className="h-3 w-3" /> {tables.find(t => t.id === res.table_id)?.table_no || '未知桌台'}</span>
                                 <span className="w-px h-3 bg-slate-200" />
                                 <span className="flex items-center gap-1"><Phone className="h-3 w-3" /> {res.contact_phone}</span>
                              </div>
                           </div>
                        </div>

                        <div className="flex items-center gap-2">
                           {/* Quick Actions for List View */}
                           {(res.status === 'paid' || res.status === 'confirmed') && (
                             <Button 
                               size="sm" 
                               className="h-9 px-4 rounded-lg bg-emerald-600 hover:bg-emerald-700 text-white font-bold shadow-sm shadow-emerald-600/20"
                               onClick={() => handleCheckIn(res.id, res.table_id)}
                               disabled={actionLoading}
                             >
                               <UserCheck className="h-4 w-4 mr-1.5" /> 确认到店
                             </Button>
                           )}
                           <Button 
                             variant="outline" 
                             size="sm" 
                             className="h-9 px-4 rounded-lg font-bold"
                             onClick={() => openCreateDialog(res.table_id, res.reservation_date, res.reservation_time)}
                           >
                             详情
                           </Button>
                        </div>
                     </div>
                     
                     {/* Dish Items Display */}
                     {res.items && res.items.length > 0 && (
                        <div className="mt-4 pt-3 border-t border-dashed pl-16">
                           <p className="text-[10px] font-bold text-slate-400 uppercase tracking-widest mb-2 flex items-center gap-2">
                             <Utensils className="h-3 w-3" /> 预点菜品 ({res.items.length})
                           </p>
                           <div className="flex flex-wrap gap-2">
                              {res.items.map((item, idx) => (
                                <Badge key={idx} variant="secondary" className="bg-slate-50 border-slate-200 text-slate-700 font-medium pl-1 pr-2 py-1 h-auto flex items-center gap-1.5">
                                   {item.image_url && (
                                     <Image src={item.image_url} alt={item.name} width={20} height={20} className="w-5 h-5 rounded object-cover" />
                                   )}
                                   <span className="text-[10px] bg-slate-200 px-1 rounded text-slate-600 font-bold">x{item.quantity}</span>
                                   <span>{item.name}</span>
                                </Badge>
                              ))}
                           </div>
                        </div>
                     )}
                  </div>
                ))
              )}
            </div>
          )}
        </div>
      </PageContent>

      <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
         <DialogContent className="sm:max-w-115 rounded-3xl">
            <DialogHeader className="space-y-3">
               <DialogTitle className="text-2xl font-black tracking-tight">管理业务预订</DialogTitle>
               <DialogDescription className="font-medium">处理电话预约、线下留位等场景，支持新增、修改及取消。</DialogDescription>
            </DialogHeader>
            <div className="grid gap-6 py-6 px-2">
               <div className="grid grid-cols-4 items-center gap-4">
                  <Label className="text-right font-black text-xs text-slate-500">预订桌位</Label>
                  <div className="col-span-3">
                    <Select value={formData.table_id} onValueChange={(v) => setFormData(f => ({...f, table_id: v}))}>
                       <SelectTrigger className="h-11 rounded-xl border-2 font-bold"><SelectValue placeholder="选择房号" /></SelectTrigger>
                       <SelectContent>{tables.map(t => <SelectItem key={t.id} value={t.id.toString()}>{t.table_no} ({t.capacity}人)</SelectItem>)}</SelectContent>
                    </Select>
                  </div>
               </div>
               <div className="grid grid-cols-4 items-center gap-4">
                  <Label className="text-right font-black text-xs text-slate-500">业务日期</Label>
                  <Input type="date" value={formData.date} onChange={e => setFormData(f => ({...f, date: e.target.value}))} className="col-span-3 h-11 rounded-xl border-2 font-black" />
               </div>
               <div className="grid grid-cols-4 items-center gap-4">
                  <Label className="text-right font-black text-xs text-slate-500">时间人数</Label>
                  <div className="col-span-3 flex gap-3">
                     <Select value={formData.time} onValueChange={v => setFormData(f => ({...f, time: v}))}>
                        <SelectTrigger className="flex-1 h-11 rounded-xl border-2 font-bold focus:ring-primary shadow-sm"><SelectValue placeholder="进店时间" /></SelectTrigger>
                        <SelectContent className="max-h-80 overflow-y-auto">
                           {timeSlots.lunch.length > 0 && (
                             <>
                               <div className="p-2 text-[10px] font-bold text-muted-foreground bg-muted/30 rounded-md mb-1 px-3">午餐时段</div>
                               {timeSlots.lunch.map(t => <SelectItem key={t} value={t} className="py-2.5">{t}</SelectItem>)}
                             </>
                           )}
                           {timeSlots.dinner.length > 0 && (
                             <>
                               <div className="p-2 text-[10px] font-bold text-muted-foreground bg-muted/30 rounded-md my-2 px-3">晚餐时段</div>
                               {timeSlots.dinner.map(t => <SelectItem key={t} value={t} className="py-2.5">{t}</SelectItem>)}
                             </>
                           )}
                           {timeSlots.lunch.length === 0 && timeSlots.dinner.length === 0 && (
                             <div className="p-4 text-center text-sm text-muted-foreground">该日期不营业或无时段</div>
                           )}
                        </SelectContent>
                     </Select>
                     <Input type="number" placeholder="人数" value={formData.guest_count} onChange={e => setFormData(f => ({...f, guest_count: e.target.value}))} className="h-11 flex-1 rounded-xl border-2 font-black" />
                  </div>
               </div>
               <Separator className="my-2" />
               <div className="grid grid-cols-4 items-center gap-4">
                  <Label className="text-right font-black text-xs text-slate-500">联系信息</Label>
                  <div className="col-span-3 flex flex-col gap-3">
                     <Input placeholder="客户姓名标识" value={formData.contact_name} onChange={e => setFormData(f => ({...f, contact_name: e.target.value}))} className="h-11 rounded-xl border-2 font-bold" />
                     <Input placeholder="联系电话 (必填)" value={formData.contact_phone} onChange={e => setFormData(f => ({...f, contact_phone: e.target.value}))} className="h-11 rounded-xl border-2 font-bold" />
                  </div>
               </div>
               <div className="grid grid-cols-4 items-start gap-4">
                  <Label className="text-right font-black text-xs text-slate-500 py-2">业务备注</Label>
                  <Textarea placeholder="是否有庆生、特殊禁忌?" value={formData.notes} onChange={e => setFormData(f => ({...f, notes: e.target.value}))} className="col-span-3 h-24 rounded-2xl border-2 font-medium" />
               </div>
            </div>
            <DialogFooter className="grid grid-cols-4 gap-2">
               <Button variant="ghost" className="font-bold rounded-xl" onClick={() => setIsCreateOpen(false)}>关 闭</Button>
               <Button variant="outline" className="text-rose-600 border-rose-100 hover:bg-rose-50 font-bold rounded-xl" disabled={!matchedReservation || actionLoading} onClick={() => setIsCancelConfirmOpen(true)}>取 消</Button>
               <Button variant="outline" className="font-bold rounded-xl" disabled={!matchedReservation || creating} onClick={handleUpdate}>更 新</Button>
               <Button className="font-black bg-primary shadow-md hover:scale-105 transition-all text-white rounded-xl" disabled={!!matchedReservation || creating} onClick={handleCreate}>{creating ? "处理中..." : "预 订"}</Button>
            </DialogFooter>
         </DialogContent>
      </Dialog>

      <AlertDialog open={isCancelConfirmOpen} onOpenChange={setIsCancelConfirmOpen}>
        <AlertDialogContent className="rounded-2xl">
          <AlertDialogHeader>
            <AlertDialogTitle className="text-xl font-black text-slate-900 tracking-tight">确认取消该时段预订？</AlertDialogTitle>
            <AlertDialogDescription className="text-sm font-medium">取消后该桌位在对应时段（{matchedReservation?.reservation_time}）将恢复为可预订状态。已支付订单将按系统退款策略处理（可能为部分退款）。此操作不可撤销。</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter className="gap-2">
            <AlertDialogCancel className="font-bold rounded-xl border-slate-200">暂不取消</AlertDialogCancel>
            <AlertDialogAction onClick={handleCancelReservation} className="bg-rose-600 hover:bg-rose-700 text-white font-bold rounded-xl border-0">确认取消</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </PageShell>
  );
}
