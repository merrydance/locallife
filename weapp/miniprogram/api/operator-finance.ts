import { request } from '../utils/request'

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
    url: '/v1/operator/finance/withdraw',
    method: 'POST',
    data
  })
}
