/**
 * 地址管理接口
 * 对齐后端 user_address.go
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/**
 * 地址信息 - 对齐后端 userAddressResponse
 */
export interface Address {
  id: number
  user_id: number
  region_id: number
  region_name?: string
  detail_address: string
  contact_name: string
  contact_phone: string
  longitude: string  // 后端返回字符串
  latitude: string   // 后端返回字符串
  is_default: boolean
  created_at?: string
}

/**
 * 创建地址请求 - 对齐后端 createUserAddressRequest
 */
export interface CreateAddressRequest {
  region_id?: number          // 可选，不传则根据经纬度自动匹配
  detail_address: string      // 必填，详细地址
  contact_name: string        // 必填，联系人姓名
  contact_phone: string       // 必填，联系电话
  longitude: string           // 必填，经度（字符串）
  latitude: string            // 必填，纬度（字符串）
  is_default?: boolean        // 可选，是否设为默认
}

/**
 * 更新地址请求 - 对齐后端 updateUserAddressRequest (所有字段可选)
 */
export interface UpdateAddressRequest {
  region_id?: number
  detail_address?: string
  contact_name?: string
  contact_phone?: string
  longitude?: string
  latitude?: string
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
  static async createAddress(data: CreateAddressRequest): Promise<Address> {
    return await request({
      url: '/v1/addresses',
      method: 'POST',
      data: data as any
    })
  }

  /**
   * 更新地址
   * PATCH /v1/addresses/:id (注意：后端用 PATCH 不是 PUT)
   */
  static async updateAddress(id: number, data: UpdateAddressRequest): Promise<Address> {
    return await request({
      url: `/v1/addresses/${id}`,
      method: 'PATCH',  // 后端期望 PATCH
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
   * PATCH /v1/addresses/:id/default (注意：后端用 PATCH 不是 POST)
   */
  static async setDefaultAddress(id: number): Promise<Address> {
    return await request({
      url: `/v1/addresses/${id}/default`,
      method: 'PATCH'  // 后端期望 PATCH
    })
  }

  /**
   * 获取默认地址
   * 从地址列表中找到 is_default=true 的地址
   */
  static async getDefaultAddress(): Promise<Address | null> {
    const addresses = await this.getAddresses()
    return addresses.find(a => a.is_default) || addresses[0] || null
  }
}

export default AddressService