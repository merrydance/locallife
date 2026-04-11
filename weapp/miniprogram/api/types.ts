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

export interface PaginationEnvelope {
    total?: number
    page?: number
    page_id?: number
    page_size?: number
    limit?: number
    total_pages?: number
    total_page?: number
    has_more?: boolean
}

export interface PaginatedListResult<T> {
    items: T[]
    total: number
    page: number
    pageSize: number
    hasMore: boolean
}

function toSafeNumber(value: unknown): number | null {
    return typeof value === 'number' && Number.isFinite(value) ? value : null
}

export function normalizePaginatedResult<T>(
    items: T[],
    response: PaginationEnvelope | null | undefined,
    fallback: { page: number, pageSize: number }
): PaginatedListResult<T> {
    const page = toSafeNumber(response?.page) ?? toSafeNumber(response?.page_id) ?? fallback.page
    const pageSize = toSafeNumber(response?.page_size) ?? toSafeNumber(response?.limit) ?? fallback.pageSize
    const total = toSafeNumber(response?.total) ?? items.length
    const totalPages = toSafeNumber(response?.total_pages) ?? toSafeNumber(response?.total_page)

    let hasMore = false
    if (typeof response?.has_more === 'boolean') {
        hasMore = response.has_more
    } else if (totalPages !== null) {
        hasMore = page < totalPages
    } else if (total > items.length) {
        hasMore = page * pageSize < total
    } else {
        hasMore = items.length === pageSize && items.length > 0
    }

    return {
        items,
        total,
        page,
        pageSize,
        hasMore
    }
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
    BAD_REQUEST = 40000,
    UNAUTHORIZED = 40100,
    FORBIDDEN = 40300,
    NOT_FOUND = 40400,
    CONFLICT = 40900,
    UNPROCESSABLE = 42200,
    TOO_MANY_REQUESTS = 42900,
    GATEWAY_TIMEOUT = 50400,
    INTERNAL_ERROR = 50000,
    BAD_GATEWAY = 50200,
    SERVICE_UNAVAILABLE = 50300,
    TOKEN_EXPIRED = 1001
}

/**
 * 将后端精细 5 位错误码映射到粗粒度 ErrorCode 分类。
 * 后端错误码规则：前 3 位对应 HTTP 语义，如 400xx → BAD_REQUEST(40000)。
 * 精确码不命中 enum 常量时，按前 3 位归类后统一处理。
 */
export function classifyErrorCode(code: number): ErrorCode {
    if (code === ErrorCode.SUCCESS) return ErrorCode.SUCCESS
    if (code === ErrorCode.TOKEN_EXPIRED) return ErrorCode.TOKEN_EXPIRED
    const hundreds = Math.floor(code / 100)
    switch (hundreds) {
        case 400: return ErrorCode.BAD_REQUEST
        case 401: return ErrorCode.UNAUTHORIZED
        case 403: return ErrorCode.FORBIDDEN
        case 404: return ErrorCode.NOT_FOUND
        case 409: return ErrorCode.CONFLICT
        case 422: return ErrorCode.UNPROCESSABLE
        case 429: return ErrorCode.TOO_MANY_REQUESTS
        case 500: return ErrorCode.INTERNAL_ERROR
        case 502: return ErrorCode.BAD_GATEWAY
        case 503: return ErrorCode.SERVICE_UNAVAILABLE
        case 504: return ErrorCode.GATEWAY_TIMEOUT
        default:  return code as ErrorCode
    }
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
