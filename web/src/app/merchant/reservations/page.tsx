import { ReservationsPageClient } from "@/components/merchant/reservations-page-client";
import { apiGet, formatDate } from "@/lib/api";
import type { ReservationResponse, ReservationStatsResponse } from "@/types/reservation";

const fallbackStats: ReservationStatsResponse = {
  pending_count: 0,
  paid_count: 0,
  confirmed_count: 0,
  completed_count: 0,
  cancelled_count: 0,
  expired_count: 0,
  no_show_count: 0,
};

interface MerchantReservationsResponse {
  reservations: ReservationResponse[];
  total: number;
  total_count: number;
}

export default async function ReservationsPage(props: {
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>
}) {
  const searchParams = await props.searchParams;
  const date = (searchParams?.date as string) || formatDate(new Date());

  const [stats, reservationsRes] = await Promise.all([
    apiGet<ReservationStatsResponse>("/reservations/merchant/stats").catch(
      () => fallbackStats
    ),
    apiGet<MerchantReservationsResponse>("/reservations/merchant", {
      page_id: 1,
      page_size: 1, // We only need total_count for the stats, the list is not used
      date: date || undefined,
    }).catch(() => ({ reservations: [], total: 0, total_count: 0 })),
  ]);

  const totalCount = reservationsRes.total_count || 0;

  return (
    <ReservationsPageClient
      totalCount={totalCount}
      stats={stats}
      date={date}
    />
  );
}
