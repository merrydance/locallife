/**
 * API通用类型定义
 */

// ==================== 基础响应类型 ====================

export interface ApiResponse<T = unknown> {
    code: number
    message: string
    data: T
    timestamp?: string
}

export interface PagingData<T> {
    items: T[]
    total: number
    page?: number
    page_size?: number
}

// ==================== 请求参数类型 ====================

/**
 * 分页参数
 */
export interface PagingParams {
    page: number
    page_size?: number
}

/**
 * 位置参数
 */
export interface LocationParams {
    latitude: number
    longitude: number
}

/**
 * 搜索参数
 */
export interface SearchParams extends PagingParams {
    keyword?: string
}

/**
 * 位置搜索参数
 */
export interface LocationSearchParams extends LocationParams, SearchParams { }

// ==================== 错误类型 ====================

/**
 * API错误响应
 */
export interface ApiError {
    code: number
    message: string
    details?: Record<string, unknown>
    timestamp?: string
}

/**
 * 错误码枚举
 */
export enum ErrorCode {
    SUCCESS = 0,
    BAD_REQUEST = 400,
    UNAUTHORIZED = 401,
    FORBIDDEN = 403,
    NOT_FOUND = 404,
    INTERNAL_ERROR = 500,
    TOKEN_EXPIRED = 1001
}

// ==================== 业务通用类型 ====================

/**
 * ID类型
 */
export type ID = string

/**
 * 时间戳类型 (毫秒)
 */
export type Timestamp = number

/**
 * 金额类型 (分)
 */
export type Amount = number

/**
 * 状态枚举
 */
export enum Status {
    ACTIVE = 'ACTIVE',
    INACTIVE = 'INACTIVE',
    DELETED = 'DELETED'
}

/**
 * 排序参数
 */
export interface SortParams {
    sort_by?: string
    sort_order?: 'asc' | 'desc'
}

/**
 * 日期范围参数
 */
export interface DateRangeParams {
    start_date?: string  // YYYY-MM-DD
    end_date?: string    // YYYY-MM-DD
}

// ==================== 工具类型 ====================

/**
 * 使某些字段可选
 */
export type PartialBy<T, K extends keyof T> = Omit<T, K> & Partial<Pick<T, K>>

/**
 * 使某些字段必填
 */
export type RequiredBy<T, K extends keyof T> = Omit<T, K> & Required<Pick<T, K>>

/**
 * 深度只读
 */
export type DeepReadonly<T> = {
    readonly [P in keyof T]: T[P] extends object ? DeepReadonly<T[P]> : T[P]
}

/**
 * 深度可选
 */
export type DeepPartial<T> = {
    [P in keyof T]?: T[P] extends object ? DeepPartial<T[P]> : T[P]
}
