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
const trust_score_system_1 = require("../../../api/trust-score-system");
const app = getApp();
Page({
    data: {
        score: 0,
        level: '普通会员',
        levelDesc: '信用良好',
        history: [],
        privileges: [],
        loading: false,
        navBarHeight: 88
    },
    onLoad() {
        this.loadCreditInfo();
    },
    onShow() {
        this.loadCreditInfo();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadCreditInfo() {
        return __awaiter(this, void 0, void 0, function* () {
            var _a;
            this.setData({ loading: true });
            try {
                const userId = (_a = app.globalData.userInfo) === null || _a === void 0 ? void 0 : _a.id;
                if (!userId) {
                    wx.showToast({ title: '请先登录', icon: 'none' });
                    this.setData({ loading: false });
                    return;
                }
                // 获取信任分档案和历史
                const [profile, historyResponse] = yield Promise.all([
                    trust_score_system_1.trustScoreSystemService.getTrustScoreProfile('customer', userId),
                    trust_score_system_1.trustScoreSystemService.getTrustScoreHistory('customer', userId, 1, 20)
                ]);
                // 转换历史记录格式
                const history = historyResponse.history.map(h => {
                    var _a;
                    return ({
                        id: h.id,
                        type: h.change_amount >= 0 ? 'REWARD' : 'PENALTY',
                        amount: h.change_amount,
                        reason: h.reason,
                        related_id: (_a = h.related_order_id) === null || _a === void 0 ? void 0 : _a.toString(),
                        created_at: h.created_at
                    });
                });
                // 根据分数计算等级和描述
                const { level, levelDesc, privileges } = this.calculateLevelInfo(profile.current_score);
                this.setData({
                    score: profile.current_score,
                    level,
                    levelDesc,
                    privileges,
                    history,
                    loading: false
                });
            }
            catch (error) {
                console.error('加载信用信息失败:', error);
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    calculateLevelInfo(score) {
        if (score >= 800) {
            return {
                level: '钻石会员',
                levelDesc: '信用极佳',
                privileges: ['优先派单', '免押金', '极速退款', '专属客服', '生日礼包']
            };
        }
        else if (score >= 700) {
            return {
                level: '黄金会员',
                levelDesc: '信用极好',
                privileges: ['优先派单', '免押金', '极速退款']
            };
        }
        else if (score >= 600) {
            return {
                level: '白银会员',
                levelDesc: '信用良好',
                privileges: ['优先派单', '快速退款']
            };
        }
        else if (score >= 500) {
            return {
                level: '普通会员',
                levelDesc: '信用一般',
                privileges: ['正常服务']
            };
        }
        else {
            return {
                level: '受限会员',
                levelDesc: '信用较低',
                privileges: ['部分功能受限']
            };
        }
    }
});
