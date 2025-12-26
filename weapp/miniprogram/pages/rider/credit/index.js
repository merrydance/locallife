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
const rider_basic_management_1 = require("../../../api/rider-basic-management");
Page({
    data: {
        score: 0,
        level: '普通骑手',
        levelDesc: '服务正常',
        metrics: [],
        history: [],
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
            this.setData({ loading: true });
            try {
                // 获取骑手积分信息和历史
                const [scoreInfo, historyResponse] = yield Promise.all([
                    rider_basic_management_1.riderBasicManagementService.getRiderScore(),
                    rider_basic_management_1.riderBasicManagementService.getScoreHistory({ page_id: 1, page_size: 20 })
                ]);
                // 计算等级信息
                const { level, levelDesc } = this.calculateLevelInfo(scoreInfo.current_score);
                // 构建指标数据
                const metrics = [
                    {
                        label: '当前积分',
                        value: scoreInfo.current_score.toString(),
                        status: scoreInfo.current_score >= 80 ? 'GOOD' : scoreInfo.current_score >= 60 ? 'WARNING' : 'BAD'
                    },
                    {
                        label: '积分等级',
                        value: scoreInfo.score_level,
                        status: 'GOOD'
                    },
                    {
                        label: '可接高值单',
                        value: scoreInfo.can_take_high_value_orders ? '是' : '否',
                        status: scoreInfo.can_take_high_value_orders ? 'GOOD' : 'WARNING'
                    }
                ];
                // 转换历史记录
                const history = historyResponse.history.map(h => ({
                    id: h.id,
                    type: h.score_change >= 0 ? 'REWARD' : 'PENALTY',
                    amount: h.score_change,
                    reason: h.reason,
                    created_at: h.created_at
                }));
                this.setData({
                    score: scoreInfo.current_score,
                    level,
                    levelDesc,
                    metrics,
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
        if (score >= 95) {
            return { level: '钻石骑手', levelDesc: '服务卓越' };
        }
        else if (score >= 85) {
            return { level: '金牌骑手', levelDesc: '服务优异' };
        }
        else if (score >= 75) {
            return { level: '银牌骑手', levelDesc: '服务良好' };
        }
        else if (score >= 60) {
            return { level: '普通骑手', levelDesc: '服务正常' };
        }
        else {
            return { level: '受限骑手', levelDesc: '需要改进' };
        }
    }
});
