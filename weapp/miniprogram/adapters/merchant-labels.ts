export function buildMerchantDisplayTags(
  systemLabels: string[] = [],
  categoryTags: string[] = [],
  limit?: number
): string[] {
  const merged: string[] = []

  const pushUnique = (labels: string[]) => {
    labels.forEach((label) => {
      const normalized = label?.trim()
      if (!normalized || merged.includes(normalized)) {
        return
      }

      merged.push(normalized)
    })
  }

  pushUnique(systemLabels)
  pushUnique(categoryTags)

  if (typeof limit === 'number' && limit >= 0) {
    return merged.slice(0, limit)
  }

  return merged
}