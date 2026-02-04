"use client";

import { OperatorSidebar } from "@/components/operator/sidebar";

export function OperatorLayoutClient({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="min-h-screen bg-background text-foreground overflow-x-hidden">
      <div className="flex min-h-screen">
        <OperatorSidebar />
        <div className="flex flex-1 flex-col min-w-0">{children}</div>
      </div>
    </div>
  );
}
