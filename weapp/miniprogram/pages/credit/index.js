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
Page({
    data: {
        score: null,
        history: [],
        loading: false,
        chartValue: 0,
            this.showDeprecatedNotice();
            '100%': '#FFB000',
        showDeprecatedNotice() {
            this.setData({
                score: null,
                history: [],
                loading: false,
                chartValue: 0
            });
            wx.showToast({ title: '信用分功能已下线', icon: 'none' });
        }
                        { id: 2, type: 'abuse', change_amount: -10, change_reason: '恶意差评', created_at: '2023-10-20' }
                    ]
                });
            }
        });
    }
});
