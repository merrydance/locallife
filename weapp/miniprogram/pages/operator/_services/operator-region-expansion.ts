import {
  applyRegionExpansion,
  getRegionExpansionStatusDisplay,
  listAvailableRegions,
  listRegionExpansionApplications,
  listRegions,
  type RegionExpansionApplication,
  type RegionExpansionStatusTheme
} from '../_main_shared/api/operator-application'

export interface OperatorRegionExpansionCityOption {
  label: string
  value: number
}

export interface OperatorRegionExpansionRegionOption {
  label: string
  secondary: string
  value: number
}

export interface OperatorRegionExpansionApplicationView extends RegionExpansionApplication {
  status_label: string
  status_theme: RegionExpansionStatusTheme
  is_rejected: boolean
}

export interface OperatorRegionExpansionCityState {
  cityOptions: OperatorRegionExpansionCityOption[]
  selectedCityId: number
  selectedCityName: string
}

export interface OperatorRegionExpansionRegionState {
  regionOptions: OperatorRegionExpansionRegionOption[]
  filteredRegions: OperatorRegionExpansionRegionOption[]
  regionKeyword: string
}

function formatDate(iso: string): string {
  try {
    const date = new Date(iso)
    const pad = (value: number) => String(value).padStart(2, '0')
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
  } catch {
    return iso
  }
}

export async function loadOperatorRegionExpansionApplications(): Promise<OperatorRegionExpansionApplicationView[]> {
  const response = await listRegionExpansionApplications()
  return (response.applications || []).map((item) => {
    const statusDisplay = getRegionExpansionStatusDisplay(item.status)
    return {
      ...item,
      created_at: formatDate(item.created_at),
      status_label: statusDisplay.label,
      status_theme: statusDisplay.theme,
      is_rejected: statusDisplay.isRejected
    }
  })
}

export async function loadOperatorRegionExpansionCityState(): Promise<OperatorRegionExpansionCityState> {
  const cityOptions: OperatorRegionExpansionCityOption[] = []
  let pageId = 1

  for (;;) {
    const items = await listRegions({ page_id: pageId, page_size: 100, level: 2 })
    if (!items || items.length === 0) {
      break
    }
    items.forEach((item) => cityOptions.push({ label: item.name, value: item.id }))
    if (items.length < 100) {
      break
    }
    pageId += 1
  }

  return {
    cityOptions,
    selectedCityId: cityOptions[0]?.value || 0,
    selectedCityName: cityOptions[0]?.label || ''
  }
}

export async function loadOperatorRegionExpansionRegionsByCity(params: {
  cityId: number
  cityName: string
  keyword?: string
}): Promise<OperatorRegionExpansionRegionState> {
  const regionOptions: OperatorRegionExpansionRegionOption[] = []
  let pageId = 1

  for (;;) {
    const response = await listAvailableRegions({
      page_id: pageId,
      page_size: 100,
      level: 3,
      parent_id: params.cityId,
      keyword: params.keyword || undefined
    })
    const items = response.regions || []
    if (!items.length) {
      break
    }
    items.forEach((item) => regionOptions.push({
      label: item.name,
      secondary: item.parent_name || params.cityName,
      value: item.id
    }))
    if (items.length < 100) {
      break
    }
    pageId += 1
  }

  return {
    regionOptions,
    filteredRegions: regionOptions,
    regionKeyword: params.keyword || ''
  }
}

export async function submitOperatorRegionExpansion(regionId: number): Promise<void> {
  await applyRegionExpansion(regionId)
}