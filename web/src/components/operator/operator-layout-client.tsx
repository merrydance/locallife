"use client";

import { useEffect } from "react";
import { usePathname } from "next/navigation";
import { OperatorSidebar } from "@/components/operator/sidebar";
import { setLastPortal } from "@/lib/role-portals";

export function OperatorLayoutClient({
  children,
}: {
  children: React.ReactNode;
}) {
  const pathname = usePathname();

  useEffect(() => {
    if (pathname?.startsWith("/operator")) {
      setLastPortal("operator");
    }
  }, [pathname]);

  return (
    <div className="min-h-screen bg-background text-foreground overflow-x-hidden">
      <div className="flex min-h-screen">
        <OperatorSidebar />
        <div className="flex flex-1 flex-col min-w-0">{children}</div>
      </div>
    </div>
  );
}
