/**
 * 商品管理接口
 * 包含商品增删改查、上下架等功能
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/**
 * 商品分类
 */
export interface Category {
    id: number
    name: string
    sort: number
}

/**
 * 商品信息
 */
export interface Product {
    id: number
    merchant_id: number
    category_id: number
    category_name?: string
    name: string
    description: string
    price: number // 分
    original_price?: number // 原价
    images: string[]
    status: 'on_shelf' | 'off_shelf'
    stock: number // -1 表示无限
    sort: number
    sales_count?: number
}

/**
 * 创建/更新商品请求
 */
export interface ProductRequest {
    category_id: number
    name: string
    description?: string
    price: number
    original_price?: number
    images: string[]
    status: 'on_shelf' | 'off_shelf'
    stock?: number
    sort?: number
}

// ==================== 商品服务 ====================

export class ProductService {

    /**
     * 获取商品列表 (商家端)
     * GET /v1/merchant/products
     */
    static async getProducts(params: { category_id?: number, page_id: number, page_size: number }): Promise<{ products: Product[], total: number }> {
        return await request({
            url: '/v1/merchant/products',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取商品详情
     * GET /v1/merchant/products/:id
     */
    static async getProductDetail(id: number): Promise<Product> {
        return await request({
            url: `/v1/merchant/products/${id}`,
            method: 'GET'
        })
    }

    /**
     * 创建商品
     * POST /v1/merchant/products
     */
    static async createProduct(data: ProductRequest): Promise<Product> {
        return await request({
            url: '/v1/merchant/products',
            method: 'POST',
            data: data as any
        })
    }

    /**
     * 更新商品
     * PUT /v1/merchant/products/:id
     */
    static async updateProduct(id: number, data: ProductRequest): Promise<Product> {
        return await request({
            url: `/v1/merchant/products/${id}`,
            method: 'PUT',
            data: data as any
        })
    }

    /**
     * 删除商品
     * DELETE /v1/merchant/products/:id
     */
    static async deleteProduct(id: number): Promise<void> {
        return await request({
            url: `/v1/merchant/products/${id}`,
            method: 'DELETE'
        })
    }

    /**
     * 更新商品状态
     * POST /v1/merchant/products/:id/status
     */
    static async updateProductStatus(id: number, status: 'on_shelf' | 'off_shelf'): Promise<Product> {
        return await request({
            url: `/v1/merchant/products/${id}/status`,
            method: 'POST',
            data: { status }
        })
    }

    /**
     * 获取分类列表
     * GET /v1/merchant/categories
     */
    static async getCategories(): Promise<Category[]> {
        return await request({
            url: '/v1/merchant/categories',
            method: 'GET'
        })
    }
}

export default ProductService
