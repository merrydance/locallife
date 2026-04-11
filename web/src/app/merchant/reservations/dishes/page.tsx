import { ReservationDishPrepPageClient } from "@/components/merchant/reservation-dish-prep-page-client";

export const metadata = {
  title: "预订备菜清单 - 商家管理后台",
  description: "按天查看预订菜品与数量，便于后厨备菜",
};

export default function MerchantReservationDishPrepPage() {
  return <ReservationDishPrepPageClient />;
}
