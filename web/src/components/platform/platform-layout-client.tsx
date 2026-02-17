"use client";

import { useEffect } from "react";
import { usePathname } from "next/navigation";
import { PlatformSidebar } from "@/components/platform/sidebar";
import { setLastPortal } from "@/lib/role-portals";

export function PlatformLayoutClient({
  children,
}: {
  children: React.ReactNode;
}) {
  const pathname = usePathname();

  useEffect(() => {
    if (pathname?.startsWith("/platform")) {
      setLastPortal("platform");
    }
  }, [pathname]);

  return (
    <div className="min-h-screen bg-background text-foreground overflow-x-hidden">
      <div className="flex min-h-screen">
        <PlatformSidebar />
        <div className="flex flex-1 flex-col min-w-0">{children}</div>
      </div>
    </div>
  );
}
