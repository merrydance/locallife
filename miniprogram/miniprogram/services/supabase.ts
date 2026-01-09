import { createClient } from 'supabase-wechat-stable'
import { Database } from '../typings/database.types'

const supabaseUrl = 'http://127.0.0.1:64321'
const supabaseKey = 'sb_publishable_ACJWlzQHlZjBrEguHvfOxg_3BJgxAaH'

export const supabase = createClient<Database>(supabaseUrl, supabaseKey, {
    auth: {
        persistSession: true,
        autoRefreshToken: true,
        detectSessionInUrl: false,
    },
    global: {
        headers: { 'x-my-custom-header': 'locallife-poc' },
    },
})
