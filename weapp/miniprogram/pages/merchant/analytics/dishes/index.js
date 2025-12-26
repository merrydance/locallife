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
            trigger: 'item'
        },
        legend: {
            top: '5%',
            left: 'center'
        },
        series: [
            {
                name: '菜品分类',
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
                    { value: 1048, name: '热菜' },
                    { value: 735, name: '凉菜' },
                    { value: 580, name: '主食' },
                    { value: 484, name: '饮料' },
                    { value: 300, name: '甜点' }
                ]
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
        topDishes: [
            { rank: 1, name: '招牌红烧肉', sales: 520, revenue: 19760, trend: 'up' },
            { rank: 2, name: '糖醋排骨', sales: 430, revenue: 13760, trend: 'up' },
            { rank: 3, name: '宫保鸡丁', sales: 310, revenue: 8680, trend: 'down' },
            { rank: 4, name: '麻婆豆腐', sales: 280, revenue: 3360, trend: 'up' },
            { rank: 5, name: '米饭', sales: 1200, revenue: 2400, trend: 'flat' }
        ]
    },
    onLoad() {
        this.setData({ isLargeScreen: (0, responsive_1.isLargeScreen)() });
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    }
});
