"use strict";
/**
 * 套餐管理接口
 * 重新导出dish.ts中的套餐相关功能，保持向后兼容
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.default = exports.searchCombos = exports.getRecommendedCombos = exports.ComboManagementService = void 0;
// 重新导出套餐相关的类型和服务
var dish_1 = require("./dish");
Object.defineProperty(exports, "ComboManagementService", { enumerable: true, get: function () { return dish_1.ComboManagementService; } });
Object.defineProperty(exports, "getRecommendedCombos", { enumerable: true, get: function () { return dish_1.getRecommendedCombos; } });
Object.defineProperty(exports, "searchCombos", { enumerable: true, get: function () { return dish_1.searchCombos; } });
// 兼容性别名
var dish_2 = require("./dish");
Object.defineProperty(exports, "default", { enumerable: true, get: function () { return dish_2.ComboManagementService; } });
