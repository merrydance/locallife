"use client";

import { useEffect, useMemo, useState, useCallback, useRef } from "react";
import { 
  RefreshCw, 
  Volume2, 
  VolumeX, 
  LogOut, 
  Clock, 
  ChefHat, 
  Timer, 
  CheckCircle2, 
  Flame, 
  Bell,
  UtensilsCrossed,
  Package,
  Store,
  Info,
  X,
  CalendarDays,
  Wifi,
  WifiOff
} from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { 
  Dialog, 
  DialogContent, 
  DialogHeader, 
  DialogTitle,
  DialogDescription,
  DialogFooter
} from "@/components/ui/dialog";
import { PageShell, PageHeader, PageContent } from "@/components/merchant/layout/page-shell";
import { apiGet, apiPost } from "@/lib/api";
import { cn } from "@/lib/utils";
import type {
  KitchenOrderResponse,
  KitchenOrdersResponse,
} from "@/types/kitchen";
import { useMerchantSession } from "@/components/providers/merchant-session-provider";

const ORDER_TYPE_MAP: Record<string, { label: string; icon: any; color: string }> = {
  takeout: { label: "外卖", icon: Package, color: "text-blue-500 bg-blue-50 border-blue-100" },
  dine_in: { label: "堂食", icon: UtensilsCrossed, color: "text-emerald-500 bg-emerald-50 border-emerald-100" },
  takeaway: { label: "自取", icon: Store, color: "text-amber-500 bg-amber-50 border-amber-100" },
  reservation: { label: "预订", icon: CalendarDays, color: "text-purple-500 bg-purple-50 border-purple-100" },
};

function formatTime(dateStr?: string) {
  if (!dateStr) return "--:--";
  const d = new Date(dateStr);
  const h = String(d.getHours()).padStart(2, "0");
  const m = String(d.getMinutes()).padStart(2, "0");
  return `${h}:${m}`;
}

type Props = {
  initialData: KitchenOrdersResponse;
};

type ColumnKey = "new_orders" | "preparing_orders" | "ready_orders";

const COLUMNS: Array<{
  key: ColumnKey;
  title: string;
  icon: any;
  color: string;
  status: string;
}> = [
  { key: "new_orders", title: "待制作", icon: Bell, color: "bg-rose-500", status: "paid" },
  { key: "preparing_orders", title: "制作中", icon: ChefHat, color: "bg-amber-500", status: "preparing" },
  { key: "ready_orders", title: "待出餐", icon: CheckCircle2, color: "bg-emerald-500", status: "ready" },
];

