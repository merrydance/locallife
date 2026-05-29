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
    profileHint: {
      type: String,
      value: ''
    },
    showOpenReportHint: {
      type: Boolean,
      value: false
    },
    isReady: {
      type: Boolean,
      value: false
    }
  }
})
