import type { ApplymentBankOption } from '../../api/applyment-bank'

export type ApplymentRecognizedBankSelection<T extends ApplymentBankOption> = {
  bank: T | null
  filteredBanks: T[]
  selectedBankIndex: number
  shouldOpenPicker: boolean
}

export function resolveRecognizedBankSelection<T extends ApplymentBankOption>(
  matches: T[],
  limit = 100
): ApplymentRecognizedBankSelection<T> {
  const filteredBanks = matches.slice(0, limit)
  const bank = filteredBanks[0] || null
  return {
    bank,
    filteredBanks,
    selectedBankIndex: 0,
    shouldOpenPicker: filteredBanks.length > 1
  }
}
