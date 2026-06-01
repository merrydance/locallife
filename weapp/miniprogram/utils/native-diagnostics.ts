import { logger } from './logger'

export interface NativeOperationSnapshot {
  name: string
  phase: 'start' | 'success' | 'fail' | 'timeout'
  startedAt: number
  updatedAt: number
  durationMs?: number
  detail?: unknown
}

const MAX_NATIVE_OPERATION_HISTORY = 12

let nativeOperationHistory: NativeOperationSnapshot[] = []

function pushNativeOperation(snapshot: NativeOperationSnapshot) {
  nativeOperationHistory.push(snapshot)
  if (nativeOperationHistory.length > MAX_NATIVE_OPERATION_HISTORY) {
    nativeOperationHistory = nativeOperationHistory.slice(-MAX_NATIVE_OPERATION_HISTORY)
  }
}

export function markNativeOperationStart(
  name: string,
  detail?: unknown
): (phase?: NativeOperationSnapshot['phase'], finishDetail?: unknown) => void {
  const startedAt = Date.now()
  pushNativeOperation({
    name,
    phase: 'start',
    startedAt,
    updatedAt: startedAt,
    detail
  })
  logger.debug('原生调用开始', { name, detail }, 'nativeDiagnostics')

  return (phase: NativeOperationSnapshot['phase'] = 'success', finishDetail?: unknown) => {
    const updatedAt = Date.now()
    const snapshot = {
      name,
      phase,
      startedAt,
      updatedAt,
      durationMs: updatedAt - startedAt,
      detail: finishDetail
    }
    pushNativeOperation(snapshot)
    const level = phase === 'success' ? 'debug' : 'warn'
    logger[level]('原生调用结束', snapshot, 'nativeDiagnostics')
  }
}

export function getNativeOperationDiagnostics(): {
  latest?: NativeOperationSnapshot
  recent: NativeOperationSnapshot[]
} {
  const recent = nativeOperationHistory.slice(-MAX_NATIVE_OPERATION_HISTORY)
  return {
    latest: recent[recent.length - 1],
    recent
  }
}
