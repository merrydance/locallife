import { redirect } from "next/navigation";

export default function AnalyticsDishesPage() {
  redirect("/merchant/analytics/dashboard?tab=sales");
}
