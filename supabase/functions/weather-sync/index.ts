import { serve } from "std/http/server.ts"
import { createClient } from "@supabase/supabase-js"

const QWEATHER_API_KEY = Deno.env.get("QWEATHER_API_KEY")
const QWEATHER_API_HOST = Deno.env.get("QWEATHER_API_HOST") || "https://devapi.qweather.com/v7"
const SUPABASE_URL = Deno.env.get("SUPABASE_URL")
const SUPABASE_SERVICE_ROLE_KEY = Deno.env.get("SUPABASE_SERVICE_ROLE_KEY")

const supabase = createClient(SUPABASE_URL!, SUPABASE_SERVICE_ROLE_KEY!)

// ==================== QWeather API 类型定义 ====================

interface QWeatherNow {
  obsTime: string
  temp: string
  feelsLike: string
  icon: string
  text: string
  wind360: string
  windDir: string
  windScale: string
  windSpeed: string
  humidity: string
  precip: string
  pressure: string
  vis: string
  cloud: string
  dew: string
}

interface QWeatherNowResponse {
  code: string
  updateTime: string
  fxLink: string
  now: QWeatherNow
  refer: {
    sources: string[]
    license: string[]
  }
}

interface QWeatherWarning {
  id: string
  sender: string
  pubTime: string
  title: string
  startTime: string
  endTime: string
  status: string
  level: string
  severity: string
  severityColor: string
  type: string
  typeName: string
  urgency: string
  certainty: string
  probability: string
  text: string
}

interface QWeatherWarningResponse {
  code: string
  updateTime: string
  fxLink: string
  warning: QWeatherWarning[]
}

type JsonValue = string | number | boolean | null | { [key: string]: JsonValue } | JsonValue[]

// ==================== 业务逻辑 ====================

// 分类天气类型 (对齐 Go classifyWeatherType)
function classifyWeatherType(text: string): string {
  const t = text.toLowerCase()
  if (["台风", "龙卷", "冰雹", "暴风", "沙尘暴"].some(kw => t.includes(kw))) return "extreme"
  if (t.includes("雪")) {
    if (t.includes("暴雪") || t.includes("大雪")) return "heavy_snow"
    if (t.includes("中雪")) return "moderate_snow"
    return "light_snow"
  }
  if (t.includes("雨")) {
    if (["暴雨", "大暴雨", "特大暴雨"].some(kw => t.includes(kw))) return "heavy_rain"
    if (t.includes("大雨") || t.includes("中雨")) return "moderate_rain"
    return "light_rain"
  }
  if (t.includes("阴") || t.includes("多云")) return "cloudy"
  return "sunny"
}

// 计算基础系数 (对齐 Go calculateBaseCoefficient)
function calculateBaseCoefficient(type: string, temp: number, windScale: number, visibility: number): number {
  let coeff = 1.00
  switch (type) {
    case "extreme": coeff = 2.00; break
    case "heavy_rain": case "heavy_snow": coeff = 1.50; break
    case "moderate_rain": case "moderate_snow": coeff = 1.30; break
    case "light_rain": case "light_snow": coeff = 1.10; break
    default: coeff = 1.00
  }
  if (temp > 35) coeff += 0.10
  if (temp < 0) coeff += 0.10
  if (windScale >= 6) coeff += 0.15
  if (visibility > 0 && visibility < 3) coeff += 0.10
  return coeff
}

serve(async (_req): Promise<Response> => {
  try {
    const { data: regions, error: regionError } = await supabase
      .from("regions")
      .select("id, name, latitude, longitude, qweather_location_id")
      .is("parent_id", null) 

    if (regionError) throw regionError
    const results = []

    for (const region of (regions || [])) {
      const location = region.qweather_location_id || `${region.longitude},${region.latitude}`
      
      // 1. 获取实时天气
      const weatherUrl = `${QWEATHER_API_HOST}/weather/now?location=${location}&key=${QWEATHER_API_KEY}`
      const weatherRes = await fetch(weatherUrl)
      const weatherData = (await weatherRes.json()) as QWeatherNowResponse
      if (weatherData.code !== "200") continue

      const now = weatherData.now
      const type = classifyWeatherType(now.text)
      
      // 2. 获取预警
      let warningCoeff = 1.00
      let suspend = false
      let warningType = ""
      let warningLevel = ""
      let warningText = ""
      
      const warningUrl = `${QWEATHER_API_HOST}/warning/now?location=${location}&key=${QWEATHER_API_KEY}`
      const warningRes = await fetch(warningUrl)
      const warningData = (await warningRes.json()) as QWeatherWarningResponse
      
      if (warningData.code === "200" && warningData.warning?.length > 0) {
        for (const w of warningData.warning) {
          const level = w.severityColor.toLowerCase()
          let currentCoeff = 1.00
          if (level === "red") { currentCoeff = 2.00; suspend = true }
          else if (level === "orange") currentCoeff = 1.30
          else if (level === "yellow") currentCoeff = 1.20
          else if (level === "blue") currentCoeff = 1.10
          
          if (["台风", "龙卷风", "冰雹", "暴风雪", "沙尘暴", "大暴雨", "特大暴雨"].some(kw => w.typeName.includes(kw))) {
            suspend = true; currentCoeff = 2.00
          }
          
          if (currentCoeff > warningCoeff) {
            warningCoeff = currentCoeff
            warningType = w.typeName
            warningLevel = level
            warningText = w.title
          }
        }
      }

      // 3. 计算最终系数
      const baseCoeff = calculateBaseCoefficient(
        type, 
        parseInt(now.temp, 10), 
        parseInt(now.windScale, 10), 
        parseInt(now.vis, 10)
      )
      const finalCoeff = baseCoeff * warningCoeff

      // 4. 入库
      await supabase.from("weather_coefficients").insert({
        region_id: region.id,
        recorded_at: new Date().toISOString(),
        weather_type: type,
        weather_code: now.icon,
        temperature: parseInt(now.temp, 10),
        feels_like: parseInt(now.feelsLike, 10),
        humidity: parseInt(now.humidity, 10),
        wind_speed: parseInt(now.windSpeed, 10),
        wind_scale: now.windScale,
        precip: parseFloat(now.precip),
        visibility: parseInt(now.vis, 10),
        has_warning: warningType !== "",
        warning_type: warningType,
        warning_level: warningLevel,
        warning_text: warningText,
        weather_coefficient: baseCoeff,
        warning_coefficient: warningCoeff,
        final_coefficient: finalCoeff,
        delivery_suspended: suspend || type === "extreme",
        weather_data: (weatherData as unknown) as JsonValue,
        warning_data: (warningData as unknown) as JsonValue
      })

      results.push({ region: region.name, finalCoeff, suspended: suspend || type === "extreme" })
    }

    return new Response(JSON.stringify({ success: true, results }), {
      headers: { "Content-Type": "application/json" },
    })
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    return new Response(JSON.stringify({ error: message }), {
      status: 500,
      headers: { "Content-Type": "application/json" },
    })
  }
})
