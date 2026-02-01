"use client";

import { useEffect } from "react";
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
  useEffect(() => {
    const handleMessage = (event: Event) => {
      const customEvent = event as CustomEvent;
      const message = customEvent.detail;

      if (!message) return;

      switch (message.type) {
        case "reservation_no_show_alert":
          const noShow = message.data as NoShowData;
          toast(`预订超时未到提醒`, {
            description: `客人 ${noShow.contact_name} 未按时抵达 (桌号 ${noShow.table_no})\n电话: ${noShow.contact_phone}`,
            duration: Infinity,
            action: {
              label: "我已知晓",
              onClick: () => {}
            },
            cancel: {
              label: "拨打电话",
              onClick: () => window.location.href = `tel:${noShow.contact_phone}`
            }
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

  return null;
}
