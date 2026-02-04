"use client";

import { PlatformSidebar } from "@/components/platform/sidebar";

export function PlatformLayoutClient({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="min-h-screen bg-background text-foreground overflow-x-hidden">
      <div className="flex min-h-screen">
        <PlatformSidebar />
        <div className="flex flex-1 flex-col min-w-0">{children}</div>
      </div>
    </div>
  );
}
