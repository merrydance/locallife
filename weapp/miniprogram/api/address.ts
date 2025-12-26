/**
 * 地址管理接口
 * 包含地址增删改查及设置默认地址
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/**
 * 地址信息
 */
export interface Address {
  id: number
  user_id: number
  contact_name: string
  contact_phone: string
  province: string
  city: string
  district: string
  address: string // 详细地址 (街道/楼号)
  latitude: number
  longitude: number
  is_default: boolean
  tag?: 'home' | 'company' | 'school' | 'other'
}

/**
 * 创建/更新地址请求
 */
export interface AddressRequest {
  contact_name: string
  contact_phone: string
  province: string
  city: string
  district: string
  address: string
  latitude: number
  longitude: number
  is_default?: boolean
  tag?: string
}

// ==================== 地址服务 ====================

export class AddressService {

  /**
   * 获取地址列表
   * GET /v1/addresses
   */
  static async getAddresses(): Promise<Address[]> {
    return await request({
      url: '/v1/addresses',
      method: 'GET'
    })
  }

  /**
   * 获取地址详情
   * GET /v1/addresses/:id
   */
  static async getAddressDetail(id: number): Promise<Address> {
    return await request({
      url: `/v1/addresses/${id}`,
      method: 'GET'
    })
  }

  /**
   * 创建地址
   * POST /v1/addresses
   */
  static async createAddress(data: AddressRequest): Promise<Address> {
    return await request({
      url: '/v1/addresses',
      method: 'POST',
      data: data as any
    })
  }

  /**
   * 更新地址
   * PUT /v1/addresses/:id
   */
  static async updateAddress(id: number, data: AddressRequest): Promise<Address> {
    return await request({
      url: `/v1/addresses/${id}`,
      method: 'PUT',
      data: data as any
    })
  }

  /**
   * 删除地址
   * DELETE /v1/addresses/:id
   */
  static async deleteAddress(id: number): Promise<void> {
    return await request({
      url: `/v1/addresses/${id}`,
      method: 'DELETE'
    })
  }

  /**
   * 设置默认地址
   * POST /v1/addresses/:id/default
   */
  static async setDefaultAddress(id: number): Promise<Address> {
    return await request({
      url: `/v1/addresses/${id}/default`,
      method: 'POST'
    })
  }
}

export default AddressService