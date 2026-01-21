"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { apiPost } from "@/lib/api";

type Props = {
  orderId: number | string;
  status?: string;
};

export function KitchenActions({ orderId, status }: Props) {
  const [loading, setLoading] = useState<string | null>(null);

  const run = async (action: "preparing" | "ready") => {
    setLoading(action);
    try {
      await apiPost(`/kitchen/orders/${orderId}/${action}`);

      window.location.reload();
    } catch {
      window.alert("操作失败，请稍后重试");
    } finally {
      setLoading(null);
    }
  };

  return (
    <div className="flex gap-2">
      <Button
        size="sm"
        onClick={() => run("preparing")}
        disabled={loading !== null || status === "preparing"}
      >
        开始制作
      </Button>
      <Button
        size="sm"
        variant="outline"
        onClick={() => run("ready")}
        disabled={loading !== null || status === "ready"}
      >
        出餐
      </Button>
    </div>
  );
}
