export type StatusTagTone = 'neutral' | 'info' | 'success' | 'warning' | 'danger'

export type StatusTagTheme = 'default' | 'primary' | 'success' | 'warning' | 'danger'

const STATUS_TAG_THEME_MAP: Record<StatusTagTone, StatusTagTheme> = {
  neutral: 'default',
  info: 'primary',
  success: 'success',
  warning: 'warning',
  danger: 'danger'
}

export function resolveStatusTagTheme(tone: StatusTagTone): StatusTagTheme {
  return STATUS_TAG_THEME_MAP[tone]
}

export function buildStatusTagView(label: string, tone: StatusTagTone) {
  return {
    label,
    tone,
    theme: resolveStatusTagTheme(tone)
  }
}