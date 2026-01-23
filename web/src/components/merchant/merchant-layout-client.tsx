"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import { MerchantSidebar } from "@/components/merchant/sidebar";
import { useMerchantSession } from "@/components/providers/merchant-session-provider";

export function MerchantLayoutClient({
  children,
}: {
  children: React.ReactNode;
}) {
  const pathname = usePathname();
  const router = useRouter();
  const session = useMerchantSession();
  const isLogin = pathname?.startsWith("/merchant/login");

  useEffect(() => {
    if (isLogin) return;
    if (session?.isReady && !session.isAuthenticated) {
      router.replace("/merchant/login");
    }
  }, [isLogin, router, session?.isAuthenticated, session?.isReady]);

  if (isLogin) {
    return <div className="min-h-screen bg-background text-foreground">{children}</div>;
  }

  if (!session?.isReady) {
    return (
      <div className="min-h-screen bg-background text-foreground">
        <div className="flex min-h-screen items-center justify-center text-sm text-muted-foreground">
          正在加载登录状态...
        </div>
      </div>
    );
  }

  if (!session.isAuthenticated) {
    return null;
  }

  return (
    <div className="min-h-screen bg-background text-foreground overflow-x-hidden">
      <div className="flex min-h-screen">
        <MerchantSidebar />
        <div className="flex flex-1 flex-col min-w-0">
          {children}
        </div>
      </div>
    </div>
  );
}
