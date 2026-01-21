import { redirect } from "next/navigation";

export default function AnalyticsCustomersPage() {
  redirect("/merchant/analytics/dashboard?tab=customer");
}
