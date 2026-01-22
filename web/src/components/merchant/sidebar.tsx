"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useState } from "react";
import { ChevronLeft, ChevronRight, LayoutDashboard, LogOut, Store } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { useMerchantSession } from "@/components/providers/merchant-session-provider";
import { cn } from "@/lib/utils";

const navItems = [
  { label: "工作台", href: "/merchant/dashboard", activePrefix: "/merchant/dashboard", icon: LayoutDashboard },
  {
    label: "经营分析",
    href: "/merchant/analytics/dashboard?tab=overview",
    activePrefix: "/merchant/analytics",
  },
  { label: "订单", href: "/merchant/orders", activePrefix: "/merchant/orders" },
  { label: "菜品", href: "/merchant/dishes", activePrefix: "/merchant/dishes" },
  { label: "桌台", href: "/merchant/tables", activePrefix: "/merchant/tables" },
  { label: "堂食", href: "/merchant/dinein", activePrefix: "/merchant/dinein" },
  { label: "后厨", href: "/merchant/kds", activePrefix: "/merchant/kds" },
  { label: "财务", href: "/merchant/finance", activePrefix: "/merchant/finance" },
  { label: "营销", href: "/merchant/marketing", activePrefix: "/merchant/marketing" },
  { label: "评价", href: "/merchant/reviews", activePrefix: "/merchant/reviews" },
  { label: "预订", href: "/merchant/reservations", activePrefix: "/merchant/reservations" },
  { label: "店铺设置", href: "/merchant/settings", activePrefix: "/merchant/settings" },
];

export function MerchantSidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const [collapsed, setCollapsed] = useState(false);
  const session = useMerchantSession();

  const handleLogout = () => {
    if (!window.confirm("确认退出登录？")) return;
    session?.logout();
    router.replace("/merchant/login");
  };

  return (
    <aside
      className={cn(
        "hidden flex-col border-r bg-card transition-all duration-300 lg:flex",
        collapsed ? "w-16" : "w-64"
      )}
    >
      <div className="flex h-16 items-center px-4">
        {!collapsed && (
          <div className="flex items-center gap-2 overflow-hidden">
            <div className="flex size-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
              <Store className="size-5" />
            </div>
            <span className="truncate text-base font-semibold">本地生活商户</span>
          </div>
        )}
        <Button
          variant="ghost"
          size="icon"
          className={cn("ml-auto", collapsed && "mx-auto")}
          onClick={() => setCollapsed(!collapsed)}
        >
          {collapsed ? <ChevronRight /> : <ChevronLeft />}
        </Button>
      </div>

      <div className="px-3 py-2">
        {!collapsed && session?.isReady && (
          <div className="mb-2 px-4">
            {session.isOpen ? (
              <Badge
                variant="secondary"
                className="bg-emerald-50 text-emerald-700 hover:bg-emerald-50 dark:bg-emerald-500/10 dark:text-emerald-400"
              >
                ● 营业中
              </Badge>
            ) : (
              <Badge
                variant="secondary"
                className="bg-rose-50 text-rose-700 hover:bg-rose-50 dark:bg-rose-500/10 dark:text-rose-400"
              >
                ● 已打烊
              </Badge>
            )}
          </div>
        )}
      </div>

      <nav className="flex-1 space-y-1 px-3 py-2">
        {navItems.map((item) => {
          const active = pathname.startsWith(item.activePrefix);
          return (
            <Link key={item.label} href={item.href} title={item.label}>
              <span
                className={cn(
                  "group flex items-center rounded-md px-3 py-2 text-sm font-medium transition-colors",
                  active
                    ? "bg-primary/10 text-primary"
                    : "text-muted-foreground hover:bg-accent hover:text-accent-foreground"
                )}
              >
                <span className={cn("flex-1 truncate", collapsed && "text-center")}>
                  {collapsed ? item.label.slice(0, 1) : item.label}
                </span>
              </span>
            </Link>
          );
        })}
      </nav>

      <div className="mt-auto p-4">
        <Separator className="mb-4" />
        <Button
          variant="ghost"
          size={collapsed ? "icon" : "default"}
          className={cn(
            "w-full justify-start text-muted-foreground hover:bg-destructive/10 hover:text-destructive",
            collapsed && "justify-center"
          )}
          onClick={handleLogout}
        >
          <LogOut className={cn("size-4", !collapsed && "mr-2")} />
          {!collapsed && <span>退出登录</span>}
        </Button>
      </div>

      {!collapsed && (
        <div className="p-4 pt-0">
          <div className="rounded-lg bg-muted/50 p-3 text-[11px] leading-relaxed text-muted-foreground">
            数据更新可能有延迟，请以实时接口返回为准。
          </div>
        </div>
      )}
    </aside>
  );
}
