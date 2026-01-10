import { serve } from 'http/server.ts'
import { createClient } from '@supabase/supabase-js'

const WECHAT_APPID = Deno.env.get('WECHAT_MINI_APP_ID')
const WECHAT_SECRET = Deno.env.get('WECHAT_MINI_APP_SECRET')
const SUPABASE_URL = Deno.env.get('SUPABASE_URL')
const SUPABASE_SERVICE_ROLE_KEY = Deno.env.get('SUPABASE_SERVICE_ROLE_KEY')

const supabase = createClient(SUPABASE_URL!, SUPABASE_SERVICE_ROLE_KEY!)

interface OcrResult {
  status: string
  ocr_at: string
  [key: string]: unknown
}

interface FoodPermitData {
  raw_text: string
  company_name?: string
  permit_no?: string
  valid_to?: string
}

interface HealthCertData {
  raw_text: string
  id_number?: string
  name?: string
  cert_number?: string
  valid_end?: string
  valid_start?: string
}

async function getWechatAccessToken() {
  const url = `https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=${WECHAT_APPID}&secret=${WECHAT_SECRET}`
  const resp = await fetch(url)
  const data = await resp.json()
  if (data.errcode) throw new Error(`WeChat AccessToken Error: ${data.errmsg}`)
  return data.access_token as string
}

async function ocrBusinessLicense(accessToken: string, fileBlob: Blob) {
  const url = `https://api.weixin.qq.com/cv/ocr/bizlicense?access_token=${accessToken}`
  const formData = new FormData()
  formData.append('img', fileBlob)
  const resp = await fetch(url, { method: 'POST', body: formData })
  return await resp.json()
}

async function ocrIDCard(accessToken: string, fileBlob: Blob, _side: 'Front' | 'Back') {
  const url = `https://api.weixin.qq.com/cv/ocr/idcard?type=photo&access_token=${accessToken}`
  const formData = new FormData()
  formData.append('img', fileBlob)
  const resp = await fetch(url, { method: 'POST', body: formData })
  return await resp.json()
}

async function ocrPrintedText(accessToken: string, fileBlob: Blob) {
  const url = `https://api.weixin.qq.com/cv/ocr/comm?access_token=${accessToken}`
  const formData = new FormData()
  formData.append('img', fileBlob)
  const resp = await fetch(url, { method: 'POST', body: formData })
  return await resp.json()
}

// 模拟 Go 后端的食品许可证解析逻辑
function parseFoodPermit(rawText: string) {
  const data: FoodPermitData = { raw_text: rawText }
  
  // 企业名称提取
  const namePatterns = [
    /(?:经营者名称|单位名称|名\s*称)\s*[:：]?\s*([^\n\r]{2,50})/,
    /主体名称\s*[:：]?\s*([^\n\r]{2,50})/,
    /统一社会信用代码[^\n]*\n\s*([^\n\r]{2,50})/,
    /^([^\n\r]{0,20}(?:餐饮|食品|饮品|酒店|饭店|餐厅)[^\n\r]{0,30})$/m
  ]

  for (const regex of namePatterns) {
    const match = rawText.match(regex)
    if (match && match[1]) {
      data.company_name = match[1].replace(/\s+/g, '').trim()
      break
    }
  }

  // 许可证编号
  const permitNoMatch = rawText.match(/JY[0-9]{12,}/)
  if (permitNoMatch) data.permit_no = permitNoMatch[0]

  // 有效期
  if (rawText.includes('长期')) {
    data.valid_to = '长期'
  } else {
    const validToRegex = /(?:有效期至|至|有效期限至)\s*[:：]?\s*(\d{4}年\d{1,2}月\d{1,2}日)/
    const match = rawText.match(validToRegex)
    if (match) data.valid_to = match[1]
  }

  return data
}

