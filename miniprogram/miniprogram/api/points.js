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
exports.PointsService = void 0;
const request_1 = require("../utils/request");
class PointsService {
    /**
     * Get points summary (balance)
     */
    static getSummary() {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/user/points/summary',
                method: 'GET'
            });
        });
    }
    /**
     * Get points history
     */
    static getHistory() {
        return __awaiter(this, arguments, void 0, function* (page = 1, pageSize = 20) {
            return (0, request_1.request)({
                url: '/v1/user/points/history',
                method: 'GET',
                data: { page, page_size: pageSize }
            });
        });
    }
}
exports.PointsService = PointsService;
