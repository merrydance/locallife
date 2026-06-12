"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useSearchParams } from "next/navigation";
import { useState } from "react";
import {
  Activity,
  BarChart3,
  ChevronLeft,
  ChevronRight,
  ClipboardList,
  FileSearch,
  LayoutDashboard,
  Landmark,
  LogOut,
  PanelsTopLeft,
  Percent,
  RadioTower,
  ShieldAlert,
  ShieldCheck,
  Store,
  Users,
  Workflow,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { usePlatformSession } from "@/components/providers/platform-session-provider";
import { buildConsolePortals, type ConsolePortalKey } from "@/lib/role-portals";
import { cn } from "@/lib/utils";

const navGroups: Array<{
  label: string;
  description: string;
  items: Array<{
    label: string;
    href: string;
    activePrefix: string;
    exact?: boolean;
    icon: LucideIcon;
    requiredRoles?: string[];
  }>;
}> = [
  {
    label: "控制台",
    description: "平台总览",
    items: [
      {
        label: "总览",
        href: "/platform",
        activePrefix: "/platform",
        exact: true,
        icon: LayoutDashboard,
        requiredRoles: ["admin", "operator"],
      },
    ],
  },
  {
    label: "平台治理",
    description: "申请审核与组织管理",
    items: [
      {
        label: "运营商申请",
        href: "/platform/operators",
        activePrefix: "/platform/operators",
        exact: true,
        icon: Users,
        requiredRoles: ["admin"],
      },
      {
        label: "区域扩展申请",
        href: "/platform/operators/region-applications",
        activePrefix: "/platform/operators/region-applications",
        icon: PanelsTopLeft,
        requiredRoles: ["admin"],
      },
      {
        label: "集团申请",
        href: "/platform/groups",
        activePrefix: "/platform/groups",
        icon: Landmark,
        requiredRoles: ["admin"],
      },
    ],
  },
  {
    label: "风控与审计",
    description: "异常裁决与审计",
    items: [
      {
        label: "流量监控",
        href: "/platform/traffic",
        activePrefix: "/platform/traffic",
        exact: true,
        icon: RadioTower,
        requiredRoles: ["admin"],
      },
      {
        label: "风控审计",
        href: "/platform/audit",
        activePrefix: "/platform/audit",
        icon: ShieldAlert,
        requiredRoles: ["admin", "operator"],
      },
      {
        label: "异常裁决复核",
        href: "/platform/adjudication",
        activePrefix: "/platform/adjudication",
        icon: FileSearch,
        requiredRoles: ["admin", "operator"],
      },
    ],
  },
  {
    label: "分账与对账",
    description: "分账复核与对账",
    items: [
      {
        label: "分账比例配置",
        href: "/platform/profit-sharing",
        activePrefix: "/platform/profit-sharing",
        icon: Percent,
        requiredRoles: ["admin"],
      },
      {
        label: "分账复核",
        href: "/platform/reconciliation",
        activePrefix: "/platform/reconciliation",
        exact: true,
        icon: ClipboardList,
        requiredRoles: ["admin", "operator"],
      },
    ],
  },
  {
    label: "跨区县运营",
    description: "区域监控",
    items: [
      {
        label: "区域监控",
        href: "/platform/regions",
        activePrefix: "/platform/regions",
        icon: BarChart3,
        requiredRoles: ["admin", "operator"],
      },
    ],
  },
  {
    label: "规则与变更",
    description: "规则生效追踪",
    items: [
      {
        label: "规则变更可视化",
        href: "/platform/rules",
        activePrefix: "/platform/rules",
        icon: Workflow,
        requiredRoles: ["admin"],
      },
      {
        label: "命中趋势",
        href: "/platform/rules?view=hits",
        activePrefix: "/platform/rules",
        icon: Activity,
        requiredRoles: ["admin"],
      },
    ],
  },
];

export function PlatformSidebar() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const [collapsed, setCollapsed] = useState(false);
  const [logoutDialogOpen, setLogoutDialogOpen] = useState(false);
  const session = usePlatformSession();
  const roles = session?.roles ?? [];
  const rolePortals = buildConsolePortals(roles);

  const handleLogoutClick = () => {
    setLogoutDialogOpen(true);
  };

  const confirmLogout = () => {
    session?.logout();
    router.replace("/login");
  };

  const portalIconMap: Record<ConsolePortalKey, LucideIcon> = {
    merchant: Store,
    operator: ShieldCheck,
    platform: PanelsTopLeft,
  };

  const filteredGroups = navGroups
    .map((group) => {
      const items = group.items.filter((item) => {
        if (!item.requiredRoles || item.requiredRoles.length === 0) return true;
        return item.requiredRoles.some((role) => roles.includes(role));
      });
      return { ...group, items };
    })
    .filter((group) => group.items.length > 0);

  return (
    <aside
      className={cn(
        "hidden flex-col border-r bg-card transition-all duration-300 lg:flex",
        collapsed ? "w-16" : "w-64"
      )}
    >
      <div className="flex h-16 items-center px-4 border-b">
        {!collapsed && (
          <div className="flex items-center gap-2 overflow-hidden flex-1 min-w-0">
            <div className="flex size-9 items-center justify-center rounded-lg bg-primary text-primary-foreground shrink-0">
              <PanelsTopLeft className="size-5" />
            </div>
            <div className="flex flex-col min-w-0">
              {session?.user?.full_name ? (
                <span className="truncate text-sm font-semibold text-slate-900">
                  {session.user.full_name}
                </span>
              ) : session?.isReady ? (
                <span className="truncate text-sm font-semibold text-slate-900">平台管理员</span>
              ) : (
                <Skeleton className="h-4 w-24" />
              )}
              {session?.isReady && (
                <span className="text-[10px] text-muted-foreground">平台控制台</span>
              )}
            </div>
          </div>
        )}
        <Button
          variant="ghost"
          size="icon"
          className={cn("shrink-0", collapsed && "mx-auto")}
          onClick={() => setCollapsed(!collapsed)}
        >
          {collapsed ? <ChevronRight className="h-4 w-4" /> : <ChevronLeft className="h-4 w-4" />}
        </Button>
      </div>

      <div className="px-3 py-2">
        {!collapsed && session?.isReady && (
          <div className="px-2">
            <Badge variant="secondary" className="bg-sky-50 text-sky-700 hover:bg-sky-50">
              ● 管理模式
            </Badge>
          </div>
        )}
      </div>

      <nav className="flex-1 overflow-y-auto px-3 py-1">
        {filteredGroups.map((group) => (
          <div key={group.label} className="mt-4 first:mt-0">
            {!collapsed && (
              <div className="px-2 mb-1">
                <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
                  {group.label}
                </span>
              </div>
            )}
            {collapsed && <Separator className="my-2" />}

            <div className="space-y-0.5">
              {group.items.map((item) => {
                const isRulesGroup = item.activePrefix === "/platform/rules";
                const view = searchParams?.get("view") || "";
                const active = isRulesGroup
                  ? (item.label === "命中趋势"
                    ? pathname?.startsWith("/platform/rules") && view === "hits"
                    : pathname?.startsWith("/platform/rules") && view !== "hits")
                  : item.exact
                  ? pathname === item.activePrefix
                  : pathname?.startsWith(item.activePrefix);
                const Icon = item.icon;
                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    title={item.label}
                    className={cn(
                      "group flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                      active
                        ? "bg-primary/10 text-primary"
                        : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                      collapsed && "justify-center px-2"
                    )}
                  >
                    <Icon className={cn("size-4 shrink-0", active && "text-primary")} />
                    {!collapsed && <span className="truncate">{item.label}</span>}
                  </Link>
                );
              })}
            </div>
          </div>
        ))}

        {rolePortals.length > 0 && (
          <div className="mt-4">
            {!collapsed && (
              <div className="px-2 mb-1">
                <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
                  角色入口
                </span>
              </div>
            )}
            {collapsed && <Separator className="my-2" />}

            <div className="space-y-0.5">
              {rolePortals.map((portal) => {
                const active = pathname?.startsWith(portal.activePrefix);
                const Icon = portalIconMap[portal.key];
                return (
                  <Link
                    key={portal.key}
                    href={portal.href}
                    prefetch={false}
                    title={portal.label}
                    className={cn(
                      "group flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                      active
                        ? "bg-primary/10 text-primary"
                        : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                      collapsed && "justify-center px-2"
                    )}
                  >
                    <Icon className={cn("size-4 shrink-0", active && "text-primary")} />
                    {!collapsed && <span className="truncate">{portal.label}</span>}
                  </Link>
                );
              })}
            </div>
          </div>
        )}
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
          onClick={handleLogoutClick}
        >
          <LogOut className={cn("size-4", !collapsed && "mr-2")} />
          {!collapsed && <span>退出登录</span>}
        </Button>
      </div>

      {!collapsed && (
        <div className="p-4 pt-0">
          <div className="rounded-lg bg-muted/50 p-3 text-[11px] leading-relaxed text-muted-foreground">
            平台操作会记录审计日志，请确认后提交。
          </div>
        </div>
      )}

      <ConfirmDialog
        open={logoutDialogOpen}
        onOpenChange={setLogoutDialogOpen}
        title="退出登录"
        description="确认要退出平台控制台吗？"
        confirmText="退出"
        variant="destructive"
        onConfirm={confirmLogout}
      />
    </aside>
  );
}
