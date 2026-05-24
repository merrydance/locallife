import { Delivery } from '../api/delivery'

export type RiderDeliveryActionKey = 'start_pickup' | 'confirm_pickup' | 'start_delivery' | 'confirm_delivery' | ''

interface RiderDeliveryActionState {
  canUpdate: boolean
  label: string
  actionKey: RiderDeliveryActionKey
  expectedStatus: Delivery['status'] | null
  locationSource: string
}

interface RiderNavigationStopView {
  title: string
  address: string
  latitude: number
  longitude: number
}

const PICKUP_STAGE_STATUSES = new Set<Delivery['status']>(['assigned', 'picking'])
const TRACKED_STATUSES = new Set<Delivery['status']>(['assigned', 'picking', 'picked', 'delivering'])
const DELIVERED_STATUSES = new Set<Delivery['status']>(['delivered', 'completed'])

const DELIVERY_ACTION_STATE_MAP: Partial<Record<Delivery['status'], Omit<RiderDeliveryActionState, 'canUpdate'>>> = {
  assigned: {
    label: '我已到达商家',
    actionKey: 'start_pickup',
    expectedStatus: 'picking',
    locationSource: 'rider_task_detail_start_pickup'
  },
  picking: {
    label: '确认取餐',
    actionKey: 'confirm_pickup',
    expectedStatus: 'picked',
    locationSource: 'rider_task_detail_confirm_pickup'
  },
  picked: {
    label: '开始代取',
    actionKey: 'start_delivery',
    expectedStatus: 'delivering',
    locationSource: 'rider_task_detail_start_delivery'
  },
  delivering: {
    label: '确认已送达',
    actionKey: 'confirm_delivery',
    expectedStatus: 'delivered',
    locationSource: 'rider_task_detail_confirm_delivery'
  },
  pending: {
    label: '',
    actionKey: '',
    expectedStatus: null,
    locationSource: 'rider_task_detail_pending'
  },
  delivered: {
    label: '',
    actionKey: '',
    expectedStatus: null,
    locationSource: 'rider_task_detail_delivered'
  },
  completed: {
    label: '',
    actionKey: '',
    expectedStatus: null,
    locationSource: 'rider_task_detail_completed'
  },
  cancelled: {
    label: '',
    actionKey: '',
    expectedStatus: null,
    locationSource: 'rider_task_detail_cancelled'
  },
  exception: {
    label: '',
    actionKey: '',
    expectedStatus: null,
    locationSource: 'rider_task_detail_exception'
  }
}

const DELIVERY_STEP_MAP: Partial<Record<Delivery['status'], number>> = {
  assigned: 0,
  picking: 1,
  picked: 2,
  delivering: 2,
  delivered: 3,
  completed: 3
}

export function isRiderDeliveryTrackedStatus(status?: Delivery['status']): boolean {
  return !!status && TRACKED_STATUSES.has(status)
}

export function isExpectedDeliveryStatusReached(status: Delivery['status'], expectedStatus: Delivery['status']): boolean {
  if (expectedStatus === 'delivered') {
    return DELIVERED_STATUSES.has(status)
  }

  return status === expectedStatus
}

export function getRiderDeliveryActionState(status: Delivery['status']): RiderDeliveryActionState {
  const config = DELIVERY_ACTION_STATE_MAP[status]

  if (!config || !config.actionKey || !config.expectedStatus) {
    return {
      canUpdate: false,
      label: '',
      actionKey: '',
      expectedStatus: null,
      locationSource: DELIVERY_ACTION_STATE_MAP[status]?.locationSource || 'rider_task_detail_action'
    }
  }

  return {
    canUpdate: true,
    ...config
  }
}

export function getRiderDeliveryStep(status?: string): number {
  const normalizedStatus = String(status || '').trim().toLowerCase() as Delivery['status'] | ''
  if (!normalizedStatus) {
    return 0
  }

  return DELIVERY_STEP_MAP[normalizedStatus] ?? 0
}

export function getRiderDeliveryDeadline(delivery: Pick<Delivery, 'status' | 'estimated_pickup_at' | 'estimated_delivery_at'>): string | undefined {
  return PICKUP_STAGE_STATUSES.has(delivery.status) ? delivery.estimated_pickup_at : delivery.estimated_delivery_at
}

export function getRiderNavigationNextStop(delivery: Pick<Delivery, 'status' | 'pickup_address' | 'pickup_latitude' | 'pickup_longitude' | 'delivery_address' | 'delivery_latitude' | 'delivery_longitude'>): RiderNavigationStopView {
  if (PICKUP_STAGE_STATUSES.has(delivery.status)) {
    return {
      title: '下一站 · 商家',
      address: delivery.pickup_address,
      latitude: delivery.pickup_latitude,
      longitude: delivery.pickup_longitude
    }
  }

  return {
    title: '下一站 · 顾客',
    address: delivery.delivery_address,
    latitude: delivery.delivery_latitude,
    longitude: delivery.delivery_longitude
  }
}