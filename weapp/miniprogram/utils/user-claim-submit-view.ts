export type SubmitResultPresentation = {
  icon: string
  color: string
  title: string
  summary: string
}

export function getSubmitResultPresentation(result: {
  payout_status?: string
  decision_status?: string
}): SubmitResultPresentation {
  if (result.payout_status === 'paid') {
    return {
      icon: 'check-circle-filled',
      color: '#2e7d32',
      title: '赔付已到账',
      summary: '平台已受理并完成自动裁定，赔付已到账。'
    }
  }

  if (result.decision_status === 'auto-adjudicated') {
    return {
      icon: 'check-circle-filled',
      color: '#1976d2',
      title: '已自动裁定',
      summary: '平台已受理并完成自动裁定，赔付正在处理中。'
    }
  }

  return {
    icon: 'time-filled',
    color: '#1976d2',
    title: '平台已受理',
    summary: '平台已受理您的反馈，正在为您处理。'
  }
}