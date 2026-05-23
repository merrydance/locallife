let requestIdSequence = 0

export function buildDefaultRequestId(method: string, url: string): string {
  requestIdSequence = (requestIdSequence + 1) % Number.MAX_SAFE_INTEGER
  return `${method}_${url}_${Date.now()}_${requestIdSequence}`
}
