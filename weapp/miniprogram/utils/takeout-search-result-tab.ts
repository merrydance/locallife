export type TakeoutSearchResultTab = 'dishes' | 'merchants'

export function chooseTakeoutSearchResultTab(params: {
  dishCount: number
  merchantCount: number
}): TakeoutSearchResultTab {
  if (params.dishCount > 0) {
    return 'dishes'
  }
  if (params.merchantCount > 0) {
    return 'merchants'
  }
  return 'dishes'
}
