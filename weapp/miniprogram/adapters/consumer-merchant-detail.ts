import { getPublicImageUrl } from '../utils/image'
import type { PublicMerchantDetail } from '../api/merchant'
import { buildMerchantDisplayTags } from './merchant-labels'

export type BusinessHoursView = NonNullable<PublicMerchantDetail['business_hours']>[number] & {
  day_name: string
}

export interface ConsumerMerchantDetailViewModel {
  id: number
  name: string
  cover_image?: string
  logo_url: string
  address: string
  phone: string
  latitude: number
  longitude: number
  tags: string[]
  systemLabels: string[]
  displayTags: string[]
  monthly_sales: number
  avg_prep_minutes: number
  biz_status: 'OPEN' | 'CLOSED'
  description: string
  business_license_image_url?: string
  food_permit_url?: string
  business_hours: BusinessHoursView[]
  business_hours_display: string
  discount_rules: PublicMerchantDetail['discount_rules']
  vouchers: PublicMerchantDetail['vouchers']
  delivery_promotions: PublicMerchantDetail['delivery_promotions']
  is_ordering_suspended: boolean
  ordering_status_label: '正常接单' | '暂停接单'
  ordering_status_tone: 'open' | 'closed'
}

const DAY_NAMES = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']

function formatBusinessHoursDisplay(hours: PublicMerchantDetail['business_hours']): string {
  if (!hours || hours.length === 0) {
    return ''
  }

  const today = new Date().getDay()
  const todayHours = hours.find((item) => item.day_of_week === today)
  const target = todayHours || hours[0]

  if (!target) {
    return ''
  }

  return `${target.open_time} - ${target.close_time}`
}

export class ConsumerMerchantDetailAdapter {
  static toViewModel(merchant: PublicMerchantDetail): ConsumerMerchantDetailViewModel {
    const isOpen = !!merchant.is_open
    const isOrderingPaused = !isOpen || !!merchant.is_ordering_suspended

    return {
      id: merchant.id,
      name: merchant.name,
      cover_image: getPublicImageUrl(merchant.cover_image || merchant.logo_url || '') || '',
      logo_url: getPublicImageUrl(merchant.logo_url || ''),
      address: merchant.address,
      phone: merchant.phone,
      latitude: merchant.latitude,
      longitude: merchant.longitude,
      tags: merchant.tags || [],
      systemLabels: merchant.system_labels || [],
      displayTags: buildMerchantDisplayTags(merchant.system_labels || [], merchant.tags || []),
      monthly_sales: merchant.monthly_sales || 0,
      avg_prep_minutes: merchant.avg_prep_minutes || 15,
      biz_status: isOpen ? 'OPEN' : 'CLOSED',
      description: merchant.description || '',
      business_license_image_url: merchant.business_license_image_url,
      food_permit_url: merchant.food_permit_url,
      business_hours: (merchant.business_hours || []).map((item) => ({
        ...item,
        day_name: DAY_NAMES[item.day_of_week]
      })),
      business_hours_display: formatBusinessHoursDisplay(merchant.business_hours),
      discount_rules: merchant.discount_rules || [],
      vouchers: merchant.vouchers || [],
      delivery_promotions: merchant.delivery_promotions || [],
      is_ordering_suspended: !!merchant.is_ordering_suspended,
      ordering_status_label: isOrderingPaused ? '暂停接单' : '正常接单',
      ordering_status_tone: isOrderingPaused ? 'closed' : 'open'
    }
  }
}

export default ConsumerMerchantDetailAdapter