// 模拟 Go 后端的健康证解析逻辑
function parseHealthCert(rawText: string) {
  const data: HealthCertData = { raw_text: rawText }

  // 身份证号 (18位)
  const idMatch = rawText.match(/\b\d{17}[0-9Xx]\b/)
  if (idMatch) data.id_number = idMatch[0].toUpperCase()

  // 姓名
  const nameRegex = /(?:从业人员姓名|持证人|体检者|姓名)\s*[:：]?\s*([^\n\r\s]{2,20})/
  const nameMatch = rawText.match(nameRegex)
  if (nameMatch) data.name = nameMatch[1].trim()

  // 证书编号
  const certRegex = /(?:健康证号|证书编号|证号|编号)\s*[:：]?\s*([A-Za-z0-9\-]{5,})/
  const certMatch = rawText.match(certRegex)
  if (certMatch) data.cert_number = certMatch[1].trim()

  // 有效期
  const validToRegex = /(?:有效期至|有效期到|有效期)\s*[:：]?\s*(\d{4}年\d{1,2}月\d{1,2}日|长期)/
  const vtMatch = rawText.match(validToRegex)
  if (vtMatch) data.valid_end = vtMatch[1].trim()

  const rangeRegex = /(\d{4}年\d{1,2}月\d{1,2}日)\s*[至到-]\s*(\d{4}年\d{1,2}月\d{1,2}日|长期)/
  const rMatch = rawText.match(rangeRegex)
  if (rMatch) {
    data.valid_start = rMatch[1].trim()
    data.valid_end = rMatch[2].trim()
  }

  return data
}

serve(async (req) => {
  if (req.method === 'OPTIONS') return new Response('ok', { headers: { 'Access-Control-Allow-Origin': '*' } })

  try {
    const { application_id, image_url, type, side, target_table } = await req.json()
    const tableName = target_table || 'merchant_applications'

    // 1. 获取图片
    const imgResp = await fetch(image_url)
    const imgBlob = await imgResp.blob()

    // 2. OCR
    const accessToken = await getWechatAccessToken()
    let ocrResult: Record<string, unknown>
    let updateField: string

    switch (type) {
      case 'business_license': {
        ocrResult = await ocrBusinessLicense(accessToken, imgBlob)
        updateField = 'business_license_ocr'
        break
      }
      case 'id_card': {
        ocrResult = await ocrIDCard(accessToken, imgBlob, side)
        updateField = side === 'Front' ? 'id_card_front_ocr' : 'id_card_back_ocr'
        // 特殊处理：骑手申请表的 id_card_ocr 是合并的
        if (tableName === 'rider_applications') {
          updateField = 'id_card_ocr'
        }
        break
      }
      case 'food_permit': {
        const printedFood = await ocrPrintedText(accessToken, imgBlob)
        const allTextFood = (printedFood.items as { text: string }[] | undefined)?.map((it) => it.text).join('\n') || ''
        ocrResult = parseFoodPermit(allTextFood) as unknown as Record<string, unknown>
        updateField = 'food_permit_ocr'
        break
      }
      case 'health_cert': {
        const printedHealth = await ocrPrintedText(accessToken, imgBlob)
        const allTextHealth = (printedHealth.items as { text: string }[] | undefined)?.map((it) => it.text).join('\n') || ''
        ocrResult = parseHealthCert(allTextHealth) as unknown as Record<string, unknown>
        updateField = 'health_cert_ocr'
        break
      }
      default:
        throw new Error('Invalid OCR type')
    }

    // 3. 更新数据库
    let updateData: OcrResult = { status: 'done', ocr_at: new Date().toISOString(), ...ocrResult }
    
    // 如果是 ID Card OCR 且是骑手申请表，需要合并旧数据（因为有 Front 和 Back）
    if (type === 'id_card' && tableName === 'rider_applications') {
      const { data: existing } = await supabase
        .from(tableName)
        .select('id_card_ocr')
        .eq('id', application_id)
        .single()
      
      const oldOcr = (existing as { id_card_ocr: Record<string, unknown> | null } | null)?.id_card_ocr || {}
      updateData = { ...oldOcr, ...ocrResult, status: 'done', ocr_at: new Date().toISOString() }
    }

    const { error } = await supabase
      .from(tableName)
      .update({ [updateField]: updateData })
      .eq('id', application_id)

    if (error) throw error

    return new Response(JSON.stringify({ success: true, ocr_result: updateData }), {
      headers: { 'Content-Type': 'application/json' }
    })

  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : String(err)
    return new Response(JSON.stringify({ error: message }), {
      status: 500,
      headers: { 'Content-Type': 'application/json' }
    })
  }
})
