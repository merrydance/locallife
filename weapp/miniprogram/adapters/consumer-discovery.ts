import type { MerchantSummary } from '../api/merchant'
import type { RoomSearchResult } from '../api/search'
import { getPublicImageUrl } from '../utils/image'
import { DishAdapter } from './dish'
import { buildMerchantDisplayTags } from './merchant-labels'

export interface MerchantDiscoverySummary {
  id: number
  name: string
  imageUrl: string
  address: string
  distanceDisplay: string
  tags: string[]
  systemLabels: string[]
  displayTags: string[]
  monthlySales: number
  deliveryFee?: number
  deliveryFeeDisplay: string
  isOpen: boolean
  label: string
}

export interface NormalizedRoomSearchResult extends RoomSearchResult {
  merchant_name: string
  table_no: string
  merchant_address: string
  primary_image: string
  distance_display: string
}

type RoomSearchSource = RoomSearchResult & {
  merchantName?: string
  tableNo?: string
  merchantAddress?: string
}

export class ConsumerDiscoveryAdapter {
  static toMerchantSummaryViewModel(source: MerchantSummary): MerchantDiscoverySummary {
    const deliveryFee = source.estimated_delivery_fee

    return {
      id: source.id,
      name: source.name,
      imageUrl: getPublicImageUrl(source.cover_image || source.logo_url || '') || '',
      address: source.address || '',
      distanceDisplay: source.distance !== undefined ? DishAdapter.formatDistance(source.distance) : '',
      tags: source.tags || [],
      systemLabels: source.system_labels || [],
      displayTags: buildMerchantDisplayTags(source.system_labels || [], source.tags || []),
      monthlySales: source.total_orders ?? source.monthly_sales ?? 0,
      deliveryFee,
      deliveryFeeDisplay: deliveryFee !== undefined
        ? `代取费¥${(deliveryFee / 100).toFixed(0)}起`
        : '',
      isOpen: source.is_open === true,
      label: source.label || ''
    }
  }

  static toRoomSearchViewModel(source: RoomSearchSource): NormalizedRoomSearchResult {
    return {
      ...source,
      merchant_name: source.merchant_name || source.merchantName || '',
      table_no: source.table_no || source.tableNo || source.name || '',
      merchant_address: source.merchant_address || source.merchantAddress || '',
      primary_image: getPublicImageUrl(source.primary_image) || '',
      distance_display: source.distance !== undefined ? DishAdapter.formatDistance(source.distance) : ''
    }
  }
}

export default ConsumerDiscoveryAdapter
