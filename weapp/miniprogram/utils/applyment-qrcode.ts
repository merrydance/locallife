import { Ecc, QrCode } from '../miniprogram_npm/tdesign-miniprogram/common/shared/qrcode/qrcodegen'

const POSTER_WIDTH = 720
const POSTER_HEIGHT = 960
const QR_CODE_SIZE = 420
const QR_CODE_MARGIN_MODULES = 4

interface SaveApplymentQRCodePosterParams {
  page: WechatMiniprogram.Page.TrivialInstance
  canvasSelector: string
  value: string
  title: string
  subtitle: string
}

type CanvasNodeLike = {
  width: number
  height: number
  getContext: (contextType: '2d') => CanvasRenderingContext2D
}

function getCanvasNode(
  page: WechatMiniprogram.Page.TrivialInstance,
  selector: string
): Promise<CanvasNodeLike> {
  return new Promise((resolve, reject) => {
    wx.createSelectorQuery()
      .in(page)
      .select(selector)
      .fields({ node: true, size: true })
      .exec((result) => {
        const canvasNode = result?.[0]?.node as CanvasNodeLike | undefined
        if (canvasNode) {
          resolve(canvasNode)
          return
        }
        reject(new Error('未找到二维码画布'))
      })
  })
}

function wrapText(
  context: CanvasRenderingContext2D,
  text: string,
  maxWidth: number,
  maxLines: number
): string[] {
  const glyphs = Array.from(String(text || '').trim())
  if (!glyphs.length) {
    return []
  }

  const lines: string[] = []
  let currentLine = ''

  for (const glyph of glyphs) {
    const nextLine = currentLine + glyph
    if (!currentLine || context.measureText(nextLine).width <= maxWidth) {
      currentLine = nextLine
      continue
    }

    lines.push(currentLine)
    currentLine = glyph

    if (lines.length === maxLines - 1) {
      break
    }
  }

  const remainingText = glyphs.slice(lines.join('').length).join('')
  const lastLine = lines.length === maxLines - 1 && remainingText
    ? remainingText
    : currentLine

  if (lastLine) {
    let fittedLine = lastLine
    while (context.measureText(fittedLine).width > maxWidth && fittedLine.length > 1) {
      fittedLine = fittedLine.slice(0, -1)
    }
    if (lines.length === maxLines - 1 && fittedLine !== lastLine) {
      fittedLine = `${fittedLine.slice(0, -1)}...`
    }
    lines.push(fittedLine)
  }

  return lines.slice(0, maxLines)
}

function drawMultilineText(
  context: CanvasRenderingContext2D,
  lines: string[],
  centerX: number,
  startY: number,
  lineHeight: number
) {
  lines.forEach((line, index) => {
    context.fillText(line, centerX, startY + (index * lineHeight))
  })
}

function drawQRCode(
  context: CanvasRenderingContext2D,
  value: string,
  x: number,
  y: number,
  size: number
) {
  const qrCode = QrCode.encodeText(value, Ecc.MEDIUM)
  const modules = qrCode.getModules()
  const totalCells = modules.length + (QR_CODE_MARGIN_MODULES * 2)
  const cellSize = size / totalCells

  context.fillStyle = '#FFFFFF'
  context.fillRect(x, y, size, size)

  context.fillStyle = '#111827'
  modules.forEach((row, rowIndex) => {
    row.forEach((filled, columnIndex) => {
      if (!filled) {
        return
      }

      const drawX = x + ((columnIndex + QR_CODE_MARGIN_MODULES) * cellSize)
      const drawY = y + ((rowIndex + QR_CODE_MARGIN_MODULES) * cellSize)
      context.fillRect(drawX, drawY, cellSize + 0.6, cellSize + 0.6)
    })
  })
}

function exportCanvasToTempFile(canvas: CanvasNodeLike): Promise<string> {
  return new Promise((resolve, reject) => {
    wx.canvasToTempFilePath({
      canvas,
      x: 0,
      y: 0,
      width: POSTER_WIDTH,
      height: POSTER_HEIGHT,
      destWidth: POSTER_WIDTH,
      destHeight: POSTER_HEIGHT,
      fileType: 'png',
      success: (result) => resolve(result.tempFilePath),
      fail: reject
    })
  })
}

function saveImageFileToAlbum(filePath: string): Promise<void> {
  return new Promise((resolve, reject) => {
    wx.saveImageToPhotosAlbum({
      filePath,
      success: () => resolve(),
      fail: reject
    })
  })
}

export async function saveApplymentQRCodePosterToAlbum(params: SaveApplymentQRCodePosterParams) {
  const canvas = await getCanvasNode(params.page, params.canvasSelector)
  const context = canvas.getContext('2d')
  const pixelRatio = wx.getWindowInfo().pixelRatio || 1

  canvas.width = POSTER_WIDTH * pixelRatio
  canvas.height = POSTER_HEIGHT * pixelRatio
  context.scale(pixelRatio, pixelRatio)

  context.fillStyle = '#F3F4F6'
  context.fillRect(0, 0, POSTER_WIDTH, POSTER_HEIGHT)

  context.fillStyle = '#FFFFFF'
  context.fillRect(36, 36, POSTER_WIDTH - 72, POSTER_HEIGHT - 72)

  context.fillStyle = '#111827'
  context.textAlign = 'center'
  context.textBaseline = 'top'
  context.font = '600 36px sans-serif'
  context.fillText(params.title, POSTER_WIDTH / 2, 96)

  context.fillStyle = '#4B5563'
  context.font = '28px sans-serif'
  const subtitleLines = wrapText(context, params.subtitle, POSTER_WIDTH - 160, 3)
  drawMultilineText(context, subtitleLines, POSTER_WIDTH / 2, 156, 40)

  const qrY = 320
  drawQRCode(context, params.value, (POSTER_WIDTH - QR_CODE_SIZE) / 2, qrY, QR_CODE_SIZE)

  context.fillStyle = '#6B7280'
  context.font = '26px sans-serif'
  context.fillText('请使用微信扫一扫继续完成当前步骤', POSTER_WIDTH / 2, qrY + QR_CODE_SIZE + 52)
  context.fillText('如当前手机即为操作微信，可先保存到相册后从微信扫一扫识别', POSTER_WIDTH / 2, qrY + QR_CODE_SIZE + 92)

  const tempFilePath = await exportCanvasToTempFile(canvas)
  await saveImageFileToAlbum(tempFilePath)
}