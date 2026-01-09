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
Object.defineProperty(exports, "__esModule", { value: true });
const responsive_1 = require("@/utils/responsive");
const echarts = __importStar(require("../../libs/echarts"));
function initChart(canvas, width, height, dpr) {
    const chart = echarts.init(canvas, null, {
        width,
        height,
        devicePixelRatio: dpr
    });
    canvas.setChart(chart);
    const option = {
        tooltip: {
            trigger: 'axis'
        },
        grid: {
            left: '3%',
            right: '4%',
            bottom: '3%',
            containLabel: true
        },
        xAxis: {
            type: 'category',
            boundaryGap: false,
            data: ['11-14', '11-15', '11-16', '11-17', '11-18', '11-19', '11-20']
        },
        yAxis: {
            type: 'value'
        },
        series: [
            {
                name: '销售额',
                type: 'line',
                stack: 'Total',
                emphasis: {
                    focus: 'series'
                },
                data: [1200, 1320, 1010, 1340, 900, 2300, 2100],
                itemStyle: { color: '#0052D9' },
                areaStyle: {
                    color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                        { offset: 0, color: 'rgba(0, 82, 217, 0.5)' },
                        { offset: 1, color: 'rgba(0, 82, 217, 0)' }
                    ])
                }
            },
            {
                name: '订单量',
                type: 'line',
                stack: 'Total',
                emphasis: {
                    focus: 'series'
                },
                data: [220, 182, 191, 234, 290, 330, 310],
                itemStyle: { color: '#00A870' },
                areaStyle: {
                    color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                        { offset: 0, color: 'rgba(0, 168, 112, 0.5)' },
                        { offset: 1, color: 'rgba(0, 168, 112, 0)' }
                    ])
                }
            }
        ]
    };
    chart.setOption(option);
    return chart;
}
Page({
    data: {
        ec: {
            onInit: initChart
        },
        isLargeScreen: false,
        navBarHeight: 88,
        salesData: [
            { date: '2024-11-20', gmv: 2100, orders: 310, avg: 6.7 },
            { date: '2024-11-19', gmv: 2300, orders: 330, avg: 6.9 },
            { date: '2024-11-18', gmv: 900, orders: 290, avg: 3.1 },
            { date: '2024-11-17', gmv: 1340, orders: 234, avg: 5.7 },
            { date: '2024-11-16', gmv: 1010, orders: 191, avg: 5.2 },
            { date: '2024-11-15', gmv: 1320, orders: 182, avg: 7.2 },
            { date: '2024-11-14', gmv: 1200, orders: 220, avg: 5.4 }
        ]
    },
    onLoad() {
        this.setData({ isLargeScreen: (0, responsive_1.isLargeScreen)() });
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    }
});
