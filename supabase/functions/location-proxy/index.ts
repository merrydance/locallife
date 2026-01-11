import { serve } from "std/http/server.ts"
import { gcj02ToWgs84 } from "../_shared/geo_converter.ts"

console.log("Edge Function: location-proxy loaded v3 (GCJ02 Support)");

const corsHeaders = {
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
}

serve(async (req: Request) => {
    if (req.method === 'OPTIONS') {
        return new Response('ok', { headers: corsHeaders })
    }

    try {
        const { action, latitude, longitude, from, to } = await req.json()

        // Configuration for Local OSM Services (Using the Docker gateway for supabase_network_locallife)
        const NOMINATIM_BASE = Deno.env.get('NOMINATIM_API_BASE') || 'http://nominatim:8080'
        const OSRM_BASE = Deno.env.get('OSRM_API_BASE') || 'http://osrm:5000'

        console.log(`[location-proxy] Action: ${action}, Using Nominatim: ${NOMINATIM_BASE}, OSRM: ${OSRM_BASE}`)

        let resultData: Record<string, unknown> | null = null

        if (action === 'reverse-geocode') {
            if (!latitude || !longitude) throw new Error('Missing lat/lng for reverse-geocode')

            // Convert GCJ02 (Frontend) -> WGS84 (OSM)
            const [wgsLat, wgsLng] = gcj02ToWgs84(Number(latitude), Number(longitude))

            // Call Nominatim (expects WGS84)
            const url = `${NOMINATIM_BASE}/reverse?format=json&lat=${wgsLat}&lon=${wgsLng}&zoom=18&addressdetails=1`
            console.log(`[location-proxy] Fetching: ${url}`)

            const resp = await fetch(url)
            console.log(`[location-proxy] Nominatim response: ${resp.status} ${resp.statusText}`)
            if (!resp.ok) {
                const errText = await resp.text()
                console.error(`[location-proxy] Nominatim error body: ${errText}`)
                throw new Error(`Nominatim error: ${resp.statusText}`)
            }

            const data = await resp.json()

            // Map Nominatim response to expected LocationInfo format
            const addr = data.address || {}

            resultData = {
                address: data.display_name,
                formatted_address: data.display_name,
                province: addr.state || addr.province || '',
                city: addr.city || addr.town || addr.county || '',
                district: addr.district || addr.suburb || '',
                street: addr.road || addr.pedestrian || '',
                street_number: addr.house_number || '',
                _raw: data
            }

        } else if (action === 'direction-bicycling') {
            if (!from || !to) throw new Error('Missing from/to for direction')

            // Helper: "lat,lng" (GCJ02) -> "lng,lat" (WGS84 for OSRM)
            const toWgs84LngLat = (str: string) => {
                const [lat, lng] = str.split(',').map(n => Number(n.trim()))
                const [wgsLat, wgsLng] = gcj02ToWgs84(lat, lng)
                return `${wgsLng},${wgsLat}`
            }

            const coords = `${toWgs84LngLat(from)};${toWgs84LngLat(to)}`
            const url = `${OSRM_BASE}/route/v1/bicycle/${coords}?overview=full&steps=true`

            console.log(`[location-proxy] Fetching: ${url}`)

            const resp = await fetch(url)
            if (!resp.ok) throw new Error(`OSRM error: ${resp.statusText}`)

            const data = await resp.json()
            resultData = data

        } else {
            throw new Error(`Unknown action: ${action}`)
        }

        return new Response(
            JSON.stringify({
                code: 0,
                message: 'ok',
                data: resultData
            }),
            { headers: { ...corsHeaders, "Content-Type": "application/json" } },
        )

    } catch (error) {
        const err = error as Error;
        console.error(err)
        return new Response(
            JSON.stringify({ code: -1, error: err.message }),
            { status: 400, headers: { ...corsHeaders, "Content-Type": "application/json" } },
        )
    }
})
