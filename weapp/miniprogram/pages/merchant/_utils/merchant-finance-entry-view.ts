export const MERCHANT_FINANCE_PAGE_PATH = '/pages/merchant/finance/index'
export const MERCHANT_FINANCE_BILLS_PAGE_PATH = '/pages/merchant/finance/bills/index'
export const MERCHANT_FINANCE_SETTLEMENTS_PAGE_PATH = '/pages/merchant/finance/settlements/index'
export const MERCHANT_SETTLEMENT_ACCOUNT_PAGE_PATH = '/pages/merchant/finance/settlement-account/index'
export const MERCHANT_WITHDRAWALS_PAGE_PATH = '/pages/merchant/finance/withdrawals/index'

export const MERCHANT_FINANCE_LABEL = '财务'
export const MERCHANT_FINANCE_BILLS_LABEL = '订单流水'
export const MERCHANT_FINANCE_SETTLEMENTS_LABEL = '结算记录'
export const MERCHANT_SETTLEMENT_ACCOUNT_LABEL = '结算账户'
export const MERCHANT_WITHDRAWALS_LABEL = '提现'
export const MERCHANT_CANCEL_WITHDRAWAL_EXTERNAL_DESCRIPTION = '注销提现请在微信支付商户平台处理'

export interface MerchantFinanceEntryView {
  id: string
  title: string
  icon: string
  path: string
}

export function buildMerchantFinanceEntries(): MerchantFinanceEntryView[] {
  return [
    {
      id: 'bills',
      title: MERCHANT_FINANCE_BILLS_LABEL,
      icon: 'chart-bar',
      path: MERCHANT_FINANCE_BILLS_PAGE_PATH
    },
    {
      id: 'settlements',
      title: MERCHANT_FINANCE_SETTLEMENTS_LABEL,
      icon: 'time',
      path: MERCHANT_FINANCE_SETTLEMENTS_PAGE_PATH
    },
    {
      id: 'settlement-account',
      title: MERCHANT_SETTLEMENT_ACCOUNT_LABEL,
      icon: 'creditcard',
      path: MERCHANT_SETTLEMENT_ACCOUNT_PAGE_PATH
    },
    {
      id: 'withdrawals',
      title: MERCHANT_WITHDRAWALS_LABEL,
      icon: 'wallet',
      path: MERCHANT_WITHDRAWALS_PAGE_PATH
    }
  ]
}
