import { trustScoreSystemService, TrustScoreProfileResponse } from '../../api/trust-score-system';

Page({
    data: {
        score: null as TrustScoreProfileResponse | null,
        history: [] as any[],
        loading: false,
        chartValue: 0,
        gradientColor: {
            '0%': '#E34D59',
            '100%': '#FFB000',
        }
    },

    onLoad() {
        this.loadCredit();
        this.loadHistory();
    },

    async loadCredit() {
        this.setData({ loading: true });
        try {
            // Assume user type is 'customer' for this demo (mapped to UserRole 'customer')
            const res = await trustScoreSystemService.getTrustScoreProfile('customer', 123); // Mock ID 123
            this.setData({
                score: res,
                chartValue: (res.current_score / 950) * 100, // Normalize 350-950 to 0-100 roughly for ring
                loading: false
            });
        } catch (error) {
            console.error(error);
            this.setData({ loading: false });
        }
    },

    async loadHistory() {
        try {
            const history = await trustScoreSystemService.getTrustScoreHistory('customer', 123, 1, 10);
            this.setData({ history: history.history || [] });
        } catch (error) {
            console.error(error);
            // Fallback mock
            this.setData({
                history: [
                    { id: 1, type: 'order_complete', change_amount: 5, change_reason: '完成订单 ORD-1002', created_at: '2023-10-26' },
                    { id: 2, type: 'abuse', change_amount: -10, change_reason: '恶意差评', created_at: '2023-10-20' }
                ]
            });
        }
    }
});
