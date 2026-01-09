export const PI = 3.1415926535897932384626;
export const A = 6378245.0;
export const EE = 0.00669342162296594323;

/**
 * Check if the coordinate is outside China.
 * @param lat Latitude
 * @param lng Longitude
 * @returns boolean
 */
export function outOfChina(lat: number, lng: number): boolean {
    if (lng < 72.004 || lng > 137.8347) {
        return true;
    }
    if (lat < 0.8293 || lat > 55.8271) {
        return true;
    }
    return false;
}

function transformLat(x: number, y: number): number {
    let ret = -100.0 + 2.0 * x + 3.0 * y + 0.2 * y * y + 0.1 * x * y + 0.2 * Math.sqrt(Math.abs(x));
    ret += (20.0 * Math.sin(6.0 * x * PI) + 20.0 * Math.sin(2.0 * x * PI)) * 2.0 / 3.0;
    ret += (20.0 * Math.sin(y * PI) + 40.0 * Math.sin(y / 3.0 * PI)) * 2.0 / 3.0;
    ret += (160.0 * Math.sin(y / 12.0 * PI) + 320 * Math.sin(y * PI / 30.0)) * 2.0 / 3.0;
    return ret;
}

function transformLng(x: number, y: number): number {
    let ret = 300.0 + x + 2.0 * y + 0.1 * x * x + 0.1 * x * y + 0.1 * Math.sqrt(Math.abs(x));
    ret += (20.0 * Math.sin(6.0 * x * PI) + 20.0 * Math.sin(2.0 * x * PI)) * 2.0 / 3.0;
    ret += (20.0 * Math.sin(x * PI) + 40.0 * Math.sin(x / 3.0 * PI)) * 2.0 / 3.0;
    ret += (150.0 * Math.sin(x / 12.0 * PI) + 300.0 * Math.sin(x / 30.0 * PI)) * 2.0 / 3.0;
    return ret;
}

/**
 * Convert WGS-84 (GPS/OSM) to GCJ-02 (WeChat/Tencent Maps)
 * @param wgsLat WGS-84 Latitude
 * @param wgsLng WGS-84 Longitude
 * @returns [gcjLat, gcjLng]
 */
export function wgs84ToGcj02(wgsLat: number, wgsLng: number): [number, number] {
    if (outOfChina(wgsLat, wgsLng)) {
        return [wgsLat, wgsLng];
    }
    let dLat = transformLat(wgsLng - 105.0, wgsLat - 35.0);
    let dLng = transformLng(wgsLng - 105.0, wgsLat - 35.0);
    const radLat = wgsLat / 180.0 * PI;
    let magic = Math.sin(radLat);
    magic = 1 - EE * magic * magic;
    const sqrtMagic = Math.sqrt(magic);
    dLat = (dLat * 180.0) / ((A * (1 - EE)) / (magic * sqrtMagic) * PI);
    dLng = (dLng * 180.0) / (A / sqrtMagic * Math.cos(radLat) * PI);
    const mgLat = wgsLat + dLat;
    const mgLng = wgsLng + dLng;
    return [mgLat, mgLng];
}

/**
 * Convert GCJ-02 (WeChat/Tencent Maps) to WGS-84 (GPS/OSM)
 * @param gcjLat GCJ-02 Latitude
 * @param gcjLng GCJ-02 Longitude
 * @returns [wgsLat, wgsLng]
 */
export function gcj02ToWgs84(gcjLat: number, gcjLng: number): [number, number] {
    if (outOfChina(gcjLat, gcjLng)) {
        return [gcjLat, gcjLng];
    }
    let dLat = transformLat(gcjLng - 105.0, gcjLat - 35.0);
    let dLng = transformLng(gcjLng - 105.0, gcjLat - 35.0);
    const radLat = gcjLat / 180.0 * PI;
    let magic = Math.sin(radLat);
    magic = 1 - EE * magic * magic;
    const sqrtMagic = Math.sqrt(magic);
    dLat = (dLat * 180.0) / ((A * (1 - EE)) / (magic * sqrtMagic) * PI);
    dLng = (dLng * 180.0) / (A / sqrtMagic * Math.cos(radLat) * PI);
    const mgLat = gcjLat + dLat;
    const mgLng = gcjLng + dLng;
    return [gcjLat * 2 - mgLat, gcjLng * 2 - mgLng];
}
