import { redirect } from "next/navigation";

export default function AnalyticsSalesPage() {
  redirect("/merchant/analytics/dashboard?tab=sales");
}
