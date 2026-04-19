Component({
  properties: {
    keyword: {
      type: String,
      value: ''
    }
  },

  data: {
    showFilter: false,
    datePickerVisible: false,
    filters: {
      date: '',
      mealPeriod: 'all',
      guestCount: 4,
      priceRange: { min: 0, max: 500 }
    }
  },

  methods: {
    onSearchInput(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
      const keyword = e.detail.value
      this.triggerEvent('search', { keyword })
    },

    toggleFilter() {
      this.setData({ showFilter: !this.data.showFilter })
    },

    openDatePicker() {
      this.setData({ datePickerVisible: true })
    },

    closeDatePicker() {
      this.setData({ datePickerVisible: false })
    },

    onDateConfirm(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
      this.setData({
        'filters.date': String(e.detail.value || ''),
        datePickerVisible: false
      })
      this.applyFilters()
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

    onGuestCountChange(e: WechatMiniprogram.CustomEvent<{ value?: number }>) {
      this.setData({ 'filters.guestCount': e.detail.value })
      this.applyFilters()
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
