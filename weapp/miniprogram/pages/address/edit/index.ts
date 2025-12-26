import { AddressService } from '../../../api/address';

Page({
    data: {
        id: null as number | null,
        formData: {
            contact_name: '',
            contact_phone: '',
            province: '',
            city: '',
            district: '',
            address: '', // detail
            latitude: 0,
            longitude: 0,
            is_default: false,
            tag: ''
        },
        addressDisplay: '', // Province City District
        tags: ['家', '公司', '学校', '其他']
    },

    onLoad(options: any) {
        if (options.id) {
            this.setData({ id: parseInt(options.id) });
            this.loadDetail(parseInt(options.id));
            wx.setNavigationBarTitle({ title: '编辑地址' });
        } else {
            wx.setNavigationBarTitle({ title: '新增地址' });
        }
    },

    async loadDetail(id: number) {
        try {
            const data = await AddressService.getAddressDetail(id);
            this.setData({
                formData: { ...data, tag: data.tag || '' },
                addressDisplay: `${data.province} ${data.city} ${data.district}`
            });
        } catch (error) {
            console.error(error);
            wx.showToast({ title: '加载失败', icon: 'none' });
        }
    },

    onChooseLocation() {
        const that = this;
        wx.chooseLocation({
            success: (res) => {
                // Parse simple logic, in reality might need reverse geocoding if result is imprecise
                // Assuming res.address contains "ProvinceCityDistrict..." or we just store full string
                // For this demo, let's just use what we get and maybe not parse strictly if UI doesn't require separate fields strictly
                // Or simplified: Just put full address in 'address' and stub P/C/D

                // Real implementation: Use QQMapSDK to reverse geocode lat/lng to get structured address components.
                // Here we simplify by mock splitting or just trusting user input.

                that.setData({
                    'formData.latitude': res.latitude,
                    'formData.longitude': res.longitude,
                    'formData.address': res.name || res.address,
                    addressDisplay: res.address // Simplified
                });
            },
            fail: (err) => {
                console.log('chooseLocation fail', err);
                // Might need to authorize scope.userLocation
            }
        });
    },

    onInput(e: any) {
        const field = e.currentTarget.dataset.field;
        this.setData({
            [`formData.${field}`]: e.detail.value
        });
    },

    onSwitchChange(e: any) {
        this.setData({
            'formData.is_default': e.detail.value
        });
    },

    onTagSelect(e: any) {
        const tag = e.currentTarget.dataset.tag;
        this.setData({
            'formData.tag': tag
        });
    },

    async onSave() {
        const { id, formData } = this.data;

        // Basic validation
        if (!formData.contact_name || !formData.contact_phone || !formData.address) {
            wx.showToast({ title: '请完善地址信息', icon: 'none' });
            return;
        }

        try {
            wx.showLoading({ title: '保存中' });

            // Mock filling P/C/D if missing
            if (!formData.province) {
                formData.province = '北京市';
                formData.city = '北京市';
                formData.district = '朝阳区';
            }

            if (id) {
                await AddressService.updateAddress(id, formData);
            } else {
                await AddressService.createAddress(formData);
            }

            wx.showToast({ title: '保存成功', icon: 'success' });
            setTimeout(() => {
                wx.navigateBack();
            }, 1500);

        } catch (error: any) {
            wx.showToast({ title: error.message || '保存失败', icon: 'none' });
        } finally {
            wx.hideLoading();
        }
    },

    async onDelete() {
        if (!this.data.id) return;

        wx.showModal({
            title: '提示',
            content: '确定要删除该地址吗？',
            success: async (res) => {
                if (res.confirm) {
                    try {
                        wx.showLoading({ title: '删除中' });
                        await AddressService.deleteAddress(this.data.id!);
                        wx.showToast({ title: '删除成功', icon: 'success' });
                        setTimeout(() => wx.navigateBack(), 1500);
                    } catch (error: any) {
                        wx.showToast({ title: '删除失败', icon: 'none' });
                    } finally {
                        wx.hideLoading();
                    }
                }
            }
        });
    }
});
