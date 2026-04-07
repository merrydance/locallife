import type { TableImageResponse, TableResponse } from '../../../api/table-device-management'
import { getTableStatusDisplay, type TableStatus } from '../../../api/table'
import { getPublicImageUrl } from '../../../utils/image'

export type TableTypeFilterKey = 'all' | 'table' | 'room'
export type TableStatusFilterKey = 'all' | TableStatus

export interface TableTagOption {
  id: number
  name: string
}

export const TABLE_UPLOAD_FILE_STATUS = {
  loading: 'loading',
  done: 'done',
  failed: 'failed'
} as const

export type TableUploadFileStatus = typeof TABLE_UPLOAD_FILE_STATUS[keyof typeof TABLE_UPLOAD_FILE_STATUS]

export interface TableUploadFile {
  url: string
  status?: TableUploadFileStatus
  mediaId?: number
  localPath?: string
}

export interface TableFormData {
  table_no: string
  table_type: 'table' | 'room'
  capacity: number
  description: string
  minimum_spend_yuan: string
  status: TableStatus
  tag_ids: number[]
}

export interface TableStatusFilterOption {
  label: string
  value: TableStatusFilterKey
}

export interface TableTabOption {
  label: string
  value: TableTypeFilterKey
  count: number
}

export interface TableSummaryMetric {
  id: string
  label: string
  value: string
  note: string
}

export interface TableListItem extends TableResponse {
  normalizedTableType: 'table' | 'room'
  statusLabel: string
  statusTheme: 'success' | 'warning' | 'danger' | 'default'
  canRelease: boolean
  canShowCode: boolean
  isAvailableLike: boolean
  typeLabel: string
  typeDescription: string
  capacityText: string
  minimumSpendText: string
  descriptionText: string
  visibleTags: string[]
  tagSummaryText: string
  qrCodeUrl: string
  coverImageUrl: string
}

export const TABLE_STATUS_FILTER_OPTIONS: TableStatusFilterOption[] = [
  { label: '全部状态', value: 'all' },
  { label: '空闲', value: 'available' },
  { label: '占用中', value: 'occupied' },
  { label: '已预订', value: 'reserved' },
  { label: '停用', value: 'disabled' }
]

export function createDefaultTableFormData(): TableFormData {
  return {
    table_no: '',
    table_type: 'table',
    capacity: 4,
    description: '',
    minimum_spend_yuan: '',
    status: 'available',
    tag_ids: []
  }
}

export function ensureArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : []
}

export function toSafeTagOptions(value: unknown): TableTagOption[] {
  if (!Array.isArray(value)) {
    return []
  }

  const result: TableTagOption[] = []
  for (const item of value) {
    if (!item || typeof item !== 'object') {
      continue
    }

    const candidate = item as { id?: unknown, name?: unknown }
    if (typeof candidate.id !== 'number' || candidate.id <= 0) {
      continue
    }

    result.push({
      id: candidate.id,
      name: typeof candidate.name === 'string' ? candidate.name : ''
    })
  }

  return result
}

export function toSafeTableImages(value: unknown): TableImageResponse[] {
  if (!Array.isArray(value)) {
    return []
  }

  const result: TableImageResponse[] = []
  for (const item of value) {
    if (!item || typeof item !== 'object') {
      continue
    }

    const candidate = item as TableImageResponse
    const normalizedImageUrl = getPublicImageUrl(typeof candidate.image_url === 'string' ? candidate.image_url : '')
    if (!normalizedImageUrl) {
      continue
    }

    result.push({
      id: typeof candidate.id === 'number' ? candidate.id : undefined,
      table_id: typeof candidate.table_id === 'number' ? candidate.table_id : undefined,
      image_url: normalizedImageUrl,
      sort_order: typeof candidate.sort_order === 'number' ? candidate.sort_order : undefined,
      is_primary: !!candidate.is_primary
    })
  }

  return result
}

