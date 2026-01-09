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
const trust_score_system_1 = require("../../api/trust-score-system");
Page({
    data: {
        score: null,
        history: [],
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
    loadCredit() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                // Assume user type is 'customer' for this demo (mapped to UserRole 'customer')
                const res = yield trust_score_system_1.trustScoreSystemService.getTrustScoreProfile('customer', 123); // Mock ID 123
                this.setData({
                    score: res,
                    chartValue: (res.current_score / 950) * 100, // Normalize 350-950 to 0-100 roughly for ring
                    loading: false
                });
            }
            catch (error) {
                console.error(error);
                this.setData({ loading: false });
            }
        });
    },
    loadHistory() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const history = yield trust_score_system_1.trustScoreSystemService.getTrustScoreHistory('customer', 123, 1, 10);
                this.setData({ history: history.history || [] });
            }
            catch (error) {
                console.error(error);
                // Fallback mock
                this.setData({
                    history: [
                        { id: 1, type: 'order_complete', change_amount: 5, change_reason: '完成订单 ORD-1002', created_at: '2023-10-26' },
                        { id: 2, type: 'abuse', change_amount: -10, change_reason: '恶意差评', created_at: '2023-10-20' }
                    ]
                });
            }
        });
    }
});
