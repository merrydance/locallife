import { type TableResponse } from '../../_main_shared/api/table'
import { type TableImageResponse } from '../../../../api/table-device-management'
import {
  ensureArray,
  normalizeQRCodeUrl,
  normalizeTableBusinessStatus,
  TABLE_UPLOAD_FILE_STATUS,
  type TableFormData,
  type TableTagOption,
  type TableUploadFile
} from '../../_utils/merchant-tables-shared'

export interface TableEditPageOptions {
  id?: string
}

export interface TableQRCodeContext {
  tableNo: string
  qrCodeUrl: string
}

export interface FormInputDetail {
  value: string
}

export type TableImageRole = 'cover' | 'gallery'

export function buildSelectedTagState(tagIds: number[]): Record<string, boolean> {
  return tagIds.reduce<Record<string, boolean>>((result, id) => {
    result[String(id)] = true
    return result
  }, {})
}

export function mergeSelectableTableTags(primaryTags: TableTagOption[], fallbackTags: TableTagOption[]): TableTagOption[] {
  const mergedTags: TableTagOption[] = []
  const seenTagIds = new Set<number>()

  for (const tag of [...primaryTags, ...fallbackTags]) {
    if (!tag || !Number.isFinite(tag.id) || tag.id <= 0) {
      continue
    }
    if (seenTagIds.has(tag.id)) {
      continue
    }
    seenTagIds.add(tag.id)
    mergedTags.push(tag)
  }

  return mergedTags
}

export function hasTableTagName(tags: TableTagOption[], name: string): boolean {
  const normalizedName = name.trim()
  return ensureArray(tags).some((tag) => (tag.name || '').trim() === normalizedName)
}

export function removeWarningMessageSegment(source: string, target: string): string {
  return source
    .split('；')
    .map((item) => item.trim())
    .filter((item) => item && item !== target)
    .join('；')
}

function mapTableImageToUploadFile(image: TableImageResponse): TableUploadFile | null {
  if (typeof image.image_url !== 'string' || !image.image_url) {
    return null
  }

  return {
    url: image.image_url,
    status: TABLE_UPLOAD_FILE_STATUS.done,
    mediaId: typeof image.media_asset_id === 'number' ? image.media_asset_id : undefined,
    imageId: typeof image.id === 'number' ? image.id : undefined,
    isPersisted: true
  }
}

export function splitTableImageFiles(tableImages: TableImageResponse[]) {
  const coverFiles: TableUploadFile[] = []
  const galleryFiles: TableUploadFile[] = []

  for (const image of ensureArray(tableImages)) {
    const file = mapTableImageToUploadFile(image)
    if (!file) {
      continue
    }

    if (image.is_primary && coverFiles.length === 0) {
      coverFiles.push(file)
      continue
    }

    galleryFiles.push(file)
  }

  return { coverFiles, galleryFiles }
}

export function pickPendingBoundFiles(files: TableUploadFile[]): TableUploadFile[] {
  return ensureArray(files).filter((file) => typeof file.mediaId === 'number' && file.mediaId > 0 && !file.imageId)
}

export function buildPersistedUploadFile(savedImage: TableImageResponse | null | undefined, fallbackUrl: string, mediaId: number): TableUploadFile {
  return {
    url: (typeof savedImage?.image_url === 'string' && savedImage.image_url) ? savedImage.image_url : fallbackUrl,
    status: TABLE_UPLOAD_FILE_STATUS.done,
    mediaId,
    imageId: typeof savedImage?.id === 'number' ? savedImage.id : undefined,
    isPersisted: true
  }
}

export function mapTableDetailToFormData(table: TableResponse): TableFormData {
  const normalizedStatus = normalizeTableBusinessStatus(table.status)

  return {
    table_no: table.table_no || '',
    table_type: table.table_type === 'room' ? 'room' : 'table',
    capacity: typeof table.capacity === 'number' ? table.capacity : 4,
    description: table.description || '',
    minimum_spend_yuan: typeof table.minimum_spend === 'number' && table.minimum_spend > 0
      ? (table.minimum_spend / 100).toFixed(2)
      : '',
    status: normalizedStatus,
    tag_ids: ensureArray(table.tags)
      .map((tag: { id: number }) => Number(tag.id))
      .filter((id) => Number.isFinite(id) && id > 0)
  }
}

export function findUploadFileIndex(files: TableUploadFile[], localPath: string): number {
  return files.findIndex((file) => file.localPath === localPath)
}

export function replaceUploadFileAt(files: TableUploadFile[], index: number, file: TableUploadFile): TableUploadFile[] {
  if (index < 0 || index >= files.length) {
    return files
  }

  const nextFiles = [...files]
  nextFiles[index] = file
  return nextFiles
}

export function removeUploadFileAt(files: TableUploadFile[], index: number): TableUploadFile[] {
  if (index < 0 || index >= files.length) {
    return files
  }

  const nextFiles = [...files]
  nextFiles.splice(index, 1)
  return nextFiles
}

export function getTableUploadingKey(field: 'cover' | 'gallery' | 'license' | 'foodPermit' | 'idCardFront' | 'idCardBack') {
  switch (field) {
    case 'license':
      return 'licenseUploading'
    case 'foodPermit':
      return 'foodPermitUploading'
    case 'idCardFront':
      return 'idCardFrontUploading'
    case 'gallery':
      return 'imageUploading'
    default:
      return field === 'cover' ? 'imageUploading' : 'idCardBackUploading'
  }
}

export function buildTableQRCodeContext(formData: TableFormData, qrCodeTableNo: string, qrCodeImageUrl: string): TableQRCodeContext {
  return {
    tableNo: formData.table_no || qrCodeTableNo || '',
    qrCodeUrl: normalizeQRCodeUrl(qrCodeImageUrl)
  }
}

export function validateTableBeforeSubmit(formData: TableFormData, uploadFiles: TableUploadFile[]) {
  const tableNo = (formData.table_no || '').trim()

  if (!tableNo) {
    return '请填写桌号或包间名'
  }

  if (!Number.isInteger(formData.capacity) || formData.capacity < 1 || formData.capacity > 100) {
    return '人数需在 1 到 100 之间'
  }

  if (uploadFiles.some((file) => file.status === TABLE_UPLOAD_FILE_STATUS.loading)) {
    return '图片仍在上传中，请稍候'
  }

  if (uploadFiles.some((file) => file.status === TABLE_UPLOAD_FILE_STATUS.failed)) {
    return '有图片上传失败，请删除后重试'
  }

  if (formData.minimum_spend_yuan && formData.minimum_spend_yuan.trim()) {
    const parsed = Number(formData.minimum_spend_yuan)
    if (!Number.isFinite(parsed) || parsed < 0) {
      return '最低消费金额不合法'
    }
  }

  return ''
}

export function buildTableSubmitPayload(formData: TableFormData) {
  const minimumSpend = formData.minimum_spend_yuan && formData.minimum_spend_yuan.trim()
    ? Math.round(Number(formData.minimum_spend_yuan) * 100)
    : undefined

  return {
    table_no: (formData.table_no || '').trim(),
    table_type: formData.table_type,
    capacity: formData.capacity,
    description: (formData.description || '').trim() || undefined,
    minimum_spend: minimumSpend,
    tag_ids: ensureArray(formData.tag_ids)
  }
}