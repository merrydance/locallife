"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiPost } from "@/lib/api";

export function DiningSessionActions() {
  const [loading, setLoading] = useState<string | null>(null);
  const [precheckTableId, setPrecheckTableId] = useState("");
  const [transferSessionId, setTransferSessionId] = useState("");
  const [transferTableId, setTransferTableId] = useState("");
  const [reason, setReason] = useState("");

  const call = async (path: string, payload: Record<string, unknown>) => {
    setLoading(path);
    try {
      await apiPost(path, payload);

      window.location.reload();
    } catch {
      toast.error("操作失败，请稍后重试");
    } finally {
      setLoading(null);
    }
  };

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <p className="text-sm text-muted-foreground">开台预检</p>
        <div className="flex flex-wrap gap-2">
          <Input
            placeholder="桌台 ID"
            value={precheckTableId}
            onChange={(event) => setPrecheckTableId(event.target.value)}
            className="w-32"
          />
          <Button
            size="sm"
            disabled={!precheckTableId || !!loading}
            onClick={() =>
              call("/dining-sessions/precheck", {
                table_id: Number(precheckTableId),
              })
            }
          >
            预检
          </Button>
        </div>
      </div>

      {/* 开台功能已移除 - 开台只能由用户扫码完成 */}

      <div className="space-y-2">
        <p className="text-sm text-muted-foreground">转台</p>
        <div className="flex flex-wrap gap-2">
          <Input
            placeholder="会话 ID"
            value={transferSessionId}
            onChange={(event) => setTransferSessionId(event.target.value)}
            className="w-32"
          />
          <Input
            placeholder="目标桌台 ID"
            value={transferTableId}
            onChange={(event) => setTransferTableId(event.target.value)}
            className="w-32"
          />
          <Input
            placeholder="原因（可选）"
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            className="min-w-50"
          />
          <Button
            size="sm"
            disabled={!transferSessionId || !transferTableId || !!loading}
            onClick={() =>
              call(`/dining-sessions/${transferSessionId}/transfer-table`, {
                to_table_id: Number(transferTableId),
                reason: reason || undefined,
              })
            }
          >
            转台
          </Button>
        </div>
      </div>
    </div>
  );
}
