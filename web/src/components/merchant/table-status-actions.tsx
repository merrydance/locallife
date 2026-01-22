"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { apiPatch } from "@/lib/api";

type Props = {
  tableId: number | string;
  currentStatus?: string;
};

const statusOptions = [
  { label: "空闲", value: "available" },
  { label: "占用", value: "occupied" },
  { label: "清洁中", value: "cleaning" },
  { label: "停用", value: "disabled" },
];

export function TableStatusActions({ tableId, currentStatus }: Props) {
  const [status, setStatus] = useState(currentStatus || "available");
  const [loading, setLoading] = useState(false);

  const updateStatus = async () => {
    setLoading(true);
    try {
      await apiPatch(`/tables/${tableId}/status`, { status });

      window.location.reload();
    } catch {
      toast.error("操作失败，请稍后重试");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex flex-wrap items-center gap-2">
      <select
        className="h-9 rounded-md border border-input bg-background px-2 text-sm"
        value={status}
        onChange={(event) => setStatus(event.target.value)}
      >
        {statusOptions.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
      <Button size="sm" onClick={updateStatus} disabled={loading}>
        更新状态
      </Button>
    </div>
  );
}
