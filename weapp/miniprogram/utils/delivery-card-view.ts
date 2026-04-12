export type DeliveryCardAction = 'accept' | 'pickup' | 'deliver' | ''

interface DeliveryCardView {
  statusClass: string
  statusText: string
  buttonClass: string
  buttonText: string
  nextAction: DeliveryCardAction
  incomeText: string
}

type DeliveryTaskStatusBucket = 'new' | 'pickup' | 'deliver' | 'done'

const NEW_TASK_STATUSES = new Set<string | number>(['PENDING', 'CONFIRMED', 'CREATED', 0])
const PICKUP_TASK_STATUSES = new Set<string | number>(['ACCEPTED', 1])
const DELIVERY_TASK_STATUSES = new Set<string | number>(['PICKED_UP', 'DELIVERING', 2])

function getStatusBucket(status?: string | number): DeliveryTaskStatusBucket {
  if (NEW_TASK_STATUSES.has(status || '')) return 'new'
  if (PICKUP_TASK_STATUSES.has(status || '')) return 'pickup'
  if (DELIVERY_TASK_STATUSES.has(status || '')) return 'deliver'
  return 'done'
}

const DELIVERY_CARD_VIEW_MAP: Record<DeliveryTaskStatusBucket, Omit<DeliveryCardView, 'incomeText'>> = {
  new: {
    statusClass: 'orange',
    statusText: '新任务',
    buttonClass: 'btn-orange',
    buttonText: '抢单',
    nextAction: 'accept'
  },
  pickup: {
    statusClass: 'blue',
    statusText: '待取货',
    buttonClass: 'btn-blue',
    buttonText: '确认取货',
    nextAction: 'pickup'
  },
  deliver: {
    statusClass: 'green',
    statusText: '配送中',
    buttonClass: 'btn-green',
    buttonText: '确认送达',
    nextAction: 'deliver'
  },
  done: {
    statusClass: 'gray',
    statusText: '已完成',
    buttonClass: 'btn-gray',
    buttonText: '查看详情',
    nextAction: ''
  }
}

export function buildDeliveryCardView(task?: { status?: string | number, fee?: number }): DeliveryCardView {
  const bucket = getStatusBucket(task?.status)
  return {
    ...DELIVERY_CARD_VIEW_MAP[bucket],
    incomeText: ((Number(task?.fee || 0)) / 100).toFixed(2)
  }
}