interface SupportChannel {
  id: string
  title: string
  subtitle: string
  action: string
  type: 'phone' | 'chat' | 'ticket'
  value: string
}

interface SupportFaq {
  id: string
  question: string
  answer: string
}

Page({
  data: {
    navBarHeight: 88,
    channels: [
      {
        id: 'food-safety',
        title: '食品安全事件上报',
        subtitle: '三起关联事件，停止商户接单',
        action: '立即上报',
        type: 'ticket',
        value: 'food-safety-report'
      },
      {
        id: 'foreign-object',
        title: '投诉餐厅',
        subtitle: '对餐厅有什么不满意可以提',
        action: '投诉餐厅',
        type: 'ticket',
        value: 'restaurant-complaint'
      },
      {
        id: 'rider-complaint',
        title: '骑手投诉',
        subtitle: '骑手小哥很辛苦，真惹毛了你，也可以投诉',
        action: '投诉骑手',
        type: 'ticket',
        value: 'rider-complaint'
      }
    ] as SupportChannel[],
    faqs: [
      {
        id: 'order',
        question: '订单异常如何快速处理？',
        answer: '可直接上传订单号，客服会实时共屏协助。'
      },
      {
        id: 'invoice',
        question: '需要报销/开票怎么办？',
        answer: '支持线上开票，客服可代填抬头信息。'
      },
      {
        id: 'delivery',
        question: '催单或改地址？',
        answer: '通过热线或专属客服可即时修改派送指令。'
      }
    ] as SupportFaq[]
  },

  onChannelTap(event: WechatMiniprogram.BaseEvent) {
    const channel = event.currentTarget.dataset.channel as SupportChannel
    if (!channel) return

    if (channel.type === 'phone') {
      wx.makePhoneCall({ phoneNumber: channel.value })
      return
    }

    wx.showToast({ title: `${channel.action} · ${channel.title}`, icon: 'none' })
  },

  onFaqTap(event: WechatMiniprogram.BaseEvent) {
    const faq = event.currentTarget.dataset.faq as SupportFaq
    if (!faq) return

    wx.showModal({
      title: faq.question,
      content: faq.answer,
      showCancel: false
    })
  },

  onNavHeight(event: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: event.detail.navBarHeight })
  }
})
