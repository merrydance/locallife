"use strict";
Component({
    properties: {
    // No properties needed for now
    },
    methods: {
        onSearch() {
            this.triggerEvent('search');
        }
    }
});
