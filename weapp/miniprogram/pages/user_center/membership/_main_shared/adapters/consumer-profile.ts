import { formatPriceNoSymbol } from '../../../../../utils/util'

const DEFAULT_MERCHANT_NAME = '商户'
const DEFAULT_MERCHANT_LOGO = '/assets/icons/shop.svg'

type MerchantIdentitySource = {
  merchant_id?: number
  merchantId?: number
  merchant_name?: string
  merchantName?: string
  merchant_logo_url?: string
  merchant_logo?: string
  logo_url?: string
  image_url?: string
}

type FavoriteMerchantSource = MerchantIdentitySource & {
  id: number
  address?: string
  created_at?: string
  is_ordering_suspended?: boolean
}

type MembershipSource = MerchantIdentitySource & {
  id: number
  balance?: number
  total_recharged?: number
  total_consumed?: number
  created_at?: string
}

export interface FavoriteMerchantDisplay {
  id: number
  targetId: number
  title: string
  image: string
  subTitle: string
  rating: number
  createdAt: string
  isOrderingSuspended: boolean
}

export interface MembershipCardDisplay {
  id: number
  merchantId: number
  merchantName: string
  logoUrl: string
  balanceDisplay: string
  totalRechargedDisplay: string
  totalConsumedDisplay: string
}

export interface WalletMembershipDisplay {
  id: number
  merchant_id: number
  merchant_name: string
  logo_url: string
  balance_display: string
  created_at_date: string
}

export interface ReviewMerchantDisplay {
  merchantId: number
  merchantName: string
  logoUrl: string
}

function formatDateOnly(value?: string): string {
  if (!value) {
    return ''
  }

  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return value.split('T')[0] || ''
  }

  const year = parsed.getFullYear()
  const month = String(parsed.getMonth() + 1).padStart(2, '0')
  const day = String(parsed.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function resolveMerchantId(source: MerchantIdentitySource): number {
  return source.merchant_id ?? source.merchantId ?? 0
}

function resolveMerchantName(source: MerchantIdentitySource, fallback: string = DEFAULT_MERCHANT_NAME): string {
  return source.merchant_name || source.merchantName || fallback
}

function resolveMerchantLogo(source: MerchantIdentitySource, fallback: string = DEFAULT_MERCHANT_LOGO): string {
  return source.merchant_logo_url || source.merchant_logo || source.logo_url || source.image_url || fallback
}

export class ConsumerProfileAdapter {
  static toFavoriteMerchantViewModel(source: FavoriteMerchantSource): FavoriteMerchantDisplay {
    return {
      id: source.id,
      targetId: resolveMerchantId(source),
      title: resolveMerchantName(source),
      image: resolveMerchantLogo(source),
      subTitle: source.address || '',
      rating: 5,
      createdAt: formatDateOnly(source.created_at),
      isOrderingSuspended: !!source.is_ordering_suspended
    }
  }

  static toMembershipCardViewModel(source: MembershipSource): MembershipCardDisplay {
    return {
      id: source.id,
      merchantId: resolveMerchantId(source),
      merchantName: resolveMerchantName(source),
      logoUrl: resolveMerchantLogo(source),
      balanceDisplay: formatPriceNoSymbol(source.balance || 0),
      totalRechargedDisplay: formatPriceNoSymbol(source.total_recharged || 0),
      totalConsumedDisplay: formatPriceNoSymbol(source.total_consumed || 0)
    }
  }

  static toWalletMembershipViewModel(source: MembershipSource): WalletMembershipDisplay {
    return {
      id: source.id,
      merchant_id: resolveMerchantId(source),
      merchant_name: resolveMerchantName(source, '商户会员卡'),
      logo_url: resolveMerchantLogo(source, ''),
      balance_display: formatPriceNoSymbol(source.balance || 0),
      created_at_date: formatDateOnly(source.created_at)
    }
  }

  static toReviewMerchantViewModel(source: MerchantIdentitySource): ReviewMerchantDisplay {
    const merchantId = resolveMerchantId(source)
    return {
      merchantId,
      merchantName: resolveMerchantName(source, merchantId > 0 ? `商户 #${merchantId}` : DEFAULT_MERCHANT_NAME),
      logoUrl: resolveMerchantLogo(source)
    }
  }
}

export default ConsumerProfileAdapter