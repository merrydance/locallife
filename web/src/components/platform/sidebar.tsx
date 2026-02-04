"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  Activity,
  BarChart3,
  ClipboardList,
  FileSearch,
  LayoutDashboard,
  ShieldAlert,
  Workflow,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { Separator } from "@/components/ui/separator";
import { usePlatformSession } from "@/components/providers/platform-session-provider";
import { cn } from "@/lib/utils";

const navGroups: Array<{
  label: string;
  description: string;
  items: Array<{
    label: string;
    href: string;
    activePrefix: string;
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
        icon: LayoutDashboard,
        requiredRoles: ["admin", "operator"],
      },
    ],
  },
  {
    label: "风控与审计",
    description: "异常裁决与审计",
    items: [
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
        label: "分账复核",
        href: "/platform/reconciliation",
        activePrefix: "/platform/reconciliation",
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
        href: "/platform/rules#hits",
        activePrefix: "/platform/rules",
        icon: Activity,
        requiredRoles: ["admin"],
      },
    ],
  },
];

export function PlatformSidebar() {
  const pathname = usePathname();
  const session = usePlatformSession();
  const roles = session?.roles ?? [];

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
    <aside className="hidden w-64 border-r bg-card/50 lg:flex lg:flex-col">
      <div className="px-6 py-6">
        <div className="text-lg font-semibold">平台控制台</div>
        <div className="text-xs text-muted-foreground">角色权限由平台配置</div>
      </div>
      <Separator />
      <nav className="flex-1 space-y-6 overflow-y-auto px-4 py-6">
        {filteredGroups.map((group) => (
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
                const active = pathname?.startsWith(item.activePrefix);
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
      </nav>
    </aside>
  );
}
