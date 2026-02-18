"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  BarChart3,
  Building2,
  CircleDollarSign,
  LayoutDashboard,
  PanelsTopLeft,
  ShieldAlert,
  ScrollText,
  ShieldCheck,
  Store,
  Timer,
  Users,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { Separator } from "@/components/ui/separator";
import { useOperatorSession } from "@/components/providers/operator-session-provider";
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
  }>;
}> = [
  {
    label: "控制台",
    description: "运营商总览",
    items: [
      {
        label: "总览",
        href: "/operator",
        activePrefix: "/operator",
        exact: true,
        icon: LayoutDashboard,
      },
    ],
  },
  {
    label: "规则与治理",
    description: "规则与时段系数",
    items: [
      {
        label: "规则配置",
        href: "/operator/rules",
        activePrefix: "/operator/rules",
        exact: true,
        icon: ScrollText,
      },
      {
        label: "时段系数",
        href: "/operator/peak-hours",
        activePrefix: "/operator/peak-hours",
        exact: true,
        icon: Timer,
      },
    ],
  },
  {
    label: "运营监控",
    description: "商户与骑手排行",
    items: [
      {
        label: "商户排行",
        href: "/operator/merchants",
        activePrefix: "/operator/merchants",
        exact: true,
        icon: Users,
      },
      {
        label: "商户管理",
        href: "/operator/merchants/manage",
        activePrefix: "/operator/merchants/manage",
        exact: true,
        icon: Store,
      },
      {
        label: "骑手排行",
        href: "/operator/riders",
        activePrefix: "/operator/riders",
        exact: true,
        icon: ShieldCheck,
      },
      {
        label: "骑手管理",
        href: "/operator/riders/manage",
        activePrefix: "/operator/riders/manage",
        exact: true,
        icon: Users,
      },
    ],
  },
  {
    label: "区域趋势",
    description: "GMV 与订单走势",
    items: [
      {
        label: "日趋势",
        href: "/operator/regions",
        activePrefix: "/operator/regions",
        exact: true,
        icon: BarChart3,
      },
    ],
  },
  {
    label: "治理与风控",
    description: "申诉与食安处理",
    items: [
      {
        label: "申诉处理",
        href: "/operator/appeals",
        activePrefix: "/operator/appeals",
        exact: true,
        icon: ShieldCheck,
      },
      {
        label: "食安事件",
        href: "/operator/safety",
        activePrefix: "/operator/safety",
        exact: true,
        icon: ShieldAlert,
      },
    ],
  },
  {
    label: "财务中心",
    description: "佣金与提现",
    items: [
      {
        label: "微信开户",
        href: "/operator/applyment",
        activePrefix: "/operator/applyment",
        exact: true,
        icon: Building2,
      },
      {
        label: "财务管理",
        href: "/operator/finance",
        activePrefix: "/operator/finance",
        exact: true,
        icon: CircleDollarSign,
      },
    ],
  },
];

export function OperatorSidebar() {
  const pathname = usePathname();
  const session = useOperatorSession();
  const rolePortals = buildConsolePortals(session?.roles ?? []).filter((portal) => portal.key !== "operator");

  const portalIconMap: Record<ConsolePortalKey, LucideIcon> = {
    merchant: Store,
    operator: ShieldCheck,
    platform: PanelsTopLeft,
  };

  return (
    <aside className="hidden w-64 border-r bg-card/50 lg:flex lg:flex-col">
      <div className="px-6 py-6">
        <div className="text-lg font-semibold">运营商控制台</div>
        <div className="text-xs text-muted-foreground">区域规则与运营看板</div>
      </div>
      <Separator />
      <nav className="flex-1 space-y-6 overflow-y-auto px-4 py-6">
        {navGroups.map((group) => (
          <div key={group.label} className="space-y-3">
            <div className="space-y-1 px-2">
              <div className="text-xs font-semibold uppercase text-muted-foreground">
                {group.label}
              </div>
              <div className="text-[11px] text-muted-foreground">
                {group.description}
              </div>
            </div>
            <div className="space-y-1">
              {group.items.map((item) => {
                const active = item.exact
                  ? pathname === item.href
                  : pathname?.startsWith(item.activePrefix);
                const Icon = item.icon;
                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    className={cn(
                      "flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition",
                      active
                        ? "bg-primary/10 text-primary"
                        : "text-muted-foreground hover:text-foreground hover:bg-muted/40"
                    )}
                  >
                    <Icon className="h-4 w-4" />
                    <span>{item.label}</span>
                  </Link>
                );
              })}
            </div>
          </div>
        ))}

        {rolePortals.length > 0 && (
          <div className="space-y-3">
            <div className="space-y-1 px-2">
              <div className="text-xs font-semibold uppercase text-muted-foreground">
                角色入口
              </div>
              <div className="text-[11px] text-muted-foreground">按权限显示可访问控制台</div>
            </div>
            <div className="space-y-1">
              {rolePortals.map((portal) => {
                const active = pathname === portal.href || pathname?.startsWith(`${portal.activePrefix}/`);
                const Icon = portalIconMap[portal.key];
                return (
                  <Link
                    key={portal.key}
                    href={portal.href}
                    prefetch={false}
                    className={cn(
                      "flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition",
                      active
                        ? "bg-primary/10 text-primary"
                        : "text-muted-foreground hover:text-foreground hover:bg-muted/40"
                    )}
                  >
                    <Icon className="h-4 w-4" />
                    <span>{portal.label}</span>
                  </Link>
                );
              })}
            </div>
          </div>
        )}
      </nav>
    </aside>
  );
}
