export type EntryParams = {
  merchant_id?: number
  table_no?: string
  table_id?: number
}

export function parseScene(scene: string): EntryParams | null {
  let decoded: string
  try {
    decoded = decodeURIComponent(scene)
  } catch (_error) {
    return null
  }

  const tableIdMatch = decoded.match(/^tid_(\d+)$/)
  if (tableIdMatch) {
    return { table_id: parseInt(tableIdMatch[1], 10) }
  }

  const merchantTableMatch = decoded.match(/^m_(\d+)-t_(.+)$/)
  if (merchantTableMatch) {
    return {
      merchant_id: parseInt(merchantTableMatch[1], 10),
      table_no: merchantTableMatch[2]
    }
  }

  const legacyTableIdMatch = decoded.match(/^(?:table_|t)?(\d+)$/)
  if (legacyTableIdMatch) {
    return { table_id: parseInt(legacyTableIdMatch[1], 10) }
  }

  return null
}

export function parseQrUrl(url: string): EntryParams | null {
  const urlObj = new URL(url)
  const tableId = urlObj.searchParams.get('table_id')
  if (tableId && !Number.isNaN(parseInt(tableId, 10))) {
    return { table_id: parseInt(tableId, 10) }
  }

  const merchantId = urlObj.searchParams.get('merchant_id')
  const tableNo = urlObj.searchParams.get('table_no')
  if (merchantId && tableNo && !Number.isNaN(parseInt(merchantId, 10))) {
    return {
      merchant_id: parseInt(merchantId, 10),
      table_no: tableNo
    }
  }

  const pathTableId = urlObj.pathname.match(/\/table\/(\d+)/)?.[1]
  if (pathTableId && !Number.isNaN(parseInt(pathTableId, 10))) {
    return { table_id: parseInt(pathTableId, 10) }
  }

  return null
}
