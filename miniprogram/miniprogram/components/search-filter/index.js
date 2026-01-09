"use strict";
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
        onSearchInput(e) {
            const keyword = e.detail.value;
            this.triggerEvent('search', { keyword });
        },
        toggleFilter() {
            this.setData({ showFilter: !this.data.showFilter });
        },
        onDateChange(e) {
            this.setData({ 'filters.date': e.detail.value });
            this.applyFilters();
        },
        onMealPeriodChange(e) {
            const { value } = e.currentTarget.dataset;
            this.setData({ 'filters.mealPeriod': value });
            this.applyFilters();
        },
        onGuestCountChange(e) {
            this.setData({ 'filters.guestCount': e.detail.value });
        },
        onGuestCountConfirm() {
            this.applyFilters();
        },
        applyFilters() {
            this.triggerEvent('filter', { filters: this.data.filters });
        },
        resetFilters() {
            this.setData({
                filters: {
                    date: '',
                    mealPeriod: 'all',
                    guestCount: 4,
                    priceRange: { min: 0, max: 500 }
                }
            });
            this.applyFilters();
        }
    }
});
