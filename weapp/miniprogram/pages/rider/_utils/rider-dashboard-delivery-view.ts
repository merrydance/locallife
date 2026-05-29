import { getDeliveryStatusDisplay } from '../_main_shared/api/delivery'
import type { Delivery } from '../_main_shared/api/delivery'
import type { RiderWorkbenchDeliveryItem } from '../_api/rider-workbench'
import { buildRiderDeliveryDeadlineView, getRiderDeliveryActionState } from '../_utils/rider-delivery-view'
import { buildRiderDeliveryIncomeView } from '../_utils/rider-delivery-income-view'
import type { RiderDeliveryIncomeView } from '../_utils/rider-delivery-income-view'
import { resolveStatusTagTheme, type StatusTagTheme } from '../_main_shared/utils/status-tag'

export const RIDER_DASHBOARD_MAX_GRAB_DISTANCE = 5000

export type RiderDashboardTagTheme = StatusTagTheme

export type DashboardDeliveryView = Delivery & {
  status_desc: string
  status_tag_theme: RiderDashboardTagTheme
  deadline_desc: string
  is_overdue: boolean
  is_very_urgent: boolean
  is_pickup_finished: boolean
  can_start_pickup: boolean
  can_confirm_pickup: boolean
  can_start_delivery: boolean
  can_confirm_delivery: boolean
  pickup_block_reason: string
  pickup_action_label: string
  is_action_loading: boolean
  income_view: RiderDeliveryIncomeView
}

export interface DashboardBannerState {
  message: string
  canRetry: boolean
}

export interface GrabDistanceCheckResult {
  canGrab: boolean
  distance: number
  message: string
}

export function buildRiderDashboardBannerState(states: DashboardBannerState[]): DashboardBannerState {
  if (!states.length) {
    return { message: '', canRetry: true }
  }

  const uniqueMessages = Array.from(new Set(states.map((state) => state.message).filter(Boolean)))

  return {
    message: uniqueMessages.join('；'),
    canRetry: states.every((state) => state.canRetry)
  }
}

export function isRiderDashboardTrackableDelivery(status: Delivery['status']): boolean {
  return status === 'assigned' || status === 'picking' || status === 'picked' || status === 'delivering'
}

export function buildRiderDashboardDeliveryActionState(delivery: Pick<
  Delivery,
  'status' | 'can_confirm_pickup' | 'pickup_block_reason' | 'pickup_action_label'
>) {
  const pickupActionState = getRiderDeliveryActionState(delivery)
  const status = delivery.status

  return {
    statusTagTheme: status === 'assigned' || status === 'picking'
      ? resolveStatusTagTheme('warning')
      : resolveStatusTagTheme('success'),
    isPickupFinished: status === 'picked' || status === 'delivering',
    canStartPickup: status === 'assigned',
    canConfirmPickup: status === 'picking' && pickupActionState.canUpdate,
    canStartDelivery: status === 'picked',
    canConfirmDelivery: status === 'delivering',
    pickupBlockReason: status === 'picking' ? pickupActionState.disabledReason : '',
    pickupActionLabel: status === 'picking' ? pickupActionState.label : ''
  }
}

export function buildDashboardDeliveryView(
  delivery: Delivery,
  loadingIds: number[] = [],
  fallbackStatusDesc: (status: Delivery['status']) => string = (status) => status
): DashboardDeliveryView {
  const deadlineView = buildRiderDeliveryDeadlineView(delivery)
  const statusDisplay = getDeliveryStatusDisplay(delivery.status)
  const actionState = buildRiderDashboardDeliveryActionState(delivery)

  return {
    ...delivery,
    status_desc: statusDisplay.text || fallbackStatusDesc(delivery.status),
    status_tag_theme: actionState.statusTagTheme,
    deadline_desc: deadlineView.text,
    is_overdue: deadlineView.isOverdue,
    is_very_urgent: deadlineView.isVeryUrgent,
    is_pickup_finished: actionState.isPickupFinished,
    can_start_pickup: actionState.canStartPickup,
    can_confirm_pickup: actionState.canConfirmPickup,
    can_start_delivery: actionState.canStartDelivery,
    can_confirm_delivery: actionState.canConfirmDelivery,
    pickup_block_reason: actionState.pickupBlockReason,
    pickup_action_label: actionState.pickupActionLabel,
    is_action_loading: loadingIds.includes(delivery.id),
    income_view: buildRiderDeliveryIncomeView(delivery)
  }
}

export function buildWorkbenchDashboardDeliveryView(item: RiderWorkbenchDeliveryItem): DashboardDeliveryView {
  return buildDashboardDeliveryView({
    id: item.id,
    order_id: item.order_id,
    order_status: item.order_status,
    fulfillment_status: item.fulfillment_status,
    status: item.status,
    delivery_fee: item.delivery_fee,
    rider_earnings: item.rider_earnings,
    rider_gross_amount: item.rider_gross_amount,
    rider_payment_fee: item.rider_payment_fee,
    rider_net_earnings: item.rider_net_earnings,
    profit_sharing_order_id: item.profit_sharing_order_id,
    profit_sharing_status: item.profit_sharing_status,
    pickup_address: item.pickup_address,
    pickup_longitude: 0,
    pickup_latitude: 0,
    delivery_address: item.delivery_address,
    delivery_longitude: 0,
    delivery_latitude: 0,
    estimated_pickup_at: item.estimated_pickup_at,
    estimated_delivery_at: item.estimated_delivery_at,
    picked_at: item.picked_at,
    delivered_at: item.delivered_at,
    created_at: item.created_at
  })
}

export function getRiderDashboardDistanceMeters(
  lat1: number,
  lng1: number,
  lat2: number,
  lng2: number
): number {
  const radiusMeters = 6371e3
  const firstLatitude = lat1 * Math.PI / 180
  const secondLatitude = lat2 * Math.PI / 180
  const latitudeDelta = (lat2 - lat1) * Math.PI / 180
  const longitudeDelta = (lng2 - lng1) * Math.PI / 180

  const a = Math.sin(latitudeDelta / 2) * Math.sin(latitudeDelta / 2) +
    Math.cos(firstLatitude) * Math.cos(secondLatitude) *
    Math.sin(longitudeDelta / 2) * Math.sin(longitudeDelta / 2)
  const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a))

  return radiusMeters * c
}

export function buildRiderDashboardGrabDistanceCheck(params: {
  currentLatitude: number
  currentLongitude: number
  pickupLatitude: number
  pickupLongitude: number
  maxDistance?: number
}): GrabDistanceCheckResult {
  const maxDistance = params.maxDistance || RIDER_DASHBOARD_MAX_GRAB_DISTANCE
  const distance = getRiderDashboardDistanceMeters(
    params.currentLatitude,
    params.currentLongitude,
    params.pickupLatitude,
    params.pickupLongitude
  )

  if (distance <= maxDistance) {
    return { canGrab: true, distance, message: '' }
  }

  return {
    canGrab: false,
    distance,
    message: `距离过远 (约${(distance / 1000).toFixed(1)}km)，仅限${maxDistance / 1000}km内抢单`
  }
}
