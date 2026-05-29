Component({
  properties: {
    rankingType: {
      type: String,
      value: 'merchant'
    },
    timeLabel: {
      type: String,
      value: '今日排行'
    },
    merchantRankings: {
      type: Array,
      value: []
    },
    riderRankings: {
      type: Array,
      value: []
    }
  },

  methods: {
    onTypeChange(e: WechatMiniprogram.CustomEvent<{ value: 'merchant' | 'rider' }>) {
      this.triggerEvent('typechange', e.detail)
    }
  }
})