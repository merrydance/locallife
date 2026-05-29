import { getCurrentRegion } from '../../../../api/location'
import { listAvailableRegions, listRegions, type OperatorApplicationResponse } from '../../_main_shared/api/operator-application'
import { getPrivateMediaUrl } from '../../_main_shared/utils/image-security'
import { locationService, type LocationInfo } from '../../../../utils/location'
import {
  buildAvailableRegionsPatch,
  buildCityOptionsPatch,
  buildProvinceOptionsPatch,
  buildRegionFullName,
  CityOption,
  findMatchedProvinceOption,
  findMatchedCityOption,
  findMatchedDistrictOption,
  ProvinceOption,
  RegionOption,
  UploadFieldValue
} from './operator-registration-view'

export function handleExistingOperatorApplication(
  res: OperatorApplicationResponse,
  userRole: string | undefined,
  onRestartRejected: () => void
): number {
  if (res.status === 'submitted') {
    return 4
  }

  if (res.status === 'approved' && res.is_operator) {
    wx.showModal({
      title: '您已是运营商',
      content: '您的入驻已完成，请选择下一步操作',
      confirmText: '进入控制台',
      cancelText: '申请更多区域',
      success(result) {
        if (result.confirm) {
          wx.reLaunch({ url: '/pages/operator/dashboard/index' })
        } else if (result.cancel) {
          wx.reLaunch({ url: '/pages/operator/region-expansion/index' })
        }
      }
    })
    return 0
  }

  if (res.status === 'approved' && !res.is_operator) {
    wx.showModal({
      title: '审核通过',
      content: '资料审核已通过，但当前账号暂未获得运营身份，请联系平台处理后再进入运营商控制台。',
      showCancel: false,
      confirmText: '知道了'
    })
    return 0
  }

  if (res.status === 'rejected') {
    wx.showModal({
      title: '审核未通过',
      content: `原因：${res.reject_reason || '资料核验失败'}`,
      confirmText: '重新填写资料',
      cancelText: '稍后再说',
      success: (result) => {
        if (result.confirm) {
          onRestartRejected()
        }
      }
    })
    return 0
  }

  if (userRole === 'operator') {
    wx.redirectTo({ url: '/pages/operator/region-expansion/index' })
  }

  return 0
}

export async function fetchOperatorProvinceOptions(selectedProvinceId: number) {
  const provinces: ProvinceOption[] = []
  let pageID = 1

  for (;;) {
    const items = await listRegions({ page_id: pageID, page_size: 100, level: 1 })
    if (!items || items.length === 0) {
      break
    }

    items.forEach((item) => {
      provinces.push({
        label: item.name,
        value: item.id
      })
    })

    if (items.length < 100) {
      break
    }
    pageID += 1
  }

  const nextSelectedProvinceId = selectedProvinceId || (provinces[0]?.value || 0)
  return {
    selectedProvinceId: nextSelectedProvinceId,
    patch: buildProvinceOptionsPatch(provinces, nextSelectedProvinceId)
  }
}

export async function fetchOperatorCityOptions(provinceId: number, selectedCityId: number) {
  const cities: CityOption[] = []
  if (!provinceId) {
    return {
      selectedCityId: 0,
      patch: buildCityOptionsPatch([], 0)
    }
  }

  let pageID = 1

  for (;;) {
    const items = await listRegions({ page_id: pageID, page_size: 100, level: 2, parent_id: provinceId })
    if (!items || items.length === 0) {
      break
    }

    items.forEach((item) => {
      cities.push({
        label: item.name,
        value: item.id
      })
    })

    if (items.length < 100) {
      break
    }
    pageID += 1
  }

  const nextSelectedCityId = selectedCityId || (cities[0]?.value || 0)
  return {
    selectedCityId: nextSelectedCityId,
    patch: buildCityOptionsPatch(cities, nextSelectedCityId)
  }
}

export async function fetchOperatorAvailableRegionsByCity(cityID: number) {
  const districts: RegionOption[] = []
  let pageID = 1

  for (;;) {
    const res = await listAvailableRegions({
      page_id: pageID,
      page_size: 100,
      level: 3,
      parent_id: cityID
    })
    const regions = res?.regions || []
    if (regions.length === 0) {
      break
    }

    regions.forEach((region) => {
      districts.push({
        label: region.name,
        secondary: region.parent_name || '',
        value: region.id,
        parentId: region.parent_id
      })
    })

    if (regions.length < 100) {
      break
    }
    pageID += 1
  }

  return buildAvailableRegionsPatch(districts)
}

export async function getOperatorCurrentLocation(): Promise<LocationInfo | null> {
  const cached = locationService.getFromGlobal()
  if (cached?.city || cached?.district) {
    return cached
  }

  try {
    return await locationService.getLocationWithPermission()
  } catch {
    return null
  }
}

export async function getOperatorCurrentRegionByLocation(location: LocationInfo) {
  const latitude = Number(location.latitude)
  const longitude = Number(location.longitude)

  if (!Number.isFinite(latitude) || !Number.isFinite(longitude)) {
    return null
  }

  try {
    return await getCurrentRegion({ latitude, longitude })
  } catch {
    return null
  }
}

export async function resolveOperatorDefaultRegionPatch(params: {
  getProvinceOptions: () => ProvinceOption[]
  getCityOptions: () => CityOption[]
  getRegionOptions: () => RegionOption[]
  fetchProvinceOptions: (withCities?: boolean) => Promise<void>
  fetchCityOptions: (provinceId: number, withRegions?: boolean) => Promise<void>
  fetchAvailableRegionsByCity: (cityId: number) => Promise<void>
}) {
  const location = await getOperatorCurrentLocation()
  if (!location) {
    return null
  }

  const currentRegion = await getOperatorCurrentRegionByLocation(location)
  const provinceName = String(location.province || '').trim()
  const cityName = String(location.city || '').trim()
  const districtName = String(location.district || '').trim()
  if (!currentRegion && (!provinceName || !cityName || !districtName)) {
    return null
  }

  if (!params.getProvinceOptions().length) {
    await params.fetchProvinceOptions(false)
  }

  const matchedProvince = findMatchedProvinceOption(params.getProvinceOptions(), provinceName)
  if (!matchedProvince) {
    return null
  }

  await params.fetchCityOptions(matchedProvince.value, false)

  const getPickerPatch = (province: ProvinceOption, city: CityOption) => {
    const provinceOptions = params.getProvinceOptions()
    const provinceIndex = provinceOptions.findIndex((item) => item.value === province.value)
    const cityOptions = params.getCityOptions()
    const cityIndex = cityOptions.findIndex((item) => item.value === city.value)
    return {
      selectedProvinceIndex: Math.max(0, provinceIndex),
      selectedProvinceId: province.value,
      selectedProvinceName: province.label,
      selectedCityIndex: Math.max(0, cityIndex),
      selectedCityId: city.value,
      selectedCityName: city.label
    }
  }

  const matchedCity = findMatchedCityOption(
    params.getCityOptions(),
    String(currentRegion?.parent_name || cityName || '').trim()
  )
  if (!matchedCity) {
    return null
  }

  await params.fetchAvailableRegionsByCity(matchedCity.value)
  let matchedDistrict = params.getRegionOptions().find((item) => item.value === currentRegion?.region_id)
  if (!matchedDistrict && currentRegion?.region_name) {
    const fallbackDistrict = findMatchedDistrictOption(params.getRegionOptions(), currentRegion.region_name)
    if (fallbackDistrict) {
      matchedDistrict = fallbackDistrict
    }
  }
  if (!matchedDistrict) {
    const fallbackDistrict = findMatchedDistrictOption(params.getRegionOptions(), districtName)
    if (fallbackDistrict) {
      matchedDistrict = fallbackDistrict
    }
  }
  if (!matchedDistrict) {
    return null
  }

  return {
    ...getPickerPatch(matchedProvince, matchedCity),
    selectedDistrictName: matchedDistrict.label,
    'formData.regionId': matchedDistrict.value,
    'formData.regionName': `${matchedCity.label} - ${matchedDistrict.label}`
  }
}

export async function resolveOperatorUploadPreviewURL(assetId?: number): Promise<string> {
  if (!assetId || assetId <= 0) {
    return ''
  }

  try {
    return await getPrivateMediaUrl(assetId)
  } catch {
    return ''
  }
}

export async function collectOperatorUploadPreviewPatch(params: {
  refreshVersion: number
  uploads: Array<{ key: 'license' | 'idFront' | 'idBack', value: UploadFieldValue }>
  getLatestUploadValue: (key: 'license' | 'idFront' | 'idBack') => UploadFieldValue | undefined
}) {
  const patch: Record<string, string> = {}

  for (const item of params.uploads) {
    const assetId = item.value?.assetId
    if (!assetId) {
      continue
    }

    const resolved = await resolveOperatorUploadPreviewURL(assetId)
    const latestValue = params.getLatestUploadValue(item.key)
    if (
      latestValue?.assetId === assetId
      && resolved
      && resolved !== latestValue?.url
    ) {
      patch[`${item.key}.url`] = resolved
    }
  }

  return patch
}

export function buildOperatorUploadStartPatch(field: 'license' | 'idFront' | 'idBack', path: string) {
  return {
    [`${field}.url`]: path,
    [`${field}.rawUrl`]: path
  }
}

export function buildRegionNamePatch(regionOptions: RegionOption[], regionId: number | string) {
  const id = Number(regionId)
  const matched = regionOptions.find((region) => Number(region.value) === id)
  if (!matched) {
    return null
  }

  return {
    selectedDistrictName: matched.label,
    'formData.regionName': buildRegionFullName(matched)
  }
}