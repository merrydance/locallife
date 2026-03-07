/**
 * 音频播报工具
 * 用于商户收到新订单时播放语音提醒
 *
 * 音频文件位于 assets/audio/new_order.mp3
 * 如需更换语音内容，用 TTS 工具重新生成并替换该文件即可
 */

let _ctx: WechatMiniprogram.InnerAudioContext | null = null

function getAudioContext(): WechatMiniprogram.InnerAudioContext {
  if (!_ctx || _ctx.src === '') {
    _ctx = wx.createInnerAudioContext()
    _ctx.src = '/assets/audio/new_order.mp3'
    _ctx.volume = 1.0
    _ctx.loop = false
    _ctx.onError((err) => {
      console.error('[audio-alert] 播放失败', err)
    })
  }
  return _ctx
}

/**
 * 播放新订单语音提醒
 * 若正在播放则先停止再播放（防止连续订单叠音）
 */
export function playNewOrderAlert(): void {
  try {
    const ctx = getAudioContext()
    ctx.stop()
    // 微小延迟确保 stop 完成后再 play
    setTimeout(() => {
      try {
        ctx.play()
      } catch (e) {
        console.error('[audio-alert] play error', e)
      }
    }, 50)
  } catch (e) {
    console.error('[audio-alert] init error', e)
  }
}

/**
 * 销毁音频上下文（页面卸载时调用，避免内存泄漏）
 */
export function destroyAudioAlert(): void {
  if (_ctx) {
    try {
      _ctx.destroy()
    } catch (_) {}
    _ctx = null
  }
}
