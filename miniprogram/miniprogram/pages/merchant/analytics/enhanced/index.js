"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
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
const responsive_1 = require("@/utils/responsive");
const echarts = __importStar(require("../../libs/echarts"));
const merchant_1 = require("../../../api/merchant");
const error_handler_1 = require("../../../utils/error-handler");
const app = getApp();
let chartLine = null;
let chartPie = null;
function initChart(canvas, width, height, dpr) {
    const chart = echarts.init(canvas, null, {
        width,
        height,
        devicePixelRatio: dpr
    });
    canvas.setChart(chart);
    const option = {
        grid: {
            left: '3%',
            right: '4%',
            bottom: '3%',
            containLabel: true
        },
        tooltip: {
            trigger: 'axis'
        },
        xAxis: {
            type: 'category',
            data: [] // Filled by API
        },
        yAxis: {
            type: 'value'
        },
        series: [{
                data: [], // Filled by API
                type: 'line',
                smooth: true,
                itemStyle: {
                    color: '#0052D9'
                },
                areaStyle: {
                    color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                        { offset: 0, color: 'rgba(0, 82, 217, 0.5)' },
                        { offset: 1, color: 'rgba(0, 82, 217, 0)' }
                    ])
                }
            }]
    };
    chart.setOption(option);
    chartLine = chart;
    return chart;
}
function initPieChart(canvas, width, height, dpr) {
    const chart = echarts.init(canvas, null, {
        width,
        height,
        devicePixelRatio: dpr
    });
    canvas.setChart(chart);
    // Mock data for Pie Chart (API doesn't provide time distribution yet)
    const option = {
        tooltip: {
            trigger: 'item'
        },
        legend: {
            bottom: '0%',
            left: 'center'
        },
        series: [
            {
                name: '时段分布',
                type: 'pie',
                radius: ['40%', '70%'],
                avoidLabelOverlap: false,
                itemStyle: {
                    borderRadius: 10,
                    borderColor: '#fff',
                    borderWidth: 2
                },
                label: {
                    show: false,
                    position: 'center'
                },
                emphasis: {
                    label: {
                        show: true,
                        fontSize: '20',
                        fontWeight: 'bold'
                    }
                },
                labelLine: {
                    show: false
                },
                data: [
                    { value: 1048, name: '午餐' },
                    { value: 735, name: '晚餐' },
                    { value: 580, name: '夜宵' },
                    { value: 484, name: '下午茶' }
                ]
            }
        ]
    };
    chart.setOption(option);
    chartPie = chart;
    return chart;
}
Page({
    data: {
        ec: {
            onInit: initChart
        },
        ecPie: {
            onInit: initPieChart
        },
        isLargeScreen: false,
        navBarHeight: 88,
        metrics: [
            { label: '今日GMV', value: '-', change: '-', trend: 'up' },
            { label: '今日订单', value: '-', change: '-', trend: 'up' },
            { label: '待处理', value: '-', change: '-', trend: 'up' },
            { label: '近7日', value: '趋势图', change: '-', trend: 'up' }
        ]
    },
    onLoad() {
        this.setData({ isLargeScreen: (0, responsive_1.isLargeScreen)() });
        if (app.globalData.merchantId) {
            this.loadData();
        }
        else {
            app.userInfoReadyCallback = () => {
                if (app.globalData.merchantId) {
                    this.loadData();
                }
            };
        }
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadData() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const merchantId = app.globalData.merchantId;
                const res = yield (0, merchant_1.getMerchantDashboard)(merchantId);
                // Update Metrics
                const metrics = [
                    { label: '今日GMV', value: `¥${(res.today_sales / 100).toFixed(2)}`, change: '-', trend: 'up' },
                    { label: '今日订单', value: String(res.today_orders), change: '-', trend: 'up' },
                    { label: '待处理', value: String(res.pending_orders), change: '-', trend: 'down' }, // Assuming lower is better or just trend
                    { label: '近7日', value: '趋势图', change: '-', trend: 'up' }
                ];
                this.setData({ metrics });
                // Update Line Chart
                if (chartLine && res.seven_days_sales) {
                    const dates = res.seven_days_sales.map((d) => d.date.slice(5)); // MM-DD
                    const sales = res.seven_days_sales.map((d) => d.amount / 100); // Yuan
                    chartLine.setOption({
                        xAxis: {
                            data: dates
                        },
                        series: [{
                                data: sales
                            }]
                    });
                }
            }
            catch (error) {
                error_handler_1.ErrorHandler.handle(error, 'Analytics.loadData');
            }
        });
    }
});
