"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { apiDelete } from "@/lib/api";

export function TableDeleteButton({ tableId }: { tableId: number | string }) {
  const [loading, setLoading] = useState(false);

  const remove = async () => {
    setLoading(true);
    try {
      await apiDelete(`/tables/${tableId}`);

      window.location.href = "/merchant/tables";
    } catch {
      window.alert("删除失败，请稍后重试");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Button variant="destructive" onClick={remove} disabled={loading}>
      删除桌台
    </Button>
  );
}
