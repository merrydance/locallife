type ToastOptions = WechatMiniprogram.ShowToastOption
type ModalOptions = WechatMiniprogram.ShowModalOption

interface PromptState {
  signature: string
  timestamp: number
}

const TOAST_DEDUP_MS = 1500
const MODAL_DEDUP_MS = 800

let lastToast: PromptState = { signature: '', timestamp: 0 }
let lastModal: PromptState = { signature: '', timestamp: 0 }
let guardsInstalled = false

function normalizePromptText(text: unknown, fallback: string): string {
  if (typeof text !== 'string') {
    return fallback
  }

  const normalized = text.replace(/\s+/g, ' ').trim()
  return normalized || fallback
}

function shouldSuppress(state: PromptState, signature: string, dedupMs: number): boolean {
  return state.signature === signature && Date.now() - state.timestamp < dedupMs
}

export function installPromptFeedbackGuards(): void {
  if (guardsInstalled) {
    return
  }

  guardsInstalled = true

  const rawShowToast = wx.showToast.bind(wx)
  const rawShowModal = wx.showModal.bind(wx)

  wx.showToast = ((options: ToastOptions) => {
    const title = normalizePromptText(options?.title, '请稍后再试')
    const icon = options?.icon || 'success'
    const signature = `${icon}:${title}`

    if (shouldSuppress(lastToast, signature, TOAST_DEDUP_MS)) {
      return undefined as ReturnType<typeof wx.showToast>
    }

    lastToast = { signature, timestamp: Date.now() }
    return rawShowToast({ ...options, title })
  }) as typeof wx.showToast

  wx.showModal = ((options: ModalOptions) => {
    const title = normalizePromptText(options?.title, '提示')
    const shouldKeepEditableContentEmpty = !!options?.editable && typeof options?.content !== 'string'
    const content = shouldKeepEditableContentEmpty ? undefined : normalizePromptText(options?.content, '请稍后再试')
    const signature = `${title}:${content || ''}`

    wx.hideToast()

    if (shouldSuppress(lastModal, signature, MODAL_DEDUP_MS)) {
      return undefined as ReturnType<typeof wx.showModal>
    }

    lastModal = { signature, timestamp: Date.now() }

    return rawShowModal({
      ...options,
      title,
      ...(content === undefined ? {} : { content })
    })
  }) as typeof wx.showModal
}
