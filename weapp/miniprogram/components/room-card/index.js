"use strict";
// / <reference path="../../../typings/index.d.ts" />
Component({
    properties: {
        room: {
            type: Object,
            value: {}
        }
    },
    methods: {
        onTap() {
            const room = this.data.room;
            if (room === null || room === void 0 ? void 0 : room.id) {
                wx.navigateTo({
                    url: `/pages/reservation/room-detail/index?id=${room.id}`
                });
            }
        }
    }
});
