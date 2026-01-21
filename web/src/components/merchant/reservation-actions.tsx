"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { apiPost } from "@/lib/api";

type Props = {
  reservationId: number | string;
};

export function ReservationActions({ reservationId }: Props) {
  const [loading, setLoading] = useState<string | null>(null);

  const run = async (action: string, payload?: Record<string, unknown>) => {
    setLoading(action);
    try {
      await apiPost(`/reservations/${reservationId}/${action}`, payload);

      window.location.reload();
    } catch {
      window.alert("操作失败，请稍后重试");
    } finally {
      setLoading(null);
    }
  };

  return (
    <div className="flex flex-wrap gap-2">
      <Button size="sm" onClick={() => run("confirm")} disabled={!!loading}>
        确认
      </Button>
      <Button size="sm" variant="outline" onClick={() => run("checkin")} disabled={!!loading}>
        到店签到
      </Button>
      <Button size="sm" variant="outline" onClick={() => run("start-cooking")} disabled={!!loading}>
        起菜通知
      </Button>
      <Button size="sm" variant="outline" onClick={() => run("complete")} disabled={!!loading}>
        完成
      </Button>
      <Button size="sm" variant="destructive" onClick={() => run("no-show")} disabled={!!loading}>
        爽约
      </Button>
    </div>
  );
}
