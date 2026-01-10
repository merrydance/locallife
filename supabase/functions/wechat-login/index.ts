import { serve } from "std/http/server.ts"
import { createClient } from "@supabase/supabase-js"
import { crypto } from "std/crypto/mod.ts"

console.log("Edge Function: wechat-login loaded v3 (Device Aware)");

const corsHeaders = {
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
}

serve(async (req: Request) => {
    // Debug Log: Version 4 (Full Headers)
    // Debug Log: Version 5 (Log Headers)
    console.log(`[wechat-login] Received ${req.method} request at ${new Date().toISOString()}`);
    
    // Print all headers for debugging
    const headersObj: Record<string, string> = {};
    req.headers.forEach((value, key) => {
        headersObj[key] = value;
    });
    console.log(`[wechat-login] Headers:`, JSON.stringify(headersObj, null, 2));

    if (req.method === 'OPTIONS') {
        return new Response('ok', { headers: corsHeaders })
    }

    try {
        const body = await req.json()
        const { code, device_id, device_type } = body
        console.log(`[wechat-login] Body:`, JSON.stringify(body, null, 2));
        console.log(`[wechat-login] code: ${code ? '*****' : 'null'}, device: ${device_id || 'N/A'}`);

        if (!code) {
            throw new Error('Missing code')
        }

        // 1. Get WeChat Secrets
        const miniappId = Deno.env.get('WECHAT_MINI_APP_ID')
        const miniappSecret = Deno.env.get('WECHAT_MINI_APP_SECRET')

        if (!miniappId || !miniappSecret) {
            console.error('[wechat-login] Missing env vars');
            throw new Error('Missing WeChat Configuration')
        }

        // 2. Call WeChat API
        const wxUrl = `https://api.weixin.qq.com/sns/jscode2session?appid=${miniappId}&secret=${miniappSecret}&js_code=${code}&grant_type=authorization_code`
        const wxResp = await fetch(wxUrl)
        const wxData = await wxResp.json()

        if (wxData.errcode) {
            console.error('WeChat API Error:', wxData)
            throw new Error(`WeChat Error: ${wxData.errmsg}`)
        }

        const { openid, session_key, unionid } = wxData
        console.log(`[wechat-login] WeChat success: ${openid}`);

        // 3. User Credentials
        const email = `${openid}@wechat.locallife`
        // Generate deterministic password for this user
        const encoder = new TextEncoder()
        const keyData = encoder.encode(miniappSecret)
        const activeKey = await crypto.subtle.importKey(
            "raw", keyData, { name: "HMAC", hash: "SHA-256" }, false, ["sign"]
        )
        const signature = await crypto.subtle.sign(
            "HMAC", activeKey, encoder.encode(openid)
        )
        const password = Array.from(new Uint8Array(signature)).map(b => b.toString(16).padStart(2, '0')).join('')

        // 4. Initialize Supabase Admin Client
        const supabaseUrl = Deno.env.get('SUPABASE_URL') ?? ''
        const supabaseKey = Deno.env.get('SUPABASE_SERVICE_ROLE_KEY') ?? ''
        const supabase = createClient(supabaseUrl, supabaseKey)

        // 5. Sign In
        let { data: sessionData, error: signInError } = await supabase.auth.signInWithPassword({
            email, password
        })

        // 6. Sign Up if needed
        if (signInError && signInError.message.includes('Invalid login credentials')) {
            console.log('[wechat-login] User not found, signing up');
            const { data: signUpData, error: signUpError } = await supabase.auth.signUp({
                email,
                password,
                options: {
                    data: {
                        openid: openid,
                        unionid: unionid,
                        full_name: 'WeChat User',
                        avatar_url: '',
                    }
                }
            })

            if (signUpError) throw signUpError
            if (signUpData.session && signUpData.user) {
                sessionData = { session: signUpData.session, user: signUpData.user }
            } else {
                throw new Error('Sign up failed: missing session or user')
            }
        } else if (signInError) {
            throw signInError
        }

        if (!sessionData.session || !sessionData.user) {
            throw new Error('Failed to create session')
        }

        const user = sessionData.user
        const userId = user.id

        // 7. Record Device Info (Fraud Detection)
        if (device_id) {
            console.log('[wechat-login] Recording device info...');
            const { error: deviceError } = await supabase
                .from('user_devices')
                .upsert({
                    user_id: userId,
                    device_id: device_id,
                    device_type: device_type || 'miniprogram',
                    last_seen_at: new Date().toISOString()
                }, { onConflict: 'user_id, device_id' })

            if (deviceError) {
                console.warn('[wechat-login] Failed to record device:', deviceError)
                // Do not fail login for this, just log warning
            }
        }

        // 8. Fetch User Roles
        const { data: rolesData } = await supabase
            .from('user_roles')
            .select('role')
            .eq('user_id', userId)

        const roles = rolesData ? rolesData.map((r) => String(r.role)) : ['customer'];

        // Update user metadata with session key if needed for later decrypts
        // (Note: Storing session_key in metadata is standard for these flows, ensure RLS protects it)
        await supabase.auth.admin.updateUserById(
            userId,
            { user_metadata: { ...user.user_metadata, session_key: session_key } }
        )

        // 9. Construct Response matching Supabase standard (which Frontend expects)
        const responseData = {
            session: {
                access_token: sessionData.session.access_token,
                refresh_token: sessionData.session.refresh_token,
                expires_in: sessionData.session.expires_in,
                token_type: sessionData.session.token_type,
                user: sessionData.user
            },
            user: {
                ...sessionData.user,
                roles: roles
            }
        }

        console.log(`[wechat-login] Success! Returning session for user: ${userId}`);

        return new Response(
            JSON.stringify(responseData),
            { headers: { ...corsHeaders, "Content-Type": "application/json" } },
        )
    } catch (error) {
        const err = error as Error;
        console.error('[wechat-login] Fatal Error:', err);
        return new Response(
            JSON.stringify({ error: err.message || 'Unknown error' }),
            { status: 400, headers: { ...corsHeaders, "Content-Type": "application/json" } },
        )
    }
})
