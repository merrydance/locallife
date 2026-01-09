import { ReviewService } from '../../../api/review';

Page({
    data: {
        orderId: 0,
        rating: 5,
        content: '',
        fileList: [] as any[], // For t-upload
        uploadAction: 'https://myserver.com/upload', // Mock upload URL
        submitting: false
    },

    onLoad(options: any) {
        if (options.orderId) {
            this.setData({ orderId: parseInt(options.orderId) });
        }
    },

    onRateChange(e: any) {
        this.setData({ rating: e.detail.value });
    },

    onContentInput(e: any) {
        this.setData({ content: e.detail.value });
    },

    onAddFile(e: any) {
        const { fileList } = this.data;
        const { files } = e.detail;

        // Mock upload: usually we upload here and get URL. 
        // For simple demo, we just append local path and pretend it's a URL in submit.
        // In real app, use wx.uploadFile

        const newFiles = files.map((file: any) => ({
            ...file,
            url: file.url, // In real world this would be result of upload
            status: 'done'
        }));

        this.setData({
            fileList: [...fileList, ...newFiles]
        });
    },

    onRemoveFile(e: any) {
        const { index } = e.detail;
        const { fileList } = this.data;
        fileList.splice(index, 1);
        this.setData({ fileList });
    },

    async onSubmit() {
        const { orderId, rating, content, fileList } = this.data;

        if (!content) {
            wx.showToast({ title: '虽然不想写，但还是写点评价吧', icon: 'none' });
            return;
        }

        this.setData({ submitting: true });

        try {
            const images = fileList.map(f => f.url);

            await ReviewService.createReview({
                order_id: orderId,
                rating: rating as any,
                content,
                images
            });

            wx.showToast({ title: '评价成功', icon: 'success' });

            setTimeout(() => {
                wx.navigateBack();
            }, 1500);

        } catch (error: any) {
            wx.showToast({ title: error.message || '评价失败', icon: 'none' });
        } finally {
            this.setData({ submitting: false });
        }
    }
});
