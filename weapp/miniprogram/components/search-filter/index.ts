Component({
  properties: {
    keyword: {
      type: String,
      value: ''
    }
  },

  data: {
    showFilter: false,
    filters: {
      date: '',
      mealPeriod: 'all',
      guestCount: 4,
      priceRange: { min: 0, max: 500 }
    }
  },

  methods: {
    onSearchInput(e: WechatMiniprogram.Input) {
      const keyword = e.detail.value
      this.triggerEvent('search', { keyword })
    },

    toggleFilter() {
      this.setData({ showFilter: !this.data.showFilter })
    },

    onDateChange(e: WechatMiniprogram.PickerChange) {
      this.setData({ 'filters.date': e.detail.value })
      this.applyFilters()
    },

    onMealPeriodChange(e: WechatMiniprogram.CustomEvent) {
      const { value } = e.currentTarget.dataset
      this.setData({ 'filters.mealPeriod': value })
      this.applyFilters()
    },

    onGuestCountChange(e: WechatMiniprogram.SliderChange) {
      this.setData({ 'filters.guestCount': e.detail.value })
    },

    onGuestCountConfirm() {
      this.applyFilters()
    },

    applyFilters() {
      this.triggerEvent('filter', { filters: this.data.filters })
    },

    resetFilters() {
      this.setData({
        filters: {
          date: '',
          mealPeriod: 'all',
          guestCount: 4,
          priceRange: { min: 0, max: 500 }
        }
      })
      this.applyFilters()
    }
  }
})
