export type FulfilledResult<T> = {
  status: 'fulfilled'
  value: T
}

export type RejectedResult = {
  status: 'rejected'
  reason: unknown
}

export type SettledResult<T> = FulfilledResult<T> | RejectedResult

export function settleAll<T extends readonly unknown[]>(
  promises: { [K in keyof T]: Promise<T[K]> }
): Promise<{ [K in keyof T]: SettledResult<T[K]> }>
export function settleAll<T>(promises: Promise<T>[]): Promise<Array<SettledResult<T>>>
export function settleAll<T>(promises: Promise<T>[]) {
  return Promise.all(
    promises.map((promise) => promise.then(
      (value) => ({ status: 'fulfilled' as const, value }),
      (reason) => ({ status: 'rejected' as const, reason })
    ))
  )
}