"use strict";
Component({
    properties: {
        merchant: {
            type: Object,
            value: null // { latitude, longitude, name }
        },
        customer: {
            type: Object,
            value: null // { latitude, longitude, name }
        },
        rider: {
            type: Object,
            value: null // { latitude, longitude }
        },
        showRoute: {
            type: Boolean,
            value: true
        }
    },
    data: {
        markers: [],
        polyline: [],
        includePoints: []
    },
    observers: {
        'merchant, customer, rider': function (merchant, customer, rider) {
            this.updateMap();
        }
    },
    methods: {
        updateMap() {
            const { merchant, customer, rider } = this.properties;
            const markers = [];
            const includePoints = [];
            if (merchant && merchant.latitude) {
                markers.push({
                    id: 1,
                    latitude: merchant.latitude,
                    longitude: merchant.longitude,
                    iconPath: '/assets/images/marker-merchant.png', // Assuming assets exist or using default
                    width: 32,
                    height: 32,
                    callout: {
                        content: '商家',
                        padding: 8,
                        borderRadius: 4,
                        display: 'ALWAYS'
                    }
                });
                includePoints.push({ latitude: merchant.latitude, longitude: merchant.longitude });
            }
            if (customer && customer.latitude) {
                markers.push({
                    id: 2,
                    latitude: customer.latitude,
                    longitude: customer.longitude,
                    iconPath: '/assets/images/marker-customer.png',
                    width: 32,
                    height: 32,
                    callout: {
                        content: '顾客',
                        padding: 8,
                        borderRadius: 4,
                        display: 'ALWAYS'
                    }
                });
                includePoints.push({ latitude: customer.latitude, longitude: customer.longitude });
            }
            if (rider && rider.latitude) {
                markers.push({
                    id: 3,
                    latitude: rider.latitude,
                    longitude: rider.longitude,
                    iconPath: '/assets/images/marker-rider.png',
                    width: 32,
                    height: 32,
                    callout: {
                        content: '骑手',
                        padding: 8,
                        borderRadius: 4,
                        display: 'ALWAYS'
                    }
                });
                includePoints.push({ latitude: rider.latitude, longitude: rider.longitude });
            }
            // Simple straight line for polyline if route API is not used
            let polyline = [];
            if (this.properties.showRoute && merchant && customer) {
                polyline = [{
                        points: [
                            { latitude: merchant.latitude, longitude: merchant.longitude },
                            { latitude: customer.latitude, longitude: customer.longitude }
                        ],
                        color: '#1890ff',
                        width: 4,
                        arrowLine: true
                    }];
            }
            this.setData({ markers, polyline, includePoints });
        }
    }
});
