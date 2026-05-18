type MapPoint = {
    latitude?: number
    longitude?: number
    name?: string
}

type MapLocationPoint = {
    latitude: number
    longitude: number
}

type MarkerItem = {
    id: number
    latitude: number
    longitude: number
    iconPath?: string
    width: number
    height: number
    callout: {
        content: string
        padding: number
        borderRadius: number
        display: 'ALWAYS'
    }
}

type PolylineItem = {
    points: MapLocationPoint[]
    color: string
    width: number
    arrowLine: boolean
}

Component({
    properties: {
        merchant: {
            type: Object,
            value: undefined // { latitude, longitude, name }
        },
        customer: {
            type: Object,
            value: undefined // { latitude, longitude, name }
        },
        rider: {
            type: Object,
            value: undefined // { latitude, longitude }
        },
        showRoute: {
            type: Boolean,
            value: true
        }
    },

    data: {
        markers: [] as MarkerItem[],
        polyline: [] as PolylineItem[],
        includePoints: [] as MapLocationPoint[],
        centerLatitude: 0,
        centerLongitude: 0,
        hasValidLocation: false
    },

    observers: {
        'merchant, customer, rider' (_merchant, _customer, _rider) {
            this.updateMap()
        }
    },

    methods: {
        updateMap() {
            const { merchant, customer, rider, showRoute } = this.properties as {
                merchant?: MapPoint
                customer?: MapPoint
                rider?: MapPoint
                showRoute?: boolean
            }
            const markers: MarkerItem[] = []
            const includePoints: MapLocationPoint[] = []
            let centerPoint: MapLocationPoint | null = null

            if (
                merchant &&
                typeof merchant.latitude === 'number' &&
                typeof merchant.longitude === 'number'
            ) {
                markers.push({
                    id: 1,
                    latitude: merchant.latitude,
                    longitude: merchant.longitude,
                    iconPath: '/assets/merchant.png',
                    width: 32,
                    height: 32,
                    callout: {
                        content: '商家',
                        padding: 8,
                        borderRadius: 4,
                        display: 'ALWAYS'
                    }
                })
                includePoints.push({ latitude: merchant.latitude, longitude: merchant.longitude })
                centerPoint = centerPoint || { latitude: merchant.latitude, longitude: merchant.longitude }
            }

            if (
                customer &&
                typeof customer.latitude === 'number' &&
                typeof customer.longitude === 'number'
            ) {
                markers.push({
                    id: 2,
                    latitude: customer.latitude,
                    longitude: customer.longitude,
                    iconPath: '/assets/customer.png',
                    width: 32,
                    height: 32,
                    callout: {
                        content: '顾客',
                        padding: 8,
                        borderRadius: 4,
                        display: 'ALWAYS'
                    }
                })
                includePoints.push({ latitude: customer.latitude, longitude: customer.longitude })
                centerPoint = centerPoint || { latitude: customer.latitude, longitude: customer.longitude }
            }

            if (
                rider &&
                typeof rider.latitude === 'number' &&
                typeof rider.longitude === 'number'
            ) {
                markers.push({
                    id: 3,
                    latitude: rider.latitude,
                    longitude: rider.longitude,
                    iconPath: '/assets/rider.png',
                    width: 32,
                    height: 32,
                    callout: {
                        content: '骑手',
                        padding: 8,
                        borderRadius: 4,
                        display: 'ALWAYS'
                    }
                })
                includePoints.push({ latitude: rider.latitude, longitude: rider.longitude })
                centerPoint = { latitude: rider.latitude, longitude: rider.longitude }
            }

            // Simple straight line for polyline if route API is not used
            let polyline: PolylineItem[] = []
            if (
                showRoute &&
                merchant &&
                typeof merchant.latitude === 'number' &&
                typeof merchant.longitude === 'number' &&
                customer &&
                typeof customer.latitude === 'number' &&
                typeof customer.longitude === 'number'
            ) {
                polyline = [{
                    points: [
                        { latitude: merchant.latitude, longitude: merchant.longitude },
                        { latitude: customer.latitude, longitude: customer.longitude }
                    ],
                    color: '#1890ff',
                    width: 4,
                    arrowLine: true
                }]
            }

            this.setData({
                markers,
                polyline,
                includePoints,
                centerLatitude: centerPoint?.latitude || 0,
                centerLongitude: centerPoint?.longitude || 0,
                hasValidLocation: !!centerPoint
            })
        }
    }
})
