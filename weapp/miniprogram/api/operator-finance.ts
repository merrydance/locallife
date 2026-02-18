import { request } from '../utils/request'

export interface OperatorAccountBalanceResponse {
  sub_mch_id: string
  available_amount: number
  pending_amount: number
  withdrawable_amount: number
}

/**
 * 提现请求参数
 */
export interface WithdrawOperatorRequest {
  amount: number // 单位：分
}

/**
 * 运营商提现
 */
export const withdrawOperator = (data: WithdrawOperatorRequest) => {
  return request({
    url: '/v1/operators/me/finance/withdraw',
    method: 'POST',
    data
  })
}

/**
 * 获取运营商收付通账户余额（微信实时）
 */
export const getOperatorAccountBalance = (): Promise<OperatorAccountBalanceResponse> => {
  return request({
    url: '/v1/operators/me/finance/account/balance',
    method: 'GET'
  })
}
