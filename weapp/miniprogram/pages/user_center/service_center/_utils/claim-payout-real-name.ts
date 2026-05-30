export const CLAIM_PAYOUT_REAL_NAME_REQUIRED_CODE = 40978

const DEFAULT_WECHAT_USER_NAME = '微信用户'

export function isClaimPayoutRealNameReady(fullName: string | undefined | null): boolean {
  const name = String(fullName || '').trim()
  if (!name) return false
  if (name === DEFAULT_WECHAT_USER_NAME) return false
  if (name.toLowerCase().startsWith('user ')) return false
  return Array.from(name).length >= 2
}

export function isClaimPayoutRealNameRequiredError(error: unknown): boolean {
  const known = error as { code?: unknown, statusCode?: unknown, message?: unknown, userMessage?: unknown }
  return known.code === CLAIM_PAYOUT_REAL_NAME_REQUIRED_CODE
    || (known.statusCode === 409 && String(known.message || known.userMessage || '').includes('微信实名'))
}

export function promptClaimPayoutRealName(): Promise<string> {
  return new Promise((resolve) => {
    wx.showModal({
      title: '确认收款姓名',
      content: '请填写与微信实名一致的姓名',
      editable: true,
      placeholderText: '例：张三',
      confirmText: '确认',
      success: (res) => {
        if (!res.confirm) {
          resolve('')
          return
        }
        resolve(String(res.content || '').trim())
      },
      fail: () => resolve('')
    })
  })
}
