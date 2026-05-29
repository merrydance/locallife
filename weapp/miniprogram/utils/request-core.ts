export function sanitizeGetParams(data: unknown): unknown {
  if (!data || typeof data !== 'object' || Array.isArray(data)) {
    return data
  }

  const source = data as Record<string, unknown>
  const cleaned: Record<string, unknown> = {}

  Object.entries(source).forEach(([key, value]) => {
    if (value === undefined || value === null) {
      return
    }

    if (typeof value === 'string') {
      const trimmed = value.trim()
      const lower = trimmed.toLowerCase()
      if (!trimmed || lower === 'undefined' || lower === 'null' || lower === 'nan') {
        return
      }
      cleaned[key] = trimmed
      return
    }

    if (typeof value === 'number' && !Number.isFinite(value)) {
      return
    }

    cleaned[key] = value
  })

  return cleaned
}

export function buildGetSingleFlightKey(url: string, data: unknown, skipAuth: boolean): string {
  let serialized = ''
  try {
    serialized = JSON.stringify(data || {})
  } catch (_error) {
    serialized = String(data)
  }
  return `GET|${url}|${serialized}|skipAuth:${skipAuth ? '1' : '0'}`
}

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}
