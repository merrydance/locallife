
import { ReviewsPageClient } from "@/components/merchant/reviews-page-client";

export const metadata = {
  title: "评价管理 - 商家管理后台",
  description: "查看并回复顾客对菜品和服务的评价，提升商户口碑与信用",
};

export default function ReviewsPage() {
  return <ReviewsPageClient />;
}