export function toSafeUploadFiles(value: unknown): TableUploadFile[] {
  if (!Array.isArray(value)) {
    return []
  }

  const result: TableUploadFile[] = []
  for (const item of value) {
    if (!item || typeof item !== 'object') {
      continue
    }

    const candidate = item as TableUploadFile
    if (typeof candidate.url !== 'string' || !candidate.url) {
      continue
    }

    result.push({
      url: candidate.url,
      status: candidate.status,
      mediaId: typeof candidate.mediaId === 'number' ? candidate.mediaId : undefined,
      localPath: typeof candidate.localPath === 'string' ? candidate.localPath : undefined
    })
  }

  return result
}

function normalizeTableType(tableType?: string): 'table' | 'room' {
  return tableType === 'room' ? 'room' : 'table'
}

function formatCurrencyFromCents(value?: number): string {
  if (typeof value !== 'number' || value <= 0) {
    return ''
  }

  return `¥${(value / 100).toFixed(2)}`
}

function buildTableTypeLabel(tableType: 'table' | 'room'): string {
  return tableType === 'room' ? '包间' : '大厅桌台'
}

function buildTableTypeDescription(tableType: 'table' | 'room'): string {
  return tableType === 'room' ? '适合预订与小聚' : '适合堂食与翻台'
}

export function buildTableTabLabel(tab: TableTypeFilterKey): string {
  if (tab === 'table') {
    return '普通桌台'
  }
  if (tab === 'room') {
    return '包间'
  }
  return '全部桌台'
}

export function buildTableStatusLabel(status: TableStatusFilterKey): string {
  if (status === 'all') {
    return '全部状态'
  }
  return getTableStatusDisplay(status).label
}

export function formatTableView(table: TableResponse): TableListItem {
  const normalizedTableType = normalizeTableType(table.table_type)
  const statusInfo = getTableStatusDisplay(table.status)
  const visibleTags = ensureArray(table.tags)
    .map((tag) => tag?.name || '')
    .filter((name) => !!name)
    .slice(0, 3)

  return {
    ...table,
    normalizedTableType,
    status: statusInfo.normalizedStatus,
    statusLabel: statusInfo.label,
    statusTheme: statusInfo.theme,
    canRelease: statusInfo.canRelease,
    canShowCode: statusInfo.canShowCode,
    isAvailableLike: statusInfo.isAvailableLike,
    typeLabel: buildTableTypeLabel(normalizedTableType),
    typeDescription: buildTableTypeDescription(normalizedTableType),
    capacityText: `${Math.max(1, Number(table.capacity) || 1)} 人位`,
    minimumSpendText: formatCurrencyFromCents(table.minimum_spend),
    descriptionText: (table.description || '').trim() || '可用于日常接待与堂食安排',
    visibleTags,
    tagSummaryText: visibleTags.length ? visibleTags.join(' / ') : '未配置桌台标签',
    qrCodeUrl: normalizeQRCodeUrl(table.qr_code_url),
    coverImageUrl: ''
  }
}

export function buildTableTabOptions(loadedTables: TableListItem[]): TableTabOption[] {
  const total = loadedTables.length
  const tableCount = loadedTables.filter((item) => item.normalizedTableType === 'table').length
  const roomCount = loadedTables.filter((item) => item.normalizedTableType === 'room').length

  return [
    { label: `全部 ${total}`, value: 'all', count: total },
    { label: `普通 ${tableCount}`, value: 'table', count: tableCount },
    { label: `包间 ${roomCount}`, value: 'room', count: roomCount }
  ]
}

export function buildTableSummaryMetrics(loadedTables: TableListItem[]): TableSummaryMetric[] {
  const total = loadedTables.length
  const availableCount = loadedTables.filter((item) => item.isAvailableLike).length
  const occupiedCount = loadedTables.filter((item) => item.canRelease).length
  const roomCount = loadedTables.filter((item) => item.normalizedTableType === 'room').length

  return [
    { id: 'total', label: '桌台总数', value: String(total), note: '含大厅与包间' },
    { id: 'available', label: '当前空闲', value: String(availableCount), note: '可直接安排入座' },
    { id: 'occupied', label: '占用 / 预订', value: String(occupiedCount), note: '建议及时跟进翻台' },
    { id: 'room', label: '包间数量', value: String(roomCount), note: '适合预订与包厢场景' }
  ]
}

