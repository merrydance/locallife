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
const responsive_1 = require("../../../utils/responsive");
const trust_score_system_1 = require("../../../api/trust-score-system");
const app = getApp();
Page({
    data: {
        score: 0,
        level: '健康',
        levelDesc: '经营状况良好',
        metrics: [],
        violations: [],
        isLargeScreen: false,
        navBarHeight: 88,
        loading: false
    },
    onLoad() {
        this.setData({ isLargeScreen: (0, responsive_1.isLargeScreen)() });
        this.loadHealthInfo();
    },
    onShow() {
        this.loadHealthInfo();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadHealthInfo() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const merchantId = app.globalData.merchantId;
                if (!merchantId) {
                    wx.showToast({ title: '请先登录商户', icon: 'none' });
                    this.setData({ loading: false });
                    return;
                }
                // 获取商户信任分档案和历史
                const merchantIdNum = Number(merchantId);
                const [profile, historyResponse] = yield Promise.all([
                    trust_score_system_1.trustScoreSystemService.getTrustScoreProfile('merchant', merchantIdNum),
                    trust_score_system_1.trustScoreSystemService.getTrustScoreHistory('merchant', merchantIdNum, 1, 20)
                ]);
                // 计算等级信息
                const { level, levelDesc } = this.calculateLevelInfo(profile.current_score);
                // 计算违规和警告次数
                const negativeChanges = historyResponse.history.filter(h => h.change_amount < 0);
                const violationCount = negativeChanges.filter(h => Math.abs(h.change_amount) >= 10).length;
                const warningCount = negativeChanges.filter(h => Math.abs(h.change_amount) < 10).length;
                // 构建指标数据
                const metrics = [
                    {
                        label: '信任分',
                        value: profile.current_score.toString(),
                        status: profile.current_score >= 80 ? 'GOOD' : profile.current_score >= 60 ? 'WARNING' : 'BAD'
                    },
                    {
                        label: '行为分',
                        value: profile.score_breakdown.behavior_score.toString(),
                        status: profile.score_breakdown.behavior_score >= 80 ? 'GOOD' : profile.score_breakdown.behavior_score >= 60 ? 'WARNING' : 'BAD'
                    },
                    {
                        label: '违规次数',
                        value: violationCount.toString(),
                        status: violationCount === 0 ? 'GOOD' : violationCount <= 2 ? 'WARNING' : 'BAD'
                    },
                    {
                        label: '警告次数',
                        value: warningCount.toString(),
                        status: warningCount === 0 ? 'GOOD' : warningCount <= 3 ? 'WARNING' : 'BAD'
                    }
                ];
                // 转换违规记录
                const violations = negativeChanges
                    .slice(0, 5)
                    .map(h => ({
                    id: h.id,
                    type: Math.abs(h.change_amount) >= 10 ? 'VIOLATION' : 'WARNING',
                    title: h.change_reason,
                    desc: `扣除${Math.abs(h.change_amount)}分`,
                    created_at: h.created_at
                }));
                this.setData({
                    score: profile.current_score,
                    level,
                    levelDesc,
                    metrics,
                    violations,
                    loading: false
                });
            }
            catch (error) {
                console.error('加载健康信息失败:', error);
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    calculateLevelInfo(score) {
        if (score >= 90) {
            return { level: '优秀', levelDesc: '经营状况优秀' };
        }
        else if (score >= 80) {
            return { level: '健康', levelDesc: '经营状况良好' };
        }
        else if (score >= 60) {
            return { level: '一般', levelDesc: '需要改进' };
        }
        else {
            return { level: '警告', levelDesc: '存在风险' };
        }
    },
    onAppeal(e) {
        const { id } = e.currentTarget.dataset;
        wx.navigateTo({ url: `/pages/merchant/appeals/index?id=${id}` });
    }
});
