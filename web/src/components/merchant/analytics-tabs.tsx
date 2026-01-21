"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";

const tabs = [
  { label: "概览", value: "overview" },
  { label: "销售分析", value: "sales" },
  { label: "财务分析", value: "finance" },
  { label: "客户分析", value: "customer" },
];

export function AnalyticsTabs() {
  const searchParams = useSearchParams();
  const currentTab = searchParams.get("tab") || "overview";
  const startDate = searchParams.get("start_date");
  const endDate = searchParams.get("end_date");

  return (
    <nav className="flex gap-2 text-sm">
      {tabs.map((tab) => {
        const active = currentTab === tab.value;
        const params = new URLSearchParams();
        params.set("tab", tab.value);
        if (startDate) params.set("start_date", startDate);
        if (endDate) params.set("end_date", endDate);
        const href = `/merchant/analytics/dashboard?${params.toString()}`;
        return (
          <Link
            key={tab.value}
            href={href}
            className={`rounded-md border px-3 py-1.5 transition-colors ${
              active
                ? "border-primary bg-accent text-accent-foreground"
                : "text-muted-foreground hover:bg-muted"
            }`}
          >
            {tab.label}
          </Link>
        );
      })}
    </nav>
  );
}
