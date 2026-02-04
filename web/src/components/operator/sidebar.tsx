"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  BarChart3,
  LayoutDashboard,
  ScrollText,
  ShieldCheck,
  Users,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { Separator } from "@/components/ui/separator";
import { cn } from "@/lib/utils";

const navGroups: Array<{
  label: string;
  description: string;
  items: Array<{
    label: string;
    href: string;
    activePrefix: string;
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
        icon: LayoutDashboard,
      },
    ],
  },
  {
    label: "规则与治理",
    description: "区域规则配置",
    items: [
      {
        label: "规则配置",
        href: "/operator/rules",
        activePrefix: "/operator/rules",
        icon: ScrollText,
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
        icon: Users,
      },
      {
        label: "骑手排行",
        href: "/operator/riders",
        activePrefix: "/operator/riders",
        icon: ShieldCheck,
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
        icon: BarChart3,
      },
    ],
  },
];

export function OperatorSidebar() {
  const pathname = usePathname();

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
