import { request } from '@/utils/request';

Page({
    data: {
        bindCode: '',
        loading: false,
        claimed: false,
        merchantName: ''
    },

    onInputChange(e: WechatMiniprogram.CustomEvent) {
        this.setData({ bindCode: e.detail.value });
    },

    async onScan() {
        try {
            const result = await wx.scanCode({ onlyFromCamera: false });
            // 认领码可能直接是码值或包含路径
            let code = result.result;
            if (code.includes('code=')) {
                code = code.split('code=')[1].split('&')[0];
            }
            this.setData({ bindCode: code });
            this.claimBoss();
        } catch (err: any) {
            if (err.errMsg && !err.errMsg.includes('cancel')) {
                wx.showToast({ title: '扫码失败', icon: 'none' });
            }
        }
    },

    async claimBoss() {
        const { bindCode } = this.data;
        if (!bindCode || bindCode.length < 6) {
            wx.showToast({ title: '请输入有效的认领码', icon: 'none' });
            return;
        }

        this.setData({ loading: true });
        try {
            const res = await request<{ merchant_id: number; merchant_name: string; message: string }>({
                url: '/v1/claim-boss',
                method: 'POST',
                data: { bind_code: bindCode }
            });

            this.setData({
                loading: false,
                claimed: true,
                merchantName: res.merchant_name
            });

            wx.showToast({ title: '认领成功', icon: 'success' });
        } catch (err: any) {
            this.setData({ loading: false });

            if (err?.statusCode === 409) {
                wx.showModal({
                    title: '已认领',
                    content: '您已经认领过该店铺，无需重复认领',
                    showCancel: false
                });
                return;
            }

            if (err?.statusCode === 400) {
                wx.showToast({ title: err.data?.message || '认领码无效或已过期', icon: 'none' });
                return;
            }

            wx.showToast({ title: '认领失败，请重试', icon: 'none' });
        }
    },

    onGoToWorkspace() {
        // 跳转到 Boss 工作台
        wx.reLaunch({ url: '/pages/boss/workspace/index' });
    },

    onGoBack() {
        wx.navigateBack();
    }
});
