"use client";

import { useEffect, useState } from "react";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Phone, User, Clock, MapPin, BellRing, ChevronRight } from "lucide-react";
import { Separator } from "@/components/ui/separator";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";

interface NoShowData {
  reservation_id: number;
  table_no: string;
  arrival_time: string;
  reservation_date: string;
  contact_name: string;
  contact_phone: string;
}

export function RealtimeNotificationHandler() {
  const [noShowAlert, setNoShowAlert] = useState<NoShowData | null>(null);

  useEffect(() => {
    const handleMessage = (event: Event) => {
      const customEvent = event as CustomEvent;
      const message = customEvent.detail;

      if (!message) return;

      switch (message.type) {
        case "reservation_no_show_alert":
          setNoShowAlert(message.data as NoShowData);
          // Also show a toast as backup
          toast.error("预订超时提醒", {
            description: `预订 ${message.data.table_no} 的客人尚未到店`,
          });
          break;
        case "new_order":
          toast.success("🏪 新订单提醒", {
            description: "您有一笔新的订单，请及时处理",
          });
          break;
        default:
          break;
      }
    };

    window.addEventListener("merchant-realtime", handleMessage);
    return () => window.removeEventListener("merchant-realtime", handleMessage);
  }, []);

  return (
    <>
      <AlertDialog open={!!noShowAlert} onOpenChange={(open) => !open && setNoShowAlert(null)}>
        <AlertDialogContent className="max-w-md border-2 border-amber-100 shadow-2xl">
          <AlertDialogHeader>
            <div className="flex items-center gap-3 mb-2">
              <div className="bg-amber-100 p-2.5 rounded-full">
                <BellRing className="h-6 w-6 text-amber-600 animate-pulse" />
              </div>
              <AlertDialogTitle className="text-xl font-bold text-slate-800 tracking-tight">
                预订超时未到店提醒
              </AlertDialogTitle>
            </div>
            <AlertDialogDescription className="text-slate-500 font-medium">
              该预订已超过规定到店时间 30 分钟，请及时联系客确认。
            </AlertDialogDescription>
          </AlertDialogHeader>

          {noShowAlert && (
            <div className="my-6 space-y-5">
              <div className="flex items-center justify-between p-4 bg-slate-50 rounded-2xl border border-slate-100">
                <div className="space-y-1">
                  <span className="text-[10px] font-bold text-slate-400 uppercase tracking-widest block font-sans">预订桌台</span>
                  <div className="flex items-center gap-2 text-slate-900 font-extrabold text-xl">
                    <MapPin className="h-4 w-4 text-primary" />
                    {noShowAlert.table_no}
                  </div>
                </div>
                <div className="text-right space-y-1">
                  <span className="text-[10px] font-bold text-slate-400 uppercase tracking-widest block font-sans">预约时间</span>
                  <Badge variant="secondary" className="bg-white text-primary font-bold px-3 py-1 shadow-sm border-slate-100">
                    <Clock className="h-3 w-3 mr-1.5" />
                    {noShowAlert.arrival_time}
                  </Badge>
                </div>
              </div>

              <Separator className="bg-slate-100" />

              <div className="space-y-4 px-1">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="h-10 w-10 rounded-full bg-slate-100 flex items-center justify-center text-slate-500">
                      <User className="h-5 w-5" />
                    </div>
                    <div>
                      <p className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">预约客户</p>
                      <p className="text-sm font-bold text-slate-700">{noShowAlert.contact_name}</p>
                    </div>
                  </div>
                </div>

                <Button variant="outline" className="w-full h-14 justify-between group border-2 hover:border-primary hover:bg-primary/5 rounded-xl transition-all" asChild>
                  <a href={`tel:${noShowAlert.contact_phone}`}>
                    <div className="flex items-center gap-3">
                      <div className="h-8 w-8 rounded-lg bg-primary/10 flex items-center justify-center">
                        <Phone className="h-4 w-4 text-primary" />
                      </div>
                      <div className="text-left">
                        <p className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">拨打客户电话</p>
                        <p className="text-lg font-black text-slate-900 leading-none">{noShowAlert.contact_phone}</p>
                      </div>
                    </div>
                    <ChevronRight className="h-5 w-5 text-slate-300 group-hover:text-primary transition-colors" />
                  </a>
                </Button>
              </div>
            </div>
          )}

          <AlertDialogFooter className="sm:justify-center">
            <AlertDialogAction 
              onClick={() => setNoShowAlert(null)}
              className="w-full h-12 text-base font-bold shadow-lg shadow-primary/20 rounded-xl"
            >
              我知道了，暂不联系
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