function buildResultSummaryText(params: {
  visibleCount: number
  currentTab: TableTypeFilterKey
  currentStatus: TableStatusFilterKey
}): string {
  const activeFilters: string[] = []
  if (params.currentTab !== 'all') {
    activeFilters.push(params.currentTab === 'room' ? '包间' : '普通桌台')
  }
  if (params.currentStatus !== 'all') {
    activeFilters.push(buildTableStatusLabel(params.currentStatus))
  }

  if (activeFilters.length > 0) {
    return `${activeFilters.join(' / ')}下共 ${params.visibleCount} 项`
  }

  return `当前共 ${params.visibleCount} 项桌台与包间`
}

function buildEmptyDescription(params: {
  currentTab: TableTypeFilterKey
  currentStatus: TableStatusFilterKey
}): string {
  if (params.currentTab !== 'all' || params.currentStatus !== 'all') {
    return '暂无符合当前筛选条件的桌台'
  }

  return '还没有桌台或包间，先新增一个'
}

export function buildTablePresentationState(params: {
  loadedTables: TableListItem[]
  currentTab: TableTypeFilterKey
  currentStatus: TableStatusFilterKey
}) {
  const filteredTables = params.loadedTables.filter((item) => {
    const matchesType = params.currentTab === 'all' || item.normalizedTableType === params.currentTab
    const matchesStatus = params.currentStatus === 'all' || item.status === params.currentStatus
    return matchesType && matchesStatus
  })

  return {
    tables: filteredTables,
    tabOptions: buildTableTabOptions(params.loadedTables),
    resultSummaryText: buildResultSummaryText({
      visibleCount: filteredTables.length,
      currentTab: params.currentTab,
      currentStatus: params.currentStatus
    }),
    emptyDescription: buildEmptyDescription({
      currentTab: params.currentTab,
      currentStatus: params.currentStatus
    })
  }
}

export function normalizeQRCodeUrl(path?: string): string {
  if (!path) {
    return ''
  }

  return getPublicImageUrl(path)
}

export function normalizeTableBusinessStatus(status?: string): TableStatus {
  return getTableStatusDisplay(status).normalizedStatus as TableStatus
}

function getMiniProgramErrorMessage(error: unknown): string {
  if (typeof error === 'string') {
    return error
  }
  if (error && typeof error === 'object' && typeof (error as { errMsg?: unknown }).errMsg === 'string') {
    return (error as { errMsg: string }).errMsg
  }
  if (error instanceof Error) {
    return error.message
  }
  return ''
}

export function isPermissionDeniedError(error: unknown): boolean {
  const message = getMiniProgramErrorMessage(error)
  return message.includes('auth deny') || message.includes('auth denied')
}

export function isUserCancelledError(error: unknown): boolean {
  return getMiniProgramErrorMessage(error).includes('cancel')
}

export async function downloadRemoteImageToAlbum(imageUrl: string) {
  const downloadResult = await new Promise<WechatMiniprogram.DownloadFileSuccessCallbackResult>((resolve, reject) => {
    wx.downloadFile({
      url: imageUrl,
      success: (res) => {
        if (res.statusCode >= 200 && res.statusCode < 300 && res.tempFilePath) {
          resolve(res)
          return
        }

        reject(new Error('download failed'))
      },
      fail: reject
    })
  })

  await new Promise<void>((resolve, reject) => {
    wx.saveImageToPhotosAlbum({
      filePath: downloadResult.tempFilePath,
      success: () => resolve(),
      fail: reject
    })
  })
}
