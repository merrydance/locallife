import {
  deliveryFeeService,
  type CreatePeakHourConfigRequest,
  type DeliveryFeeConfigResponse,
  type PeakHourConfigResponse
} from '../_main_shared/api/delivery-fee'

export type OperatorCreatePeakHourConfigRequest = CreatePeakHourConfigRequest

export interface OperatorDeliveryFeeConfigView {
  base_fee: string
  base_distance: string
  extra_fee_per_km: string
  value_ratio: string
  min_fee: string
  max_fee: string
  is_active: boolean
}

export interface OperatorPeakHourViewItem extends PeakHourConfigResponse {
  daysText: string
}

export interface OperatorRegionConfigSummary {
  baseFee: string
  baseDistance: string
  extraFeePerKm: string
  valueRatio: string
  minFee: string
  maxFee: string
  statusText: string
}

export interface OperatorPeakConfigSummary {
  count: number
  daysText: string
  note: string
}

export interface OperatorRegionConfigOverview {
  feeSummary: OperatorRegionConfigSummary
  peakSummary: OperatorPeakConfigSummary
}

export function createEmptyOperatorRegionConfigSummary(): OperatorRegionConfigSummary {
  return {
    baseFee: '未配置',
    baseDistance: '未配置',
    extraFeePerKm: '未配置',
    valueRatio: '未配置',
    minFee: '未配置',
    maxFee: '不限',
    statusText: '当前区域还没有基础代取费配置'
  }
}

export function createEmptyOperatorPeakConfigSummary(): OperatorPeakConfigSummary {
  return {
    count: 0,
    daysText: '暂无峰时时段',
    note: '当前后端仅支持新增和删除；若需调整已有时段，请删除后重建。'
  }
}

export function createEmptyOperatorDeliveryFeeConfigView(): OperatorDeliveryFeeConfigView {
  return {
    base_fee: '',
    base_distance: '',
    extra_fee_per_km: '',
    value_ratio: '0',
    min_fee: '',
    max_fee: '',
    is_active: true
  }
}

function adaptDeliveryFeeConfigToView(config: DeliveryFeeConfigResponse): OperatorDeliveryFeeConfigView {
  return {
    base_fee: (config.base_fee / 100).toFixed(2),
    base_distance: String(config.base_distance),
    extra_fee_per_km: (config.extra_fee_per_km / 100).toFixed(2),
    value_ratio: String(config.value_ratio),
    min_fee: (config.min_fee / 100).toFixed(2),
    max_fee: typeof config.max_fee === 'number' ? (config.max_fee / 100).toFixed(2) : '',
    is_active: config.is_active
  }
}

function formatFen(amount?: number): string {
  if (typeof amount !== 'number') {
    return '未配置'
  }
  return `${(amount / 100).toFixed(2)}元`
}

function formatDayList(days: number[]): string {
  const map = ['日', '一', '二', '三', '四', '五', '六']
  return days.map((day) => map[day] || '').filter(Boolean).join('、')
}

export function formatOperatorPeakDays(days: number[]): string {
  return formatDayList(days || [])
}

export function hasOperatorPeakConflict(items: OperatorPeakHourViewItem[], startTime: string, endTime: string, days: number[]) {
  const nextStart = timeToMinutes(startTime)
  const nextEnd = timeToMinutes(endTime)

  return items.find((item) => {
    const overlapDay = (item.days_of_week || []).some((day) => days.includes(day))
    if (!overlapDay) {
      return false
    }

    const currentStart = timeToMinutes(item.start_time)
    const currentEnd = timeToMinutes(item.end_time)
    return nextStart < currentEnd && nextEnd > currentStart
  })
}

export async function loadOperatorDeliveryFeeConfigView(regionId: number): Promise<OperatorDeliveryFeeConfigView> {
  const config = await deliveryFeeService.getRegionConfig(regionId)
  return adaptDeliveryFeeConfigToView(config)
}

export async function saveOperatorDeliveryFeeConfig(regionId: number, config: OperatorDeliveryFeeConfigView): Promise<OperatorDeliveryFeeConfigView> {
  const savedConfig = await deliveryFeeService.updateRegionConfig(regionId, {
    region_id: regionId,
    base_fee: Math.round(Number(config.base_fee) * 100),
    base_distance: Number(config.base_distance),
    extra_fee_per_km: Math.round(Number(config.extra_fee_per_km) * 100),
    value_ratio: Number(config.value_ratio),
    min_fee: Math.round(Number(config.min_fee) * 100),
    max_fee: config.max_fee ? Math.round(Number(config.max_fee) * 100) : null,
    is_active: config.is_active
  })
  return adaptDeliveryFeeConfigToView(savedConfig)
}

export async function loadOperatorPeakHourViews(regionId: number): Promise<OperatorPeakHourViewItem[]> {
  const items = await deliveryFeeService.getPeakConfigs(regionId)
  return items.map((item) => ({
    ...item,
    daysText: formatOperatorPeakDays(item.days_of_week || [])
  }))
}

export async function createOperatorPeakHour(regionId: number, data: CreatePeakHourConfigRequest): Promise<void> {
  await deliveryFeeService.createPeakConfig(regionId, data)
}

export async function deleteOperatorPeakHour(id: number): Promise<void> {
  await deliveryFeeService.deletePeakConfig(id)
}

export async function loadOperatorRegionConfigOverview(regionId: number): Promise<OperatorRegionConfigOverview> {
  const [feeConfig, peakConfigs] = await Promise.all([
    loadOperatorFeeConfigSafe(regionId),
    deliveryFeeService.getPeakConfigs(regionId)
  ])

  return {
    feeSummary: buildFeeSummary(feeConfig),
    peakSummary: buildPeakSummary(peakConfigs)
  }
}

function buildFeeSummary(config: DeliveryFeeConfigResponse | null): OperatorRegionConfigSummary {
  if (!config) {
    return createEmptyOperatorRegionConfigSummary()
  }

  return {
    baseFee: formatFen(config.base_fee),
    baseDistance: `${config.base_distance}米`,
    extraFeePerKm: formatFen(config.extra_fee_per_km),
    valueRatio: `${(config.value_ratio * 100).toFixed(2)}%`,
    minFee: formatFen(config.min_fee),
    maxFee: typeof config.max_fee === 'number' ? formatFen(config.max_fee) : '不限',
    statusText: config.is_active ? '当前配置已启用' : '当前配置未启用'
  }
}

function buildPeakSummary(configs: PeakHourConfigResponse[]): OperatorPeakConfigSummary {
  if (!configs.length) {
    return createEmptyOperatorPeakConfigSummary()
  }

  const uniqueDays = Array.from(
    new Set(configs.reduce<number[]>((allDays, item) => allDays.concat(item.days_of_week || []), []))
  ).sort((left, right) => left - right)

  return {
    count: configs.length,
    daysText: uniqueDays.length ? `覆盖周${formatDayList(uniqueDays)}` : '已配置峰时时段',
    note: '当前后端仅支持新增和删除；若需调整已有时段，请删除后重建。'
  }
}

async function loadOperatorFeeConfigSafe(regionId: number): Promise<DeliveryFeeConfigResponse | null> {
  try {
    return await deliveryFeeService.getRegionConfig(regionId)
  } catch {
    return null
  }
}

function timeToMinutes(value: string): number {
  const [hours, minutes] = value.split(':').map((item) => parseInt(item, 10))
  return (hours * 60) + minutes
}
