"use client";

import Link from "next/link";
import { Ticket, Tag, Truck, ChevronRight } from "lucide-react";
import { PageShell, PageHeader, PageContent } from "@/components/merchant/layout/page-shell";
import { Card, CardContent } from "@/components/ui/card";

const marketingTools = [
  {
    title: "代金券管理",
    description: "创建各种额度的代金券，吸引新老客户领券下单。",
    icon: Ticket,
    href: "/merchant/marketing/vouchers",
    color: "bg-rose-50 text-rose-600",
    status: "已上线"
  },
  {
    title: "限时满减",
    description: "设置商户满减促销活动，提高客单价和订单量。",
    icon: Tag,
    href: "/merchant/marketing/discounts",
    color: "bg-amber-50 text-amber-600",
    status: "已上线"
  },
  {
    title: "运费满返",
    description: "满额免运费或返还运费，激励客户凑单提升客单价。",
    icon: Truck,
    href: "/merchant/marketing/delivery",
    color: "bg-blue-50 text-blue-600",
    status: "已上线"
  }
];

export default function MarketingPage() {
  return (
    <PageShell>
      <PageHeader
        title="营销中心"
        description="多种营销手段齐发力，助您提升店铺业绩。"
      />
      <PageContent>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {marketingTools.map((tool) => (
            <Link 
              key={tool.title} 
              href={tool.status === "已上线" ? tool.href : "#"}
              className={tool.status === "已上线" ? "cursor-pointer" : "cursor-not-allowed opacity-75"}
            >
              <Card className="h-full hover:shadow-md transition-all border-slate-100 group overflow-hidden">
                <CardContent className="p-6 flex flex-col h-full">
                  <div className="flex justify-between items-start mb-4">
                    <div className={`p-3 rounded-xl ${tool.color}`}>
                      <tool.icon className="h-6 w-6" />
                    </div>
                    <span className={`text-[10px] px-2 py-0.5 rounded-full font-medium ${
                      tool.status === "已上线" 
                        ? "bg-emerald-50 text-emerald-600 border border-emerald-100" 
                        : "bg-slate-100 text-slate-500 border border-slate-200"
                    }`}>
                      {tool.status}
                    </span>
                  </div>
                  
                  <h3 className="font-bold text-lg mb-2 flex items-center group-hover:text-primary transition-colors">
                    {tool.title}
                    {tool.status === "已上线" && <ChevronRight className="h-4 w-4 ml-1 opacity-0 -translate-x-2 group-hover:opacity-100 group-hover:translate-x-0 transition-all" />}
                  </h3>
                  
                  <p className="text-sm text-muted-foreground leading-relaxed flex-1">
                    {tool.description}
                  </p>
                  
                  {tool.status === "已上线" && (
                    <div className="mt-6 pt-4 border-t border-slate-50 flex items-center text-xs font-semibold text-primary">
                      立即进入管理
                    </div>
                  )}
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>
      </PageContent>
    </PageShell>
  );
}
