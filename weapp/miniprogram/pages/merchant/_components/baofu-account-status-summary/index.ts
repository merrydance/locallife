Component({
  properties: {
    statusText: {
      type: String,
      value: ''
    },
    statusDesc: {
      type: String,
      value: ''
    },
    nextActionText: {
      type: String,
      value: ''
    },
    tagTheme: {
      type: String,
      value: 'default'
    },
    verifyFeePrompt: {
      type: String,
      value: ''
    },
    showVerifyFeePrompt: {
      type: Boolean,
      value: false
    },
    showOpenReportHint: {
      type: Boolean,
      value: false
    },
    isReady: {
      type: Boolean,
      value: false
    },
    feedbackTitle: {
      type: String,
      value: '开户状态'
    },
    reasonTitle: {
      type: String,
      value: '状态说明'
    },
    nextStepTitle: {
      type: String,
      value: '下一步'
    },
    statusIcon: {
      type: String,
      value: 'info-circle'
    }
  }
})
