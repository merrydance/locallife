import { getOrderDetail } from '../api/order'
import ReviewService from '../api/review'
import { ReservationService, type ReservationResponse } from '../api/reservation'

export type OrderDetailReservation = ReservationResponse

export async function loadOrderDetailBundle(orderId: number) {
  const orderDTO = await getOrderDetail(orderId)

  let reservationInfo: ReservationResponse | null = null
  if (orderDTO.order_type === 'reservation' && orderDTO.reservation_id) {
    try {
      reservationInfo = await ReservationService.getReservationDetail(orderDTO.reservation_id)
    } catch (_error) {
      reservationInfo = null
    }
  }

  return {
    orderDTO,
    reservationInfo
  }
}

export function getOrderReview(orderId: number) {
  return ReviewService.getReviewByOrderId(orderId)
}