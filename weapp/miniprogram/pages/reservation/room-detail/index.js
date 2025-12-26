"use strict";
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
Object.defineProperty(exports, "__esModule", { value: true });
const reservation_1 = require("../../../api/reservation");
Page({
    data: {
        roomId: '',
        room: null,
        navBarHeight: 88,
        loading: false
    },
    onLoad(options) {
        if (options.id) {
            this.setData({ roomId: options.id });
            this.loadRoomDetail(options.id);
        }
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadRoomDetail(id) {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const room = yield (0, reservation_1.getRoomDetail)(id);
                this.setData({
                    room,
                    loading: false
                });
            }
            catch (error) {
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    onBook() {
        const { room } = this.data;
        if (room) {
            wx.navigateTo({
                url: `/pages/reservation/confirm/index?roomId=${room.id}&roomName=${room.name}&deposit=${room.deposit}`
            });
        }
    }
});
