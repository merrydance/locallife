import { AddressService, Address } from '../../../api/address';

Page({
    data: {
        addresses: [] as Address[],
        loading: true
    },

    onShow() {
        this.loadAddresses();
    },

    async loadAddresses() {
        this.setData({ loading: true });
        try {
            const addresses = await AddressService.getAddresses();
            this.setData({ addresses, loading: false });
        } catch (error) {
            console.error(error);
            this.setData({ loading: false });
            wx.showToast({ title: '加载失败', icon: 'none' });
        }
    },

    async onSetDefault(e: any) {
        const id = e.currentTarget.dataset.id;
        try {
            wx.showLoading({ title: '设置中' });
            await AddressService.setDefaultAddress(id);
            this.loadAddresses(); // Reload to reflect changes
            wx.showToast({ title: '已设置默认', icon: 'success' });
        } catch (error: any) {
            wx.showToast({ title: '设置失败', icon: 'none' });
        } finally {
            wx.hideLoading();
        }
    },

    onEdit(e: any) {
        const id = e.currentTarget.dataset.id;
        wx.navigateTo({ url: `/pages/address/edit/index?id=${id}` });
    },

    onAdd() {
        wx.navigateTo({ url: '/pages/address/edit/index' });
    }
});
