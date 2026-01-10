import { serve } from "std/http/server.ts"
import { createClient } from "@supabase/supabase-js"

const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
}

interface ImageUploadPayload {
  bucket: 'assets' | 'identity'
  path: string // e.g., 'merchants/123/license.jpg'
}

serve(async (req: Request) => {
  if (req.method === 'OPTIONS') {
    return new Response('ok', { headers: corsHeaders })
  }

  try {
    const authHeader = req.headers.get('Authorization')
    if (!authHeader) throw new Error('Missing Authorization header')

    const supabase = createClient(
      Deno.env.get('SUPABASE_URL') ?? '',
      Deno.env.get('SUPABASE_ANON_KEY') ?? '',
      { global: { headers: { Authorization: authHeader } } }
    )

    // Get user from token
    const { data: { user }, error: userError } = await supabase.auth.getUser()
    if (userError || !user) throw new Error('Unauthorized')

    // Parse multipart form
    const formData = await req.formData()
    const file = formData.get('file') as File
    const bucket = formData.get('bucket') as 'assets' | 'identity'
    const customPath = formData.get('path') as string

    if (!file || !bucket || !customPath) {
      throw new Error('Missing required fields (file, bucket, path)')
    }

    // 1. WeChat Image Security Check
    await wechatImgSecCheck(file)

    // 2. Prepare Storage Path: {user_id}/{customPath}
    // We prefix with user_id to match RLS policies
    const storagePath = `${user.id}/${customPath.replace(/^\//, '')}`

    // 3. Upload to Supabase Storage
    const { data: _uploadData, error: uploadError } = await supabase.storage
      .from(bucket)
      .upload(storagePath, file, {
        upsert: true,
        contentType: file.type
      })

    if (uploadError) throw uploadError

    // 4. Return Public URL or Signed URL
    let finalUrl = ''
    if (bucket === 'assets') {
      const { data: { publicUrl } } = supabase.storage.from(bucket).getPublicUrl(storagePath)
      finalUrl = publicUrl
    } else {
      // For identity, we return the path so client can request a signed URL when needed, 
      // or we can generate one now (but it expires)
      finalUrl = storagePath 
    }

    return new Response(
      JSON.stringify({ 
        url: finalUrl, 
        path: storagePath,
        bucket: bucket,
        full_url: bucket === 'assets' ? finalUrl : null
      }),
      { headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    )

  } catch (error) {
    return new Response(
      JSON.stringify({ error: error instanceof Error ? error.message : 'Unknown error' }),
      { status: 400, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    )
  }
})

async function wechatImgSecCheck(file: File) {
  const appId = Deno.env.get('WECHAT_MINI_APP_ID')
  const secret = Deno.env.get('WECHAT_MINI_APP_SECRET')
  
  // 1. Get Access Token
  const tokenUrl = `https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=${appId}&secret=${secret}`
  const tokenResp = await fetch(tokenUrl)
  const tokenData = await tokenResp.json()
  const accessToken = tokenData.access_token

  if (!accessToken) {
    console.error('Failed to get WeChat access token', tokenData)
    // In dev, we might skip if not configured? No, let's enforce it.
    throw new Error('Internal Configuration Error: WeChat Token')
  }

  // 2. Check Security
  // Note: WeChat API wants multipart/form-data with 'media' field
  const checkUrl = `https://api.weixin.qq.com/wxa/img_sec_check?access_token=${accessToken}`
  const checkFormData = new FormData()
  checkFormData.append('media', file)

  const checkResp = await fetch(checkUrl, {
    method: 'POST',
    body: checkFormData
  })
  const checkData = await checkResp.json()

  if (checkData.errcode === 87014) {
    throw new Error('图片内容涉及敏感或违规信息，请更换图片')
  } else if (checkData.errcode !== 0) {
    console.warn('WeChat Security Check warning:', checkData)
    // Some errors might be ignored, but 87014 is the block one.
  }
}
