"use client";

import { useState, useEffect, useMemo, useCallback } from "react";
import Image from "next/image";
import { 
  RefreshCw, 
  Users, 
  Clock, 
  ArrowRightLeft, 
  LogOut, 
  CheckCircle2, 
  AlertCircle,
  Armchair,
  Search,
  LayoutGrid,
  List as ListIcon,
  ChefHat,
  Receipt,
  ChevronRight,
  TrendingUp,
  Calendar,
  Phone,
  Sparkles,
  UserCheck
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { 
  Dialog, 
  DialogContent, 
  DialogHeader, 
  DialogTitle, 
  DialogDescription, 
  DialogFooter 
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Progress } from "../ui/progress";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { PageShell, PageHeader, PageContent } from "@/components/merchant/layout/page-shell";
import { apiGet, apiPost, apiPatch, formatAmount } from "@/lib/api";
import { cn } from "@/lib/utils";
import type { TableResponse, TableStatus } from "@/types/table";
import type { OrderResponse } from "@/types/order";

interface EnhancedTable extends TableResponse {
  activeOrder?: OrderResponse;
  kitchenProgress?: {
    ready: number;
    total: number;
    percentage: number;
  };
  todayReservation?: ReservationSummary;
}

interface ReservationSummary {
  id: number;
  table_id: number;
  contact_name: string;
  contact_phone: string;
  reservation_time: string;
  table_no: string;
  guest_count: number;
}

const STATUS_CONFIG: Record<TableStatus, { label: string; color: string; bgColor: string; icon: LucideIcon; badgeVariant: "default" | "secondary" | "outline" | "destructive" }> = {
  available: { label: "空闲", color: "text-emerald-600", bgColor: "bg-emerald-50", icon: CheckCircle2, badgeVariant: "outline" },
  occupied: { label: "用餐中", color: "text-primary", bgColor: "bg-primary/5", icon: Armchair, badgeVariant: "default" },
  reserved: { label: "已预订", color: "text-amber-600", bgColor: "bg-amber-50", icon: AlertCircle, badgeVariant: "outline" },
  disabled: { label: "已停用", color: "text-muted-foreground", bgColor: "bg-slate-50", icon: LogOut, badgeVariant: "secondary" },
};

export function DineInPageClient() {
  const [tables, setTables] = useState<EnhancedTable[]>([]);
  const [recentReservations, setRecentReservations] = useState<ReservationSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [activeTab, setActiveTab] = useState("all");
  const [viewMode, setViewMode] = useState<"grid" | "list">("grid");

  // Interaction State
  const [actionLoading, setActionLoading] = useState<number | null>(null);
  
  // Dialogs
  const [confirmConfig, setConfirmConfig] = useState<{ 
    open: boolean; 
    type: 'open' | 'close' | 'reset'; 
    table: EnhancedTable | null 
  }>({ open: false, type: 'open', table: null });

  const [isTransferModalOpen, setIsTransferModalOpen] = useState(false);
  const [transferTargetId, setTransferTargetId] = useState<number | null>(null);
  const [isDetailModalOpen, setIsDetailModalOpen] = useState(false);
  const [selectedTable, setSelectedTable] = useState<EnhancedTable | null>(null);

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const tableResp = await apiGet<{ tables: TableResponse[] }>("/tables");
      const baseTables = tableResp.tables || [];

      const activeOrdersResp = await apiGet<{ orders: OrderResponse[] }>("/merchant/orders", { 
        page_id: 1, 
        page_size: 50 
      });
      const activeOrders = (activeOrdersResp.orders || []).filter(o => 
        ["paid", "preparing", "ready"].includes(o.status)
      );

      const resResp = await apiGet<{ reservations: ReservationSummary[] }>("/reservations/merchant/today");
      const dayReservations = resResp.reservations || [];
      setRecentReservations(dayReservations);

      const nowTimeStr = new Date().toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' });

      const enhanced: EnhancedTable[] = baseTables.map(t => {
        const order = activeOrders.find(o => o.table_id === t.id && o.order_type === 'dine_in');
        
        // Find nearest upcoming reservation
        const tableDayRes = dayReservations
          .filter(r => r.table_id === t.id)
          .sort((a, b) => a.reservation_time.localeCompare(b.reservation_time));
        const dayRes = tableDayRes.find(r => r.reservation_time >= nowTimeStr) || tableDayRes[0];
        
        let kitchenProgress = undefined;
        if (order && order.items) {
          const total = order.items.reduce((sum, item) => sum + item.quantity, 0);
          const readyCount = order.status === 'ready' ? total : (order.status === 'preparing' ? Math.floor(total * 0.6) : 0);
          kitchenProgress = {
            ready: readyCount,
            total,
            percentage: total > 0 ? (readyCount / total) * 100 : 0
          };
        }

        return { ...t, activeOrder: order, kitchenProgress, todayReservation: dayRes };
      });

      setTables(enhanced);
    } catch (error) {
      console.error(error);
      toast.error("同步实时数据失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadData();
    const timer = setInterval(loadData, 30000);
    return () => clearInterval(timer);
  }, [loadData]);

  const stats = useMemo(() => {
    const withReservation = tables.filter(t => t.todayReservation || t.status === 'reserved').length;
    return {
      total: tables.length,
      available: tables.filter(t => t.status === 'available').length,
      occupied: tables.filter(t => t.status === 'occupied').length,
      reserved: tables.filter(t => t.status === 'reserved').length,
      withReservation, // 有今日预约的桌台数
      todayReservations: recentReservations.length, // 今日预约总档数
    };
  }, [tables, recentReservations]);

  const filteredTables = useMemo(() => {
    return tables.filter(table => {
      const matchesSearch = table.table_no.toLowerCase().includes(searchQuery.toLowerCase());
      let matchesTab = true;
      if (activeTab === "all") {
        matchesTab = true;
      } else if (activeTab === "hasReservation") {
        matchesTab = !!table.todayReservation || table.status === 'reserved';
      } else {
        matchesTab = table.status === activeTab;
      }
      return matchesSearch && matchesTab;
    });
  }, [tables, searchQuery, activeTab]);

  // 注意：商户代开台功能已移除，开台操作只能由用户扫码完成

  const handleCloseTable = (table: EnhancedTable) => {
    setConfirmConfig({ open: true, type: 'close', table });
  };

  const handleResetTable = (table: EnhancedTable) => {
    setConfirmConfig({ open: true, type: 'reset', table });
  };

  // 判断预订是否在可签到时段（当前时间在预订时间前后30分钟内）
  const isReservationCheckInReady = (reservationTime: string): boolean => {
    const now = new Date();
    const today = now.toISOString().split('T')[0];
    const reservationDateTime = new Date(`${today}T${reservationTime}:00`);
    const thirtyMinutes = 30 * 60 * 1000;
    return now.getTime() >= reservationDateTime.getTime() - thirtyMinutes && 
           now.getTime() <= reservationDateTime.getTime() + thirtyMinutes;
  };

  // 预订到店签到（商户帮客户开台）
  const handleCheckinReservation = async (table: EnhancedTable) => {
    const reservation = table.current_reservation || table.todayReservation;
    if (!reservation) {
      toast.error("该桌台没有预订信息");
      return;
    }

    setActionLoading(table.id);
    try {
      await apiPost("/dining-sessions/open", {
        table_id: table.id,
        reservation_id: reservation.id
      });
      toast.success(`${reservation.contact_name} 的预订已签到入座`);
      await loadData();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "签到失败";
      toast.error(message);
    } finally {
      setActionLoading(null);
    }
  };

  const executeAction = async () => {
    if (!confirmConfig.table) return;
    const { type, table } = confirmConfig;
    
    setActionLoading(table.id);
    try {
      if (type === 'close') {
        // 商户不再手动调用订单完成接口，直接释放桌台即可触发后端的关闭会话事务
        await apiPatch(`/tables/${table.id}/status`, { status: 'available' });
        toast.success(`桌台 ${table.table_no} 结账完成并释放`);
      } else if (type === 'reset') {
        await apiPatch(`/tables/${table.id}/status`, { status: 'available' });
        toast.success(`桌台 ${table.table_no} 已完成清扫`);
      }
      await loadData();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "操作失败";
      toast.error(message);
    } finally {
      setActionLoading(null);
      setConfirmConfig(prev => ({ ...prev, open: false }));
    }
  };

  const executeTransfer = async () => {
    if (!selectedTable || !transferTargetId) return;
    
    // 转台只对正在用餐的桌台有效
    if (selectedTable.status !== 'occupied') {
      toast.error("只能对正在用餐的桌台进行转台操作");
      return;
    }
    
    setActionLoading(selectedTable.id);
    setIsTransferModalOpen(false);

    try {
      // 先获取当前桌台的活动会话（不创建新会话）
      // 由于后端 openDiningSession 现在会返回已有会话，这里仍可使用
      // 但我们已在前端限制只有 occupied 状态才能转台
      const openResp = await apiPost<{ session: { id: number } }>("/dining-sessions/open", { 
        table_id: selectedTable.id 
      });
      const sessionId = openResp.session.id;

      await apiPost(`/dining-sessions/${sessionId}/transfer-table`, {
        to_table_id: transferTargetId
      });

      toast.success(`桌台转移成功：${selectedTable.table_no} → ${tables.find(t => t.id === transferTargetId)?.table_no}`);
      await loadData();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "转台失败";
      toast.error(message);
    } finally {
      setActionLoading(null);
    }
  };

  const handleShowDetail = (table: EnhancedTable) => {
    setSelectedTable(table);
    setIsDetailModalOpen(true);
  };

  return (
    <PageShell>
      <PageHeader 
        title="堂食中心" 
        description="实时桌态监控与用餐会话管理"
        actions={
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={loadData} disabled={loading}>
              <RefreshCw className={cn("h-4 w-4 mr-2", loading && "animate-spin")} />
              刷新数据
            </Button>
            <Button size="sm">
              <ChefHat className="size-4 mr-2" />
              进入厨房系统
            </Button>
          </div>
        }
      />

      <PageContent>
        {/* Statistics Dashboard */}
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
          {[
            { label: "就餐中", value: stats.occupied, icon: TrendingUp, color: "text-primary", bg: "bg-primary/5" },
            { label: "空闲中", value: stats.available, icon: CheckCircle2, color: "text-emerald-600", bg: "bg-emerald-50" },
            { label: "待入座", value: stats.reserved, icon: Clock, color: "text-amber-600", bg: "bg-amber-50" },
            { label: "今日预约", value: stats.todayReservations, icon: Calendar, color: "text-blue-600", bg: "bg-blue-50" },
          ].map((s, i) => (
            <div key={i} className="bg-white rounded-xl border shadow-sm p-4 flex items-center gap-4">
              <div className={cn("size-10 rounded-lg flex items-center justify-center", s.bg)}>
                <s.icon className={cn("size-5", s.color)} />
              </div>
              <div>
                <p className="text-xs text-muted-foreground font-medium uppercase tracking-wider">{s.label}</p>
                <p className="text-2xl font-bold">{s.value}</p>
              </div>
            </div>
          ))}
        </div>

        {/* Master-Detail Style Layout */}
        <div className="flex flex-col lg:flex-row gap-6">
          {/* Sidebar Filters */}
          <div className="w-full lg:w-64 space-y-4">
            <section className="bg-white rounded-xl border shadow-sm p-4 space-y-4">
              <h3 className="text-sm font-semibold text-slate-900 border-l-4 border-primary pl-3">
                桌态筛选
              </h3>
              <div className="flex flex-col gap-1">
                {[
                  { id: "all", label: "全部桌位", count: stats.total, icon: LayoutGrid },
                  { id: "available", label: "空闲中", count: stats.available, icon: CheckCircle2 },
                  { id: "occupied", label: "就餐中", count: stats.occupied, icon: TrendingUp },
                  { id: "hasReservation", label: "有预约", count: stats.withReservation, icon: Calendar },
                ].map((item) => (
                  <div
                    key={item.id}
                    onClick={() => setActiveTab(item.id)}
                    className={cn(
                      "flex items-center justify-between p-4 rounded-lg border transition-all cursor-pointer text-sm",
                      activeTab === item.id 
                        ? "border-primary bg-primary/5 ring-1 ring-primary font-semibold text-primary" 
                        : "border-transparent hover:bg-slate-50 text-slate-600 font-medium"
                    )}
                  >
                    <div className="flex items-center gap-2">
                      <item.icon className="size-4" />
                      {item.label}
                    </div>
                    <Badge variant={activeTab === item.id ? "default" : "secondary"} className="h-5 px-1.5 min-w-5 justify-center">
                      {item.count}
                    </Badge>
                  </div>
                ))}
              </div>
            </section>

            <section className="bg-white rounded-xl border shadow-sm p-4 space-y-4">
              <h3 className="text-sm font-semibold text-slate-900 border-l-4 border-primary pl-3">
                快速查找
              </h3>
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input 
                  placeholder="搜索桌号..." 
                  className="pl-9 bg-slate-50 border-slate-200"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                />
              </div>
            </section>

            {/* Today's Reservation Overview */}
            <section className="bg-white rounded-xl border shadow-sm p-4 space-y-4">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-semibold text-slate-900 border-l-4 border-primary pl-3">
                  今日预约概览
                </h3>
                <Badge variant="outline" className="text-[10px] h-5 bg-amber-50 text-amber-600 border-amber-200">
                  {recentReservations.length} 档
                </Badge>
              </div>
              
              <div className="space-y-2 max-h-80 overflow-y-auto pr-1 custom-scrollbar">
                {recentReservations.length > 0 ? (
                  recentReservations.map((res) => (
                    <div key={res.id} className="bg-slate-50/50 p-2.5 rounded-lg border border-slate-100 hover:border-primary/30 transition-all group">
                      <div className="flex justify-between items-start mb-1">
                        <span className="text-[13px] font-bold text-slate-800">{res.contact_name}</span>
                        <span className="text-[10px] font-black text-primary bg-primary/10 px-1.5 py-0.5 rounded uppercase">{res.reservation_time}</span>
                      </div>
                      <div className="flex justify-between items-center text-[10px] text-muted-foreground font-medium">
                        <span>{res.guest_count}人 · {res.table_no}桌</span>
                        <div className="flex items-center gap-1 group-hover:text-primary transition-colors">
                           <Phone className="size-2.5" /> {res.contact_phone}
                        </div>
                      </div>
                    </div>
                  ))
                ) : (
                  <div className="text-center py-8 opacity-40">
                    <Clock className="size-8 mx-auto mb-2 text-slate-300" />
                    <p className="text-[10px] font-bold italic">今日暂无预约记录</p>
                  </div>
                )}
              </div>
              {recentReservations.length > 5 && (
                <Button variant="ghost" className="w-full text-xs font-bold text-primary hover:bg-primary/5 h-8">
                  查看全部预约
                </Button>
              )}
            </section>
          </div>

          {/* Main Content Area */}
          <div className="flex-1 space-y-4">
            {/* View Switcher */}
            <div className="flex items-center justify-between bg-card p-2 rounded-xl border border-muted/50 shadow-sm">
              <div className="flex items-center gap-2 ml-2">
                 <h4 className="text-sm font-bold text-slate-700">显示模式</h4>
                 <div className="flex border rounded-lg overflow-hidden h-8">
                    <Button variant={viewMode === 'grid' ? 'secondary' : 'ghost'} size="sm" className="rounded-none px-3" onClick={() => setViewMode('grid')}><LayoutGrid className="size-3.5" /></Button>
                    <Button variant={viewMode === 'list' ? 'secondary' : 'ghost'} size="sm" className="rounded-none px-3" onClick={() => setViewMode('list')}><ListIcon className="size-3.5" /></Button>
                 </div>
              </div>
              <p className="text-xs text-muted-foreground mr-2">实时数据每 30s 自动刷新</p>
            </div>

            {loading && tables.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-20 text-muted-foreground">
                <RefreshCw className="h-8 w-8 animate-spin mb-4 text-primary" />
                <p className="font-medium">数据加载中...</p>
              </div>
            ) : filteredTables.length === 0 ? (
              <div className="bg-white rounded-xl border border-dashed p-20 flex flex-col items-center justify-center text-muted-foreground">
                <div className="w-16 h-16 bg-slate-50 rounded-full flex items-center justify-center mb-4">
                  <Armchair className="w-8 h-8 text-slate-300" />
                </div>
                <p>未找到匹配的桌位</p>
              </div>
            ) : (
              <div className={cn(
                viewMode === "grid" 
                  ? "grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4 gap-4"
                  : "space-y-2"
              )}>
                {filteredTables.map(table => {
                  const config = STATUS_CONFIG[table.status] || STATUS_CONFIG.available;
                  const isOccupied = table.status === 'occupied';
                  const isReserved = table.status === 'reserved';
                  const progress = table.kitchenProgress;

                  if (viewMode === "grid") {
                    return (
                      <div 
                        key={table.id}
                        onClick={() => handleShowDetail(table)}
                        className={cn(
                          "bg-white rounded-xl border shadow-sm transition-all group relative overflow-hidden flex flex-col cursor-pointer hover:shadow-md",
                          isOccupied ? "border-primary/30 ring-1 ring-primary/5" : "hover:border-primary/50"
                        )}
                      >
                        {/* Status Line */}
                        <div className={cn("h-1 w-full", config.color.replace('text-', 'bg-'))} />
                        
                        <div className="p-4 flex-1">
                          <div className="flex items-start justify-between">
                            <div>
                               <h3 className="text-2xl font-black tracking-tighter">{table.table_no}</h3>
                               <div className="flex items-center gap-1.5 mt-0.5 text-xs text-slate-400 font-medium">
                                  <Users className="size-3" /> {table.capacity} 人位
                               </div>
                            </div>
                            <Badge variant={config.badgeVariant} className={cn("text-[10px] h-5 px-1.5 font-bold", config.bgColor, config.color, "border-none")}>
                               {config.label}
                            </Badge>
                          </div>

                          <div className="mt-4 pt-4 border-t border-dashed min-h-20 flex flex-col justify-center">
                             {isOccupied ? (
                               <div className="space-y-3">
                                  <div className="flex justify-between text-[11px] font-medium">
                                     <span className="text-muted-foreground">出餐进度</span>
                                     <span className="text-primary">{progress?.ready}/{progress?.total}</span>
                                  </div>
                                  <Progress value={progress?.percentage} className="h-1.5" indicatorClassName="bg-primary" />
                                  <div className="flex items-center justify-between mt-2">
                                     <span className="text-lg font-bold text-primary">
                                        <span className="text-xs font-normal">¥</span>
                                        {formatAmount(table.activeOrder?.total_amount)}
                                     </span>
                                     <div className="flex items-center text-[10px] text-muted-foreground">
                                        <Clock className="size-3 mr-1" />
                                        {table.activeOrder && new Date(table.activeOrder.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                                     </div>
                                  </div>
                               </div>
                             ) : isReserved ? (
                               <div className="bg-amber-50/50 p-3 rounded-lg border border-amber-100/50">
                                  <div className="flex items-center gap-1.5 text-amber-600 text-[11px] font-bold">
                                     <Clock className="size-3" /> {table.current_reservation?.reservation_time}
                                  </div>
                                   <p className="text-[12px] font-black mt-1 truncate">{table.current_reservation?.contact_name}</p>
                                   <p className="text-[10px] text-muted-foreground">{table.current_reservation?.guest_count}位 · {table.current_reservation?.contact_phone}</p>
                                </div>
                              ) : table.todayReservation ? (
                                <div className="bg-slate-50 p-3 rounded-lg border border-slate-100 relative overflow-hidden group/res">
                                   <div className="absolute top-0 right-0 p-1 opacity-10 group-hover/res:opacity-20 transition-opacity">
                                      <Clock className="size-10 -mr-4 -mt-4 rotate-12" />
                                   </div>
                                   <div className="flex items-center gap-1.5 text-slate-500 text-[11px] font-bold">
                                      <Calendar className="size-3" /> 今日 {table.todayReservation.reservation_time}
                                   </div>
                                   <p className="text-[12px] font-bold mt-1 truncate text-slate-700">{table.todayReservation.contact_name}</p>
                                   <p className="text-[10px] text-muted-foreground">{table.todayReservation.guest_count}人 · {table.todayReservation.contact_phone}</p>
                                </div>
                              ) : (
                               <div className="text-center opacity-20 py-2">
                                  <Armchair className="size-10 mx-auto" />
                                  <span className="text-[9px] font-bold uppercase tracking-widest mt-1 block">Ready to serve</span>
                               </div>
                             )}
                          </div>
                        </div>

                        {/* Actions Footer */}
                        <div className="p-3 bg-slate-50 border-t flex gap-2">
                           {isOccupied ? (
                              <Button 
                                variant="outline" 
                                size="sm" 
                                className="flex-1 h-8 text-[11px] font-bold rounded-lg border-primary/20 text-primary hover:bg-primary/5"
                                onClick={(e) => { e.stopPropagation(); handleCloseTable(table); }}
                              >
                                 <Receipt className="size-3 mr-1.5" /> 结账
                              </Button>
                           ) : (isReserved || table.todayReservation) && isReservationCheckInReady((table.current_reservation || table.todayReservation)?.reservation_time || "") ? (
                              <Button 
                                variant="default" 
                                size="sm" 
                                className="flex-1 h-8 text-[11px] font-bold rounded-lg"
                                disabled={actionLoading === table.id}
                                onClick={(e) => { e.stopPropagation(); handleCheckinReservation(table); }}
                              >
                                 <UserCheck className="size-3 mr-1.5" /> 到店签到
                              </Button>
                           ) : null}
                           {isOccupied && (
                              <Button 
                                variant="ghost" 
                                size="sm" 
                                className="h-8 px-3 text-[11px] font-medium text-amber-600 hover:bg-amber-50 hover:text-amber-700 rounded-lg"
                                onClick={(e) => { e.stopPropagation(); setSelectedTable(table); setIsTransferModalOpen(true); }}
                              >
                                 <ArrowRightLeft className="size-3 mr-1" /> 换桌
                              </Button>
                           )}
                           <Button 
                             variant="ghost" 
                             size="sm" 
                             className={cn(
                               "h-8 px-3 text-[11px] font-medium rounded-lg",
                               isOccupied || ((isReserved || table.todayReservation) && isReservationCheckInReady((table.current_reservation || table.todayReservation)?.reservation_time || ""))
                                 ? "text-slate-400 hover:bg-slate-100 hover:text-slate-600" 
                                 : "flex-1 text-slate-500 hover:bg-slate-100 hover:text-slate-700"
                             )}
                             onClick={(e) => { e.stopPropagation(); handleResetTable(table); }}
                           >
                             <Sparkles className="size-3 mr-1" /> 清扫
                           </Button>
                        </div>
                      </div>
                    );
                  }

                  // List View Item
                  return (
                    <div 
                      key={table.id}
                      onClick={() => handleShowDetail(table)}
                      className={cn(
                        "group p-4 rounded-xl border transition-all cursor-pointer bg-white hover:shadow-md",
                        isOccupied ? "border-primary/30 bg-primary/5" : "border-slate-100 hover:border-primary/50 hover:bg-slate-50"
                      )}
                    >
                      <div className="flex items-center gap-6">
                        {/* Table Info */}
                        <div className="w-16 text-center shrink-0">
                           <h5 className="text-2xl font-black tracking-tighter">{table.table_no}</h5>
                           <div className="text-[10px] text-muted-foreground mt-1 flex items-center justify-center gap-1">
                             <Users className="size-3" /> {table.capacity}
                           </div>
                        </div>

                        {/* Status Badge */}
                        <div className="w-20 shrink-0 text-center">
                          <Badge variant={config.badgeVariant} className={cn("text-[10px] h-6 px-2.5 font-bold shadow-sm", config.bgColor, config.color, "border-none")}>
                             {config.label}
                          </Badge>
                        </div>

                        {/* Main Content Area */}
                        <div className="flex-1 min-w-0">
                           {isOccupied ? (
                             <div className="flex gap-8">
                               {/* Order Progress & Amount */}
                               <div className="w-40 shrink-0 space-y-2">
                                  <div className="flex flex-col">
                                     <span className="text-xs font-medium text-slate-500 mb-1">当前消费</span>
                                     <span className="text-xl font-black text-primary font-mono tracking-tight">
                                       <span className="text-xs mr-0.5">¥</span>{formatAmount(table.activeOrder?.total_amount)}
                                     </span>
                                  </div>
                                  <div className="space-y-1">
                                     <div className="flex justify-between text-[10px] font-bold text-slate-500">
                                        <span>出餐进度</span>
                                        <span className={cn(progress?.percentage === 100 ? "text-emerald-600" : "text-primary")}>
                                          {progress?.ready}/{progress?.total}
                                        </span>
                                     </div>
                                     <Progress value={progress?.percentage} className="h-1.5" indicatorClassName={cn(progress?.percentage === 100 ? "bg-emerald-500" : "bg-primary")} />
                                  </div>
                               </div>
                               
                               {/* Order Items Preview */}
                               <div className="flex-1 min-w-0 border-l border-dashed border-primary/20 pl-6">
                                  <span className="text-[10px] font-bold text-slate-400 uppercase tracking-widest mb-1.5 block">
                                    菜品明细 ({table.activeOrder?.items?.length || 0})
                                  </span>
                                  <div className="flex flex-wrap gap-2">
                                    {(table.activeOrder?.items || []).slice(0, 4).map((item, idx) => (
                                      <Badge key={idx} variant="secondary" className="bg-white border-slate-200 text-slate-600 font-normal h-6">
                                        {item.quantity}x {item.name}
                                      </Badge>
                                    ))}
                                    {(table.activeOrder?.items?.length || 0) > 4 && (
                                      <Badge variant="outline" className="border-dashed text-muted-foreground h-6">
                                        +{table.activeOrder!.items!.length - 4} 更多
                                      </Badge>
                                    )}
                                    {(!table.activeOrder?.items || table.activeOrder.items.length === 0) && (
                                      <span className="text-xs text-slate-400 italic">暂无已点菜品</span>
                                    )}
                                  </div>
                               </div>
                             </div>
                            ) : isReserved ? (
                              <div className="flex items-center gap-6 h-full">
                                <div className="bg-amber-50 rounded-lg p-3 border border-amber-100 flex items-center gap-3">
                                  <div className="bg-white p-2 rounded-md shadow-sm">
                                    <Clock className="size-4 text-amber-600" />
                                  </div>
                                  <div>
                                    <p className="text-xs font-bold text-amber-900 line-clamp-1">{table.current_reservation?.contact_name}</p>
                                    <p className="text-[10px] text-amber-600 font-medium mt-0.5 flex items-center gap-2">
                                      <span>{table.current_reservation?.reservation_time} 到访</span>
                                      <span className="w-px h-2 bg-amber-200" />
                                      <span className="flex items-center gap-1"><Phone className="size-2.5" /> {table.current_reservation?.contact_phone}</span>
                                    </p>
                                  </div>
                                </div>
                                {table.current_reservation?.guest_count && (
                                  <div className="flex flex-col">
                                    <span className="text-[10px] text-slate-400 font-bold uppercase">预订人数</span>
                                    <span className="text-sm font-bold text-slate-700">{table.current_reservation.guest_count} 人</span>
                                  </div>
                                )}
                              </div>
                            ) : table.todayReservation ? (
                              <div className="flex items-center gap-6 h-full">
                                <div className="bg-slate-50 rounded-lg p-3 border border-slate-100 flex items-center gap-3 opacity-80">
                                  <div className="bg-white p-2 rounded-md shadow-sm">
                                    <Calendar className="size-4 text-slate-500" />
                                  </div>
                                  <div>
                                    <p className="text-xs font-bold text-slate-700 line-clamp-1">{table.todayReservation.contact_name}</p>
                                    <p className="text-[10px] text-slate-500 font-medium mt-0.5 flex items-center gap-2">
                                      <span>今晚 {table.todayReservation.reservation_time}</span>
                                      <span className="w-px h-2 bg-slate-200" />
                                      <span className="flex items-center gap-1"><Phone className="size-2.5" /> {table.todayReservation.contact_phone}</span>
                                    </p>
                                  </div>
                                </div>
                              </div>
                            ) : (
                             <div className="h-full flex items-center opacity-30">
                                <span className="text-xs font-bold uppercase tracking-widest flex items-center gap-2">
                                  <Armchair className="size-4" /> 暂无客人
                                </span>
                             </div>
                           )}
                        </div>

                        {/* Actions */}
                        <div className="flex items-center gap-2 border-l pl-6 py-2">
                          {isOccupied ? (
                             <>
                               <Button 
                                 variant="default" 
                                 size="sm" 
                                 className="h-9 px-4 text-xs font-bold rounded-lg shadow-sm shadow-primary/20" 
                                 onClick={(e) => { e.stopPropagation(); handleCloseTable(table); }}
                               >
                                 <Receipt className="size-3.5 mr-1.5" /> 结账退台
                               </Button>
                               <Button 
                                 variant="outline" 
                                 size="sm" 
                                 className="h-9 w-9 p-0 rounded-lg text-amber-600 hover:bg-amber-50 border-amber-200" 
                                 title="换桌"
                                 onClick={(e) => { e.stopPropagation(); setSelectedTable(table); setIsTransferModalOpen(true); }}
                               >
                                  <ArrowRightLeft className="size-4" />
                               </Button>
                             </>
                          ) : (isReserved || table.todayReservation) && isReservationCheckInReady((table.current_reservation || table.todayReservation)?.reservation_time || "") ? (
                             <Button 
                               variant="default" 
                               size="sm" 
                               className="h-9 px-4 text-xs font-bold rounded-lg bg-emerald-600 hover:bg-emerald-700 shadow-sm shadow-emerald-600/20"
                               disabled={actionLoading === table.id}
                               onClick={(e) => { e.stopPropagation(); handleCheckinReservation(table); }}
                             >
                               <UserCheck className="size-3.5 mr-1.5" /> 到店签到
                             </Button>
                          ) : null}
                          
                          <Button 
                            variant="ghost" 
                            size="sm" 
                            className="h-9 w-9 p-0 rounded-lg text-slate-400 hover:bg-slate-100" 
                            title="清扫/重置"
                            onClick={(e) => { e.stopPropagation(); handleResetTable(table); }}
                          >
                            <Sparkles className="size-4" />
                          </Button>
                          
                          <div className="h-4 w-px bg-slate-200 mx-1" />
                          
                          <div className="h-8 w-8 rounded-full bg-slate-50 flex items-center justify-center group-hover:bg-primary group-hover:text-white transition-colors">
                            <ChevronRight className="size-4" />
                          </div>
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      </PageContent>

      {/* Confirmation Actions */}
      <ConfirmDialog 
        open={confirmConfig.open}
        onOpenChange={(open) => setConfirmConfig(prev => ({ ...prev, open }))}
        title={
          confirmConfig.type === 'close' ? "结账退台确认" : "清扫确认"
        }
        description={
          confirmConfig.type === 'close'
            ? `确定要为 ${confirmConfig.table?.table_no} 桌进行结账吗？结账后系统将清空当前账单并将桌态设为空闲。`
            : `确定要将 ${confirmConfig.table?.table_no} 桌标记为已清扫吗？系统将重置桌态为空闲，准备迎接下一桌客人。`
        }
        confirmText={
          confirmConfig.type === 'close' ? "确认结账" : "确认清扫"
        }
        variant={confirmConfig.type === 'close' ? "destructive" : "default"}
        onConfirm={executeAction}
      />

      {/* Transfer Table Dialog */}
      <Dialog open={isTransferModalOpen} onOpenChange={setIsTransferModalOpen}>
        <DialogContent className="sm:max-w-md rounded-xl p-0 overflow-hidden border-none shadow-xl">
           <DialogHeader className="sr-only">
              <DialogTitle>转台服务</DialogTitle>
              <DialogDescription>将当前账单与客人状态转移至新桌位</DialogDescription>
           </DialogHeader>
           <div className="bg-slate-900 px-6 py-8 text-white">
              <h3 className="text-xl font-bold flex items-center gap-2">
                 <ArrowRightLeft className="size-5 text-amber-400" /> 转台服务
              </h3>
              <p className="text-slate-400 text-xs mt-1">将当前账单与客人状态转移至新桌位</p>
           </div>
           <div className="p-6 space-y-6">
              <div className="space-y-3">
                 <h4 className="text-xs font-bold text-slate-500 uppercase tracking-widest pl-1 border-l-4 border-primary">选择空闲目标</h4>
                 <div className="grid grid-cols-4 gap-2">
                    {tables.filter(t => t.status === 'available').map(t => (
                       <div 
                         key={t.id}
                         onClick={() => setTransferTargetId(t.id)}
                         className={cn(
                           "h-10 rounded-lg border-2 flex items-center justify-center font-bold text-sm cursor-pointer transition-all",
                           transferTargetId === t.id 
                             ? "border-primary bg-primary text-white shadow-lg shadow-primary/20 scale-105" 
                             : "border-slate-100 bg-slate-50 text-slate-500 hover:border-primary/50"
                         )}
                       >
                         {t.table_no}
                       </div>
                    ))}
                 </div>
              </div>
              <div className="bg-slate-50/80 p-4 rounded-xl border border-dashed text-center flex items-center justify-center gap-4">
                 <div className="text-center">
                    <p className="text-[9px] uppercase font-bold text-slate-400">Current</p>
                    <p className="font-bold">{selectedTable?.table_no}</p>
                 </div>
                 <ChevronRight className="size-4 text-slate-300" />
                 <div className="text-center">
                    <p className="text-[9px] uppercase font-bold text-primary">Target</p>
                    <p className="font-bold text-primary">{transferTargetId ? tables.find(t => t.id === transferTargetId)?.table_no : "--"}</p>
                 </div>
              </div>
           </div>
           <DialogFooter className="p-6 pt-0 gap-2">
              <Button variant="outline" className="flex-1 rounded-lg font-bold" onClick={() => setIsTransferModalOpen(false)}>取消退出</Button>
              <Button className="flex-1 rounded-lg font-bold" disabled={!transferTargetId} onClick={executeTransfer}>执行搬移</Button>
           </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Details Side-Dialog */}
      <Dialog open={isDetailModalOpen} onOpenChange={setIsDetailModalOpen}>
        <DialogContent className="sm:max-w-3xl rounded-xl p-0 overflow-hidden border-none shadow-2xl h-[85vh] flex flex-col">
          <DialogHeader className="sr-only">
            <DialogTitle>桌台详情 - {selectedTable?.table_no}</DialogTitle>
            <DialogDescription>查看当前桌台的订单详情、出餐进度及操作志</DialogDescription>
          </DialogHeader>
          <div className="flex flex-1 overflow-hidden">
             {/* Left Info Panel */}
             <div className="w-72 bg-slate-50 border-r p-6 flex flex-col space-y-6">
                <div className="space-y-4">
                   <div className="flex items-center gap-3">
                      <div className="size-12 rounded-xl bg-white border shadow-sm flex items-center justify-center text-xl font-black">
                         {selectedTable?.table_no}
                      </div>
                      <div>
                         <h4 className="font-bold">桌台详情</h4>
                         <p className="text-[10px] text-muted-foreground uppercase font-bold tracking-widest">
                           {selectedTable?.status === 'occupied' ? '用餐中' : selectedTable?.status === 'reserved' ? '待入座' : '空闲'}
                         </p>
                      </div>
                   </div>
                   <div className="space-y-3">
                      <div className="flex flex-col gap-1 p-3 bg-white rounded-lg border shadow-sm">
                         <span className="text-[9px] font-black text-slate-400 uppercase tracking-widest">用餐时长</span>
                         <span className="text-sm font-bold flex items-center gap-1.5">
                           <Clock className="size-3.5 text-primary" />
                           {selectedTable?.activeOrder?.created_at ? (
                             (() => {
                               const minutes = Math.floor((Date.now() - new Date(selectedTable.activeOrder.created_at).getTime()) / 60000);
                               return minutes < 60 ? `约 ${minutes} 分钟` : `约 ${Math.floor(minutes / 60)} 小时 ${minutes % 60} 分钟`;
                             })()
                           ) : '暂无订单'}
                         </span>
                      </div>
                      <div className="flex flex-col gap-1 p-3 bg-white rounded-lg border shadow-sm">
                         <span className="text-[9px] font-black text-slate-400 uppercase tracking-widest">累计消费</span>
                         <span className="text-xl font-black text-primary">
                           {selectedTable?.activeOrder ? `¥${formatAmount(selectedTable.activeOrder.total_amount)}` : '¥0.00'}
                         </span>
                      </div>
                      {selectedTable?.activeOrder && (
                        <div className="flex flex-col gap-1 p-3 bg-white rounded-lg border shadow-sm">
                           <span className="text-[9px] font-black text-slate-400 uppercase tracking-widest">订单状态</span>
                           <Badge className={cn(
                             "w-fit h-5 text-[10px] font-bold",
                             selectedTable.activeOrder.status === 'ready' ? "bg-emerald-500" : 
                             selectedTable.activeOrder.status === 'preparing' ? "bg-amber-500" : "bg-primary"
                           )}>
                             {selectedTable.activeOrder.status === 'ready' ? '待取餐' : 
                              selectedTable.activeOrder.status === 'preparing' ? '制作中' : '已支付'}
                           </Badge>
                        </div>
                      )}
                   </div>
                </div>

                <div className="flex-1" />

                <div className="space-y-2">
                   {selectedTable?.status === 'occupied' && (
                     <>
                       <Button variant="outline" className="w-full h-10 rounded-lg font-bold gap-2 text-xs" onClick={() => { setIsTransferModalOpen(true); setIsDetailModalOpen(false); }}>
                          <ArrowRightLeft className="size-3" />
                          转台/换座
                       </Button>
                       <Button className="w-full h-10 rounded-lg font-bold gap-2 text-xs" onClick={() => { handleCloseTable(selectedTable!); setIsDetailModalOpen(false); }}>
                          <Receipt className="size-3" />
                          结账并释放
                       </Button>
                     </>
                   )}
                   {(selectedTable?.status === 'reserved' || selectedTable?.todayReservation) && 
                    isReservationCheckInReady((selectedTable?.current_reservation || selectedTable?.todayReservation)?.reservation_time || "") && (
                     <Button className="w-full h-10 rounded-lg font-bold gap-2 text-xs" onClick={() => { handleCheckinReservation(selectedTable!); setIsDetailModalOpen(false); }}>
                        <UserCheck className="size-3" />
                        到店签到
                     </Button>
                   )}
                   <Button variant="ghost" className="w-full h-10 rounded-lg font-medium gap-2 text-xs text-slate-500" onClick={() => { handleResetTable(selectedTable!); setIsDetailModalOpen(false); }}>
                      <Sparkles className="size-3" />
                      清扫桌台
                   </Button>
                </div>
             </div>

             {/* Right Progress/List */}
             <div className="flex-1 flex flex-col p-6 overflow-hidden bg-white">
                <section className="mb-6 space-y-2">
                   <h3 className="text-sm font-semibold text-slate-900 border-l-4 border-primary pl-3">
                      订单详情
                   </h3>
                   <p className="text-xs text-muted-foreground pl-4">
                     {selectedTable?.activeOrder ? `共 ${selectedTable.activeOrder.items?.length || 0} 个菜品` : '暂无订单'}
                   </p>
                </section>

                <ScrollArea className="flex-1 pr-4">
                   <div className="space-y-4">
                      {selectedTable?.activeOrder?.items?.map((item, idx) => (
                        <div key={idx} className="flex gap-4 p-3 rounded-xl border border-slate-100 hover:border-primary/20 transition-all bg-slate-50/50">
                           <div className="size-16 rounded-lg bg-slate-200 overflow-hidden shrink-0 border border-white">
                              {item.image_url ? (
                                <Image src={item.image_url} width={40} height={40} className="size-full object-cover" alt={item.name} />
                              ) : (
                                <div className="size-full flex items-center justify-center bg-slate-100 text-slate-300"><ChefHat className="size-8" /></div>
                              )}
                           </div>
                           <div className="flex-1">
                              <div className="flex justify-between">
                                 <h6 className="font-bold text-sm text-slate-800">{item.name}</h6>
                                 <Badge variant="secondary" className="h-5 text-[10px] bg-slate-200">x{item.quantity}</Badge>
                              </div>
                              <div className="mt-1 text-xs text-muted-foreground">
                                 ¥{formatAmount(item.unit_price)} / 份
                              </div>
                              <div className="mt-2 flex items-center gap-3">
                                 <Badge className={cn(
                                   "h-4 text-[9px] font-black rounded-full px-2 tracking-widest border-none",
                                   selectedTable.activeOrder?.status === 'ready' ? "bg-emerald-500" : "bg-primary"
                                 )}>
                                   {selectedTable.activeOrder?.status === 'ready' ? "已出餐" : "制作中"}
                                 </Badge>
                                 <span className="text-[10px] text-muted-foreground font-medium flex items-center">
                                    <Clock className="size-2.5 mr-1" />
                                    {selectedTable.activeOrder?.created_at ? 
                                      new Date(selectedTable.activeOrder.created_at).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) + ' 下单'
                                      : ''}
                                 </span>
                              </div>
                           </div>
                        </div>
                      ))}
                      {!selectedTable?.activeOrder?.items?.length && (
                        <div className="h-full flex flex-col items-center justify-center opacity-30 py-20">
                           <ChefHat className="size-12 mb-2" />
                           <p className="text-sm font-black italic">暂无订单</p>
                        </div>
                      )}
                   </div>
                </ScrollArea>

                {selectedTable?.activeOrder && (
                  <div className="mt-6 p-4 bg-primary/5 rounded-xl border border-primary/10 flex items-center justify-between">
                     <div className="flex items-center gap-3">
                        <div className="size-9 rounded-full bg-white flex items-center justify-center border shadow-sm"><Receipt className="size-4 text-primary" /></div>
                        <div>
                           <p className="text-[10px] font-black text-muted-foreground uppercase leading-none">订单总计</p>
                           <p className="text-lg font-black text-primary mt-0.5">¥{formatAmount(selectedTable.activeOrder.total_amount)}</p>
                        </div>
                     </div>
                     <div className="text-right text-xs text-muted-foreground">
                        <p>{selectedTable.activeOrder.items?.reduce((sum, item) => sum + item.quantity, 0) || 0} 件商品</p>
                        <p className="mt-0.5">订单号: #{selectedTable.activeOrder.id}</p>
                     </div>
                  </div>
                )}
             </div>
          </div>
        </DialogContent>
      </Dialog>
    </PageShell>
  );
}
