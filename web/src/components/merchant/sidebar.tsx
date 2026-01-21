"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import { Badge } from "@/components/ui/badge";

const navItems = [
  { label: "工作台", href: "/merchant/dashboard", activePrefix: "/merchant/dashboard" },
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
  const [collapsed, setCollapsed] = useState(false);

  return (
    <aside
      className={`hidden flex-col border-r bg-card py-6 transition-all duration-200 lg:flex ${
        collapsed ? "w-16 px-2" : "w-60 px-4"
      }`}
    >
      <div className="mb-6 flex items-center justify-between">
        <div className={`text-lg font-semibold ${collapsed ? "sr-only" : ""}`}>
          本地生活商户
        </div>
        <Badge className={`bg-primary text-primary-foreground ${collapsed ? "sr-only" : ""}`}>
          营业中
        </Badge>
        <button
          type="button"
          className="ml-auto rounded-md border px-2 py-1 text-xs text-muted-foreground hover:bg-muted"
          onClick={() => setCollapsed((prev) => !prev)}
          aria-label={collapsed ? "展开导航" : "折叠导航"}
        >
          {collapsed ? "»" : "«"}
        </button>
      </div>
      <nav className="flex flex-1 flex-col gap-1">
        {navItems.map((item) => {
          const active = pathname.startsWith(item.activePrefix);
          return (
            <Link
              key={item.label}
              href={item.href}
              className={`flex items-center rounded-md px-3 py-2 text-sm transition-colors ${
                active
                  ? "bg-accent text-accent-foreground"
                  : "text-muted-foreground hover:bg-muted"
              }`}
              title={item.label}
            >
              <span className={collapsed ? "text-xs" : ""}>
                {collapsed ? item.label.slice(0, 1) : item.label}
              </span>
            </Link>
          );
        })}
      </nav>
      <div
        className={`mt-6 rounded-md border bg-muted/40 p-3 text-xs text-muted-foreground ${
          collapsed ? "hidden" : "block"
        }`}
      >
        数据以实时接口为准
      </div>
    </aside>
  );
}
