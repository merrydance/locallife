"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";

const tabs = [
  { label: "概览", value: "overview" },
  { label: "销售分析", value: "sales" },
  { label: "财务分析", value: "finance" },
  { label: "客户分析", value: "customer" },
];

export function AnalyticsTabs() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const currentTab = searchParams.get("tab") || "overview";
  const startDate = searchParams.get("start_date");
  const endDate = searchParams.get("end_date");

  const handleTabChange = (value: string) => {
    const params = new URLSearchParams(searchParams.toString());
    params.set("tab", value);
    if (startDate) params.set("start_date", startDate);
    if (endDate) params.set("end_date", endDate);
    router.push(`/merchant/analytics/dashboard?${params.toString()}`);
  };

  return (
    <Tabs value={currentTab} onValueChange={handleTabChange}>
      <TabsList>
        {tabs.map((tab) => (
          <TabsTrigger key={tab.value} value={tab.value}>
            {tab.label}
          </TabsTrigger>
        ))}
      </TabsList>
    </Tabs>
  );
}
