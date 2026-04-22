import { operatorBasicManagementService } from '../api/operator-basic-management'

export interface OperatorCommissionRowView {
  date: string
  order_count: number
  total_gmv_fen: number
  total_commission_fen: number
}

interface CommissionListResponseLike {
  commissions?: Array<{
    items?: Array<{
      date: string
      order_count: number
      total_gmv: number
      commission: number
    }>
  }>
  items?: Array<{
    date: string
    order_count: number
    total_gmv: number
    commission: number
  }>
}

export interface OperatorFinancePageData {
  totalIncomeFen: number
  currentMonthIncomeFen: number
  currentMonthGmvFen: number
  currentMonthOrders: number
  currentMonthCommissionFen: number
  operatorShareRatio: number
  loadError: string
  commissionRows: OperatorCommissionRowView[]
  commissionError: string
}

function adaptCommissionRows(response: unknown): OperatorCommissionRowView[] {
  if (!response || typeof response !== 'object') {
    return []
  }

  const payload = response as CommissionListResponseLike
  const rawItems = Array.isArray(payload.items)
    ? payload.items
    : Array.isArray(payload.commissions)
      ? payload.commissions.reduce<Array<{ date: string, order_count: number, total_gmv: number, commission: number }>>((accumulator, item) => {
        if (Array.isArray(item.items)) {
          accumulator.push(...item.items)
        }
        return accumulator
      }, [])
      : []

  return rawItems.slice(0, 10).map((item) => ({
    date: item.date,
    order_count: Number(item.order_count || 0),
    total_gmv_fen: Number(item.total_gmv || 0),
    total_commission_fen: Number(item.commission || 0)
  }))
}

export async function loadOperatorFinancePageData(): Promise<OperatorFinancePageData> {
  const [overview, commissionList] = await Promise.all([
    operatorBasicManagementService.getFinanceOverview().catch(() => null),
    operatorBasicManagementService.getCommissionList({ page: 1, limit: 10 }).catch(() => null)
  ])

  return {
    totalIncomeFen: overview?.total?.operator_income ?? 0,
    currentMonthIncomeFen: overview?.current_month?.operator_income ?? 0,
    currentMonthGmvFen: overview?.current_month?.total_gmv ?? 0,
    currentMonthOrders: overview?.current_month?.total_orders ?? 0,
    currentMonthCommissionFen: overview?.current_month?.total_commission ?? 0,
    operatorShareRatio: overview?.operator_share_ratio ?? 0,
    loadError: overview ? '' : '收入概览加载失败，请稍后重试',
    commissionRows: adaptCommissionRows(commissionList),
    commissionError: commissionList ? '' : '佣金明细加载失败，请稍后重试'
  }
}