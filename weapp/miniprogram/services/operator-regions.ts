import { operatorBasicManagementService, type RegionResponse } from '../api/operator-basic-management'

export interface ConsolePickerOption {
  label: string
  value: string
}

export interface ConsoleRegionOption {
  id: number
  name: string
}

export interface ConsoleRegionPickerState {
  regions: ConsoleRegionOption[]
  regionPickerOptions: ConsolePickerOption[]
  regionPickerVisible: boolean
  selectedRegionIdx: number
  selectedRegionId: number
  selectedRegionValue: string
}

function buildRegionPickerState(regions: ConsoleRegionOption[]): ConsoleRegionPickerState {
  return {
    regions,
    regionPickerOptions: regions.map((item) => ({ label: item.name, value: String(item.id) })),
    regionPickerVisible: false,
    selectedRegionIdx: 0,
    selectedRegionId: regions[0]?.id || 0,
    selectedRegionValue: String(regions[0]?.id || '')
  }
}

function mapRegions(source: RegionResponse[]): ConsoleRegionOption[] {
  return (source || []).map((item) => ({ id: item.id, name: item.name }))
}

export async function loadOperatorRegions(): Promise<ConsoleRegionPickerState> {
  try {
    const response = await operatorBasicManagementService.getOperatorRegions({ page: 1, limit: 100 })
    return buildRegionPickerState(mapRegions(response.regions || []))
  } catch (_error) {
    return buildRegionPickerState([])
  }
}