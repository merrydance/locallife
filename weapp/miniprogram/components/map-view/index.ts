Component({
  properties: {
    task: {
      type: Object,
      value: {},
      observer: 'updateMap'
    }
  },

  data: {
    markers: [] as Array<Record<string, unknown>>,
    polyline: [] as Array<Record<string, unknown>>,
    latitude: 39.980014,
    longitude: 116.313082
  },

  methods: {
    updateMap(task: Record<string, unknown>) {
      if (!task) return

      // Mock coordinates for demo
      const start = { latitude: 39.980014, longitude: 116.313082 } // Shop
      const end = { latitude: 39.982014, longitude: 116.315082 }   // Customer

      const markers = [
        {
          id: 1,
          latitude: start.latitude,
          longitude: start.longitude,
          title: '取货点',
          iconPath: '/assets/marker_shop.png', // Need to ensure assets exist or use default
          width: 30,
          height: 30
        },
        {
          id: 2,
          latitude: end.latitude,
          longitude: end.longitude,
          title: '送货点',
          iconPath: '/assets/marker_user.png',
          width: 30,
          height: 30
        }
      ]

      const polyline = [{
        points: [start, { latitude: 39.981000, longitude: 116.314000 }, end],
        color: '#0052D9',
        width: 4,
        arrowLine: true
      }]

      this.setData({
        markers,
        polyline,
        latitude: start.latitude,
        longitude: start.longitude
      })
    }
  }
})
