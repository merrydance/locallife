import { request } from '../utils/request'

export interface AgreementBrief {
  type: string
  title: string
  version: string
  published_on: string
}

export interface AgreementDetail {
  id: number
  type: string
  title: string
  content: string
  version: string
  published_on: string
  is_active: boolean
  created_at: string
  updated_at: string
}

export const AgreementService = {
  /**
   * 获取活跃协议列表
   */
  async listActiveAgreements(): Promise<AgreementBrief[]> {
    return request({ url: '/v1/agreements', method: 'GET' })
  },

  /**
   * 获取协议详情
   * @param type 协议类型
   */
  async getAgreement(type: string): Promise<AgreementDetail> {
    return request({ url: `/v1/agreements/${type}`, method: 'GET' })
  }
}
