"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { apiPost } from "@/lib/api";

type Props = {
  orderId: number | string;
  status?: string;
};

export function OrderActions({ orderId, status }: Props) {
  const [loading, setLoading] = useState<string | null>(null);
  const [reason, setReason] = useState("");

  const run = async (action: string, payload?: Record<string, unknown>) => {
    setLoading(action);
    try {
      await apiPost(`/merchant/orders/${orderId}/${action}`, payload);

      window.location.reload();
    } catch {
      window.alert("操作失败，请稍后重试");
    } finally {
      setLoading(null);
    }
  };

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap gap-2">
        <Button
          onClick={() => run("accept")}
          disabled={loading !== null || status === "accepted"}
        >
          接单
        </Button>
        <Button
          variant="outline"
          onClick={() => run("ready")}
          disabled={loading !== null}
        >
          出餐
        </Button>
        <Button
          variant="outline"
          onClick={() => run("complete")}
          disabled={loading !== null}
        >
          完成
        </Button>
      </div>
      <div className="space-y-2">
        <label className="text-sm text-muted-foreground">拒单原因</label>
        <textarea
          className="min-h-20 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
          placeholder="填写拒单原因"
          value={reason}
          onChange={(event) => setReason(event.target.value)}
        />
        <Button
          variant="destructive"
          onClick={() => run("reject", { reason })}
          disabled={loading !== null || reason.trim().length === 0}
        >
          拒单
        </Button>
      </div>
    </div>
  );
}
