"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.getSafeAreaInfo = getSafeAreaInfo;
function getSafeAreaInfo() {
    const windowInfo = wx.getWindowInfo();
    const _appBaseInfo = wx.getAppBaseInfo();
    const menuButton = wx.getMenuButtonBoundingClientRect();
    const statusBarHeight = windowInfo.statusBarHeight || 0;
    const navBarContentHeight = menuButton.height + (menuButton.top - statusBarHeight) * 2;
    const navBarHeight = statusBarHeight + navBarContentHeight;
    const bottomSafeHeight = windowInfo.screenHeight - windowInfo.safeArea.bottom;
    return {
        statusBarHeight,
        navBarHeight,
        navBarContentHeight,
        bottomSafeHeight
    };
}