export function KdsPageClient({ initialData }: Props) {
  const session = useMerchantSession();
  const [data, setData] = useState(initialData);
  const [loading, setLoading] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [voiceEnabled, setVoiceEnabled] = useState(true);
  const [currentTime, setCurrentTime] = useState("");
  const [selectedOrder, setSelectedOrder] = useState<KitchenOrderResponse | null>(null);
  const [exitConfirmOpen, setExitConfirmOpen] = useState(false);
  
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const prevNewCountRef = useRef(initialData.stats.new_count);
  const wakeLockRef = useRef<any>(null);

  // Settings Recovery
  useEffect(() => {
    const savedVoice = localStorage.getItem("kds_voice_enabled");
    if (savedVoice !== null) setVoiceEnabled(savedVoice === "true");
    
    const savedAuto = localStorage.getItem("kds_auto_refresh");
    if (savedAuto !== null) setAutoRefresh(savedAuto === "true");
  }, []);

  // Settings Sync
  useEffect(() => {
    localStorage.setItem("kds_voice_enabled", String(voiceEnabled));
  }, [voiceEnabled]);

  useEffect(() => {
    localStorage.setItem("kds_auto_refresh", String(autoRefresh));
  }, [autoRefresh]);

  // Screen Wake Lock (Industrial Grade)
  useEffect(() => {
    const requestWakeLock = async () => {
      if ('wakeLock' in navigator) {
        try {
          wakeLockRef.current = await (navigator as any).wakeLock.request('screen');
        } catch (err) {}
      }
    };
    requestWakeLock();
    return () => {
      wakeLockRef.current?.release().catch(() => {});
    };
  }, []);

  // Clock Update
  useEffect(() => {
    const updateTime = () => {
      setCurrentTime(new Date().toLocaleTimeString('zh-CN', { hour12: false }));
    };
    updateTime();
    const timer = setInterval(updateTime, 1000);
    return () => clearInterval(timer);
  }, []);

  // Audio System
  useEffect(() => {
    // Production ready: Using a reliable generated beep if external sound fails
    audioRef.current = new Audio("https://assets.mixkit.co/active_storage/sfx/2869/2869-preview.mp3");
  }, []);

  const refresh = useCallback(async (silent = false) => {
    if (!silent) setLoading(true);
    try {
      const next = await apiGet<KitchenOrdersResponse>("/kitchen/orders");
      
      // Industrial Alert Logic
      if (next.stats.new_count > prevNewCountRef.current && voiceEnabled) {
        audioRef.current?.play().catch(() => {
          // Fallback to console beep or browser notification if allowed
          console.log("New order sound alert triggered");
        });
      }
      prevNewCountRef.current = next.stats.new_count;
      setData(next);
    } catch {
      if (!silent) toast.error("数据链路同步失败，请检查网络");
    } finally {
      if (!silent) setLoading(false);
    }
  }, [voiceEnabled]);

  // Real-time Event Bridge
  useEffect(() => {
    const handleRealtime = (event: any) => {
      const detail = event.detail;
      // Listening for specific kitchen/order updates from WebSocket
      if (detail?.type === "new_order" || detail?.type === "order_update") {
        refresh(true);
      }
    };
    window.addEventListener("merchant-realtime", handleRealtime);
    return () => window.removeEventListener("merchant-realtime", handleRealtime);
  }, [refresh]);

  // Production Watchdog (Fallback Polling)
  useEffect(() => {
    if (!autoRefresh) return;
    const timer = setInterval(() => {
      refresh(true);
    }, 15000); // 15s watchdog
    return () => clearInterval(timer);
  }, [autoRefresh, refresh]);

  const runAction = async (orderId: number, action: "preparing" | "ready") => {
    try {
      await apiPost(`/kitchen/orders/${orderId}/${action}`);
      toast.success(action === "preparing" ? "开始制作" : "制作完成");
      refresh(true);
    } catch (err: any) {
      toast.error(err.message || "操作指令执行失败");
    }
  };

  const statusInfo = useMemo(() => {
    if (!session?.wsConnected) return { label: "离线 (自动重连中)", icon: WifiOff, color: "text-rose-500 bg-rose-50" };
    return { label: "数据链路正常", icon: Wifi, color: "text-emerald-500 bg-emerald-50 border-emerald-100" };
  }, [session?.wsConnected]);

  return (
    <PageShell className="h-screen bg-slate-50 overflow-hidden flex flex-col">
      <audio ref={audioRef} preload="auto" />
      
      <PageHeader
        className="bg-white border-b shadow-sm py-4 shrink-0"
        title="🍳 厨房显示系统 (KDS)"
        description={
          <div className="flex items-center gap-4 mt-1">
             <div className={cn("flex items-center gap-1.5 px-3 py-1 rounded-full text-[10px] font-bold border transition-all", statusInfo.color)}>
               <statusInfo.icon className={cn("h-3.5 w-3.5", session?.wsConnected ? "animate-none" : "animate-pulse")} />
               {statusInfo.label}
             </div>
             <span className="text-[11px] font-mono text-muted-foreground flex items-center gap-1.5 bg-slate-100 px-2 py-1 rounded-md">
               <Clock className="h-3.5 w-3.5" />
               {currentTime}
             </span>
          </div>
        }
        actions={
          <div className="flex items-center gap-2">
            <div className="hidden lg:flex items-center gap-5 mr-6 border-r pr-6">
               <div className="text-center group">
                 <div className="text-[10px] text-muted-foreground font-bold uppercase tracking-wider group-hover:text-primary transition-colors">今日已出</div>
                 <div className="text-lg font-black tabular-nums">{data.stats.completed_today_count}</div>
               </div>
               <div className="text-center group">
                 <div className="text-[10px] text-muted-foreground font-bold uppercase tracking-wider group-hover:text-amber-500 transition-colors">平均出餐</div>
                 <div className="text-lg font-black text-amber-600 tabular-nums">{data.stats.avg_prepare_time}<span className="text-[10px] font-normal ml-0.5">MIN</span></div>
               </div>
            </div>

            <Button 
              variant="outline" 
              size="sm" 
              className={cn("h-10 w-10 p-0 rounded-xl transition-all", voiceEnabled ? "text-primary border-primary/20 bg-primary/5 shadow-inner" : "text-muted-foreground")}
              onClick={() => setVoiceEnabled(!voiceEnabled)}
            >
              {voiceEnabled ? <Volume2 className="h-5 w-5" /> : <VolumeX className="h-5 w-5" />}
            </Button>

            <Button 
              variant="outline" 
              size="sm" 
              className={cn("h-10 w-10 p-0 rounded-xl transition-all", autoRefresh ? "text-primary border-primary/20 bg-primary/5 shadow-inner" : "text-muted-foreground")}
              onClick={() => setAutoRefresh(!autoRefresh)}
            >
              <RefreshCw className={cn("h-5 w-5", loading && "animate-spin")} />
            </Button>

            <Button 
              variant="destructive" 
              size="sm" 
              className="h-10 px-6 rounded-xl font-bold shadow-lg shadow-rose-500/10"
              onClick={() => setExitConfirmOpen(true)}
            >
              <LogOut className="h-4 w-4 mr-2" />
              退出系统
            </Button>
          </div>
        }
      />

      <PageContent className="p-4 flex-1 h-0 overflow-hidden space-y-0">
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 h-full">
          {COLUMNS.map((column) => (
            <div key={column.key} className="flex flex-col h-full bg-slate-200/40 rounded-2xl border border-slate-200 overflow-hidden shadow-sm">
              <div className="flex items-center justify-between p-4 bg-white/80 backdrop-blur-md border-b shrink-0">
                <div className="flex items-center gap-3">
                  <div className={cn("p-2 rounded-xl border shadow-sm", column.color.replace('bg-', 'bg-').concat('/10'), column.color.replace('bg-', 'text-'), column.color.replace('bg-', 'border-').concat('/20'))}>
                    <column.icon className="h-5 w-5" />
                  </div>
                  <h3 className="font-black text-slate-800 tracking-tight">{column.title}</h3>
                </div>
                <Badge variant="secondary" className="h-7 px-3 font-black bg-slate-100 text-slate-600 border border-slate-200 tabular-nums">
                  {data[column.key].length}
                </Badge>
              </div>

              <ScrollArea className="flex-1 p-3">
                <div className="space-y-4 pb-10">
                  {data[column.key].length === 0 ? (
                    <div className="flex flex-col items-center justify-center py-24 opacity-20 grayscale select-none">
                      <div className="h-20 w-20 bg-white rounded-3xl flex items-center justify-center mb-4 shadow-sm border rotate-3">
                        <column.icon className="h-10 w-10 text-slate-400" />
                      </div>
                      <p className="text-sm font-black tracking-widest uppercase">No Orders</p>
                    </div>
                  ) : (
                    data[column.key].map((order) => {
                      const typeInfo = ORDER_TYPE_MAP[order.order_type] || { label: order.order_type, icon: Info, color: "text-slate-500 bg-slate-50" };
                      return (
                        <Card 
                          key={order.id} 
                          className={cn(
                            "group cursor-pointer transition-all border-none shadow-[0_4px_12px_-4px_rgba(0,0,0,0.05)] hover:shadow-[0_8px_24px_-8px_rgba(0,0,0,0.12)] hover:ring-2 ring-primary/20 overflow-hidden relative bg-white",
                            order.is_urged && "ring-2 ring-rose-500 shadow-rose-500/20 bg-rose-50/10"
                          )}
                          onClick={() => setSelectedOrder(order)}
                        >
                          {order.is_urged && (
                             <div className="absolute top-0 right-0 p-2 bg-rose-500 text-white rounded-bl-xl z-10 animate-pulse">
                               <Flame className="h-4 w-4 fill-current" />
                             </div>
                          )}
                          
                          <div className="p-4 space-y-4">
                             <div className="flex items-center justify-between">
                               <div className="flex items-center gap-2.5">
                                 <span className="text-2xl font-black italic tracking-tighter text-slate-900 group-hover:text-primary transition-colors">
                                   #{order.order_no.slice(-4)}
                                 </span>
                                 <Badge variant="outline" className={cn("text-[10px] h-6 px-2 font-black border border-current/20", typeInfo.color)}>
                                   <typeInfo.icon className="h-3.5 w-3.5 mr-1.5" />
                                   {typeInfo.label.toUpperCase()}
                                 </Badge>
                               </div>
                               <div className="text-[11px] font-black text-slate-400 tabular-nums bg-slate-50 px-2 py-1 rounded border border-slate-100">
                                 {formatTime(order.paid_at || order.created_at)}
                               </div>
                             </div>

                             <div className="flex flex-wrap gap-2">
                               {order.table_no && (
                                 <div className="flex items-center gap-1.5 bg-slate-900 text-white px-2.5 py-1 rounded-lg w-fit shadow-sm">
                                   <UtensilsCrossed className="h-3.5 w-3.5 text-primary" />
                                   <span className="text-[11px] font-black tracking-tighter">TABLE #{order.table_no}</span>
                                 </div>
                               )}

                               {order.pickup_number && (
                                 <div className="flex items-center gap-1.5 bg-amber-500 text-white px-2.5 py-1 rounded-lg w-fit shadow-sm">
                                   <Info className="h-3.5 w-3.5" />
                                   <span className="text-[11px] font-black tracking-tighter">CODE: {order.pickup_number}</span>
                                 </div>
                               )}
                             </div>

                             <div className="space-y-3 border-t border-slate-100 pt-4">
                               {Object.entries(
                                 order.items.reduce((acc, item) => {
                                   const cat = item.category_name || "其他";
                                   if (!acc[cat]) acc[cat] = [];
                                   acc[cat].push(item);
                                   return acc;
                                 }, {} as Record<string, typeof order.items>)
                               ).map(([category, items], gIdx) => (
                                 <div key={`${order.id}-group-${gIdx}`} className="space-y-1.5">
                                   <div className="flex items-center gap-2 mb-1">
                                     <span className="text-[9px] font-black bg-slate-100 text-slate-400 px-1.5 py-0.5 rounded tracking-tighter uppercase">{category}</span>
                                     <div className="h-px bg-slate-100 flex-1" />
                                   </div>
                                   {items.map((item, idx) => (
                                     <div key={`${order.id}-item-${idx}`} className="flex items-start justify-between gap-4">
                                       <div className="text-[13px] font-bold text-slate-700 leading-tight">
                                         {item.name}
                                         {item.customizations && item.customizations.length > 0 && (
                                           <div className="text-[10px] text-muted-foreground font-medium mt-0.5 space-x-1">
                                             {item.customizations.map((c, ci) => (
                                               <span key={ci} className="bg-slate-100 px-1 py-0.5 rounded italic">
                                                 {c.value}
                                               </span>
                                             ))}
                                           </div>
                                         )}
                                       </div>
                                       <div className="text-base font-black text-slate-900 italic tabular-nums">×{item.quantity}</div>
                                     </div>
                                   ))}
                                 </div>
                               ))}
                             </div>

                             {order.notes && (
                               <div className="bg-amber-50/50 p-3 rounded-xl text-[11px] text-amber-900 font-medium italic flex items-start gap-2 border border-amber-100/50">
                                 <div className="mt-0.5 shrink-0 bg-amber-200 text-amber-700 rounded-md p-1">
                                   <Info className="h-3 w-3" />
                                 </div>
                                 <span className="leading-relaxed">"{order.notes}"</span>
                               </div>
                             )}

                             <div className="flex items-center justify-between pt-2 border-t border-slate-50">
                               <div className="flex items-center gap-4">
                                  <div className={cn("flex items-center gap-1.5 text-[11px] font-black tabular-nums transition-colors", order.waiting_minutes > 15 ? "text-rose-500" : "text-slate-500")}>
                                    <Timer className={cn("h-4 w-4", order.waiting_minutes > 15 && "animate-bounce")} />
                                    {order.waiting_minutes}m
                                  </div>
                                  {order.estimated_ready_at && (
                                    <div className="text-slate-400 font-bold text-[11px] flex items-center gap-1.5">
                                      <Clock className="h-3.5 w-3.5" />
                                      {formatTime(order.estimated_ready_at)}
                                    </div>
                                  )}
                               </div>

                               {column.key === "new_orders" && (
                                  <Button 
                                    size="sm" 
                                    className="h-10 px-6 rounded-xl font-black shadow-md active:scale-95 transition-transform"
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      runAction(order.id, "preparing");
                                    }}
                                  >
                                    START
                                  </Button>
                               )}

                               {column.key === "preparing_orders" && (
                                  <Button 
                                    size="sm" 
                                    variant="outline"
                                    className="h-10 px-6 rounded-xl border-2 border-primary font-black text-primary hover:bg-primary hover:text-white transition-all shadow-sm flex items-center gap-2"
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      runAction(order.id, "ready");
                                    }}
                                  >
                                    <CheckCircle2 className="h-4 w-4" />
                                    FINISH
                                  </Button>
                               )}

                               {column.key === "ready_orders" && (
                                  <div className="h-10 flex items-center font-black text-emerald-500 text-[10px] bg-emerald-50 px-3 rounded-xl border border-emerald-100">
                                    <CheckCircle2 className="h-4 w-4 mr-2" />
                                    READY FOR PICKUP
                                  </div>
                                )}
                             </div>
                          </div>
                        </Card>
                      );
                    })
                  )}
                </div>
              </ScrollArea>
            </div>
          ))}
        </div>
      </PageContent>

      <Dialog open={!!selectedOrder} onOpenChange={() => setSelectedOrder(null)}>
        <DialogContent className="max-w-md p-0 overflow-hidden border-none shadow-2xl rounded-3xl">
          {selectedOrder && (
            <>
              <DialogHeader className="p-8 bg-slate-900 text-white relative">
                <div className="flex items-center justify-between mb-4">
                   <Badge className="bg-primary/20 hover:bg-primary/20 text-primary-foreground border-primary/30 uppercase tracking-widest text-[10px] font-black">
                     Order Control
                   </Badge>
                   <Button 
                    variant="ghost" 
                    size="icon" 
                    className="h-10 w-10 text-white/50 hover:text-white hover:bg-white/10 rounded-full" 
                    onClick={() => setSelectedOrder(null)}
                   >
                     <X className="h-5 w-5" />
                   </Button>
                </div>
                <DialogTitle className="text-5xl font-black italic tracking-tighter leading-none mb-2">
                  #{selectedOrder.order_no.slice(-4)}
                </DialogTitle>
                <DialogDescription className="text-white/40 text-[11px] font-mono leading-none">
                  {selectedOrder.order_no}
                </DialogDescription>
              </DialogHeader>

              <div className="p-8 space-y-8">
                 <div className="grid grid-cols-2 gap-8">
                    <div className="space-y-1.5">
                      <div className="text-[10px] text-muted-foreground uppercase font-black tracking-widest">Type</div>
                      <div className="font-black text-lg flex items-center gap-2 italic">
                        {selectedOrder.order_type === "takeout" ? <Package className="h-5 w-5 text-blue-500" /> : 
                         selectedOrder.order_type === "dine_in" ? <UtensilsCrossed className="h-5 w-5 text-emerald-500" /> : 
                         selectedOrder.order_type === "takeaway" ? <Store className="h-5 w-5 text-amber-500" /> :
                         <CalendarDays className="h-5 w-5 text-purple-500" />}
                        {ORDER_TYPE_MAP[selectedOrder.order_type]?.label || selectedOrder.order_type}
                      </div>
                    </div>
                    {(selectedOrder.table_no || selectedOrder.pickup_number) && (
                      <div className="space-y-1.5 text-right">
                        <div className="text-[10px] text-muted-foreground uppercase font-black tracking-widest">
                          {selectedOrder.table_no ? "Table" : "Pickup Code"}
                        </div>
                        <div className={cn("text-3xl font-black font-mono italic leading-none", selectedOrder.table_no ? "text-primary" : "text-amber-500")}>
                          {selectedOrder.table_no ? `#${selectedOrder.table_no}` : selectedOrder.pickup_number}
                        </div>
                      </div>
                    )}
                 </div>

                 <Separator className="bg-slate-100" />

                 <div className="space-y-5">
                    <h4 className="font-black text-slate-800 flex items-center gap-2.5 uppercase tracking-tighter">
                      <ChefHat className="h-5 w-5 text-primary" />
                      Product List ({selectedOrder.items.length})
                    </h4>
                    <ScrollArea className="max-h-[35vh] pr-4">
                      <div className="space-y-8">
                        {Object.entries(
                          selectedOrder.items.reduce((acc, item) => {
                            const cat = item.category_name || "其他";
                            if (!acc[cat]) acc[cat] = [];
                            acc[cat].push(item);
                            return acc;
                          }, {} as Record<string, typeof selectedOrder.items>)
                        ).map(([category, items], gIdx) => (
                          <div key={`modal-group-${gIdx}`} className="space-y-4">
                            <div className="flex items-center gap-2">
                               <Badge className="bg-slate-100 hover:bg-slate-100 text-slate-400 border-none px-2 py-0.5 text-[9px] font-black tracking-widest uppercase">
                                 {category}
                               </Badge>
                               <div className="h-px bg-slate-100 flex-1" />
                            </div>
                            {items.map((item, idx) => (
                              <div key={`modal-item-${idx}`} className="flex items-start justify-between gap-6 group">
                                <div className="space-y-1.5">
                                  <div className="font-black text-slate-900 leading-tight group-hover:text-primary transition-colors text-lg italic tracking-tight">{item.name}</div>
                                  {item.customizations && item.customizations.length > 0 && (
                                    <div className="flex flex-wrap gap-2">
                                      {item.customizations.map((c, cidx) => (
                                        <Badge key={`c-${item.id}-${cidx}`} variant="secondary" className="text-[10px] font-bold h-5 px-2 bg-slate-100 text-slate-500 border-none italic">
                                          {c.name}: {c.value}
                                        </Badge>
                                      ))}
                                    </div>
                                  )}
                                </div>
                                <div className="font-black text-2xl text-slate-900 italic tracking-tighter shrink-0 ring-offset-4 ring-1 ring-slate-100 rounded-lg px-2">×{item.quantity}</div>
                              </div>
                            ))}
                          </div>
                        ))}
                      </div>
                    </ScrollArea>
                 </div>

                 {selectedOrder.notes && (
                   <div className="space-y-3">
                      <div className="text-[10px] text-muted-foreground uppercase font-black tracking-widest">System Message</div>
                      <div className="p-5 bg-amber-50 rounded-2xl text-sm text-amber-900 italic border border-amber-100 leading-relaxed font-bold shadow-inner">
                        "{selectedOrder.notes}"
                      </div>
                   </div>
                 )}
              </div>

              <DialogFooter className="p-6 bg-slate-900 border-t border-white/5 flex flex-row gap-3">
                {selectedOrder.status === "paid" && (
                  <Button 
                    className="flex-1 h-16 rounded-2xl font-black text-lg italic shadow-2xl hover:scale-[1.02] transition-transform"
                    onClick={() => {
                      runAction(selectedOrder.id, "preparing");
                      setSelectedOrder(null);
                    }}
                  >
                    START PRODUCTION
                    <ChefHat className="ml-3 h-5 w-5" />
                  </Button>
                )}
                {selectedOrder.status === "preparing" && (
                  <Button 
                    className="flex-1 h-16 rounded-2xl font-black text-lg italic shadow-2xl bg-emerald-600 hover:bg-emerald-700 hover:scale-[1.02] transition-transform"
                    onClick={() => {
                      runAction(selectedOrder.id, "ready");
                      setSelectedOrder(null);
                    }}
                  >
                    MARK AS READY
                    <CheckCircle2 className="ml-3 h-5 w-5" />
                  </Button>
                )}
                <Button 
                  variant="outline" 
                  className="px-8 h-16 rounded-2xl font-black border-white/20 text-white bg-transparent hover:bg-white/10 hover:text-white"
                  onClick={() => setSelectedOrder(null)}
                >
                  CLOSE
                </Button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>

      <Dialog open={exitConfirmOpen} onOpenChange={setExitConfirmOpen}>
        <DialogContent className="max-w-sm rounded-3xl border-none p-8">
          <DialogHeader className="space-y-4">
            <div className="h-14 w-14 bg-rose-50 rounded-2xl flex items-center justify-center text-rose-500 mb-2">
              <LogOut className="h-7 w-7" />
            </div>
            <DialogTitle className="text-2xl font-black tracking-tighter">系统的安全退出现</DialogTitle>
            <DialogDescription className="text-slate-500 font-bold leading-relaxed">
              确定要退出厨房显示系统吗？退出后您的 WebSocket 数据链路将断开，新订单将无法实时推送。
            </DialogDescription>
          </DialogHeader>
          <DialogFooter className="gap-3 mt-6">
            <Button variant="outline" className="flex-1 h-12 rounded-xl font-black border-slate-200" onClick={() => setExitConfirmOpen(false)}>
              保持挂载
            </Button>
            <Button variant="destructive" className="flex-1 h-12 rounded-xl font-black" onClick={() => window.location.href = "/merchant/dashboard"}>
              确认退出
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </PageShell>
  );
}
