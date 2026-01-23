"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useState } from "react";
import {
  ChevronLeft,
  ChevronRight,
  LayoutDashboard,
  LogOut,
  Store,
  ShoppingBag,
  ChefHat,
  UtensilsCrossed,
  TrendingUp,
  Wallet,
  Package,
  Settings,
  Star,
  Users,
  Tag,
  Armchair,
  CalendarCheck,
  UserCog,
  Boxes,
  Building2,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { useMerchantSession } from "@/components/providers/merchant-session-provider";
import { cn } from "@/lib/utils";

// 导航分组
const navGroups = [
  {
    label: "日常运营",
    description: "每天开门使用",
    items: [
      {
        label: "工作台",
        href: "/merchant/dashboard",
        activePrefix: "/merchant/dashboard",
        icon: LayoutDashboard,
      },
      {
        label: "订单管理",
        href: "/merchant/orders",
        activePrefix: "/merchant/orders",
        icon: ShoppingBag,
      },
      {
        label: "后厨看板",
        href: "/merchant/kds",
        activePrefix: "/merchant/kds",
        icon: ChefHat,
      },
      {
        label: "堂食点餐",
        href: "/merchant/dinein",
        activePrefix: "/merchant/dinein",
        icon: UtensilsCrossed,
      },
      {
        label: "预订管理",
        href: "/merchant/reservations",
        activePrefix: "/merchant/reservations",
        icon: CalendarCheck,
      },
    ],
  },
  {
    label: "数据与财务",
    description: "经营分析和财务",
    items: [
      {
        label: "经营分析",
        href: "/merchant/analytics",
        activePrefix: "/merchant/analytics",
        icon: TrendingUp,
      },
      {
        label: "财务管理",
        href: "/merchant/finance",
        activePrefix: "/merchant/finance",
        icon: Wallet,
      },
      {
        label: "评价管理",
        href: "/merchant/reviews",
        activePrefix: "/merchant/reviews",
        icon: Star,
      },
    ],
  },
  {
    label: "商品配置",
    description: "菜品、套餐和库存",
    items: [
      {
        label: "桌台管理",
        href: "/merchant/tables",
        activePrefix: "/merchant/tables",
        icon: Armchair,
      },
      {
        label: "菜品管理",
        href: "/merchant/dishes",
        activePrefix: "/merchant/dishes",
        icon: Package,
      },
      {
        label: "套餐管理",
        href: "/merchant/combos",
        activePrefix: "/merchant/combos",
        icon: Boxes,
      },
      {
        label: "库存管理",
        href: "/merchant/inventory",
        activePrefix: "/merchant/inventory",
        icon: Package,
      },
    ],
  },
  {
    label: "营销与会员",
    description: "促销和客户管理",
    items: [
      {
        label: "营销活动",
        href: "/merchant/marketing",
        activePrefix: "/merchant/marketing",
        icon: Tag,
      },
      {
        label: "会员管理",
        href: "/merchant/members",
        activePrefix: "/merchant/members",
        icon: Users,
      },
    ],
  },
  {
    label: "集团协同",
    description: "多门店管理",
    shouldShow: (session: any) => !!session?.merchant?.group_id,
    items: [
      {
        label: "集团中心",
        href: "/merchant/group",
        activePrefix: "/merchant/group",
        icon: Building2,
      },
    ],
  },
  {
    label: "店铺配置",
    description: "基础设置",
    items: [
      {
        label: "员工管理",
        href: "/merchant/staff",
        activePrefix: "/merchant/staff",
        icon: UserCog,
      },
      {
        label: "店铺设置",
        href: "/merchant/settings",
        activePrefix: "/merchant/settings",
        icon: Settings,
      },
    ],
  },
];

export function MerchantSidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const [collapsed, setCollapsed] = useState(false);
  const [logoutDialogOpen, setLogoutDialogOpen] = useState(false);
  const session = useMerchantSession();

  const handleLogoutClick = () => {
    setLogoutDialogOpen(true);
  };

  const confirmLogout = () => {
    session?.logout();
    router.replace("/merchant/login");
  };

  // 商户名称：优先使用API返回的名称，否则显示加载中或默认值
  const merchantName = session?.merchant?.name || (session?.isReady ? "我的店铺" : null);

  return (
    <aside
      className={cn(
        "hidden flex-col border-r bg-card transition-all duration-300 lg:flex",
        collapsed ? "w-16" : "w-64"
      )}
    >
      {/* 顶部：店铺名称 */}
      <div className="flex h-16 items-center px-4 border-b">
        {!collapsed && (
          <div className="flex items-center gap-2 overflow-hidden flex-1 min-w-0">
            <div className="flex size-9 items-center justify-center rounded-lg bg-primary text-primary-foreground shrink-0">
              <Store className="size-5" />
            </div>
            <div className="flex flex-col min-w-0">
              {merchantName ? (
                <span className="truncate text-sm font-semibold text-slate-900">
                  {merchantName}
                </span>
              ) : (
                <Skeleton className="h-4 w-24" />
              )}
              {session?.isReady && (
                <span className="text-[10px] text-muted-foreground">
                  商户后台
                </span>
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

      {/* 营业状态 */}
      <div className="px-3 py-2">
        {!collapsed && session?.isReady && (
          <div className="px-2">
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

      {/* 导航分组 */}
      <nav className="flex-1 overflow-y-auto px-3 py-1">
        {navGroups.map((group, groupIndex) => {
          if ((group as any).shouldShow && !(group as any).shouldShow(session)) {
            return null;
          }
          return (
            <div key={group.label} className={cn(groupIndex > 0 && "mt-4")}>
              {/* 分组标题 */}
              {!collapsed && (
                <div className="px-2 mb-1">
                  <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
                    {group.label}
                  </span>
                </div>
              )}
              {collapsed && groupIndex > 0 && (
                <Separator className="my-2" />
              )}

              {/* 分组内的导航项 */}
              <div className="space-y-0.5">
                {group.items.map((item) => {
                  const active = pathname.startsWith(item.activePrefix);
                  const Icon = item.icon;
                  return (
                    <Link key={item.label} href={item.href} title={item.label}>
                      <span
                        className={cn(
                          "group flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                          active
                            ? "bg-primary/10 text-primary"
                            : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                          collapsed && "justify-center px-2"
                        )}
                      >
                        <Icon className={cn("size-4 shrink-0", active && "text-primary")} />
                        {!collapsed && (
                          <span className="truncate">{item.label}</span>
                        )}
                      </span>
                    </Link>
                  );
                })}
              </div>
            </div>
          );
        })}
      </nav>

      {/* 底部：退出登录 */}
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

      {/* 提示信息 */}
      {!collapsed && (
        <div className="p-4 pt-0">
          <div className="rounded-lg bg-muted/50 p-3 text-[11px] leading-relaxed text-muted-foreground">
            数据更新可能有延迟，请以实时接口返回为准。
          </div>
        </div>
      )}

      <ConfirmDialog
        open={logoutDialogOpen}
        onOpenChange={setLogoutDialogOpen}
        title="退出登录"
        description="确认要退出登录吗？"
        confirmText="退出"
        variant="destructive"
        onConfirm={confirmLogout}
      />
    </aside>
  );
}
