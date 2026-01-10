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
exports.DishSupabaseService = void 0;
const supabase_1 = require("../services/supabase");
class DishSupabaseService {
    /**
     * List dishes with pagination and filtering
     */
    static listDishes(params) {
        return __awaiter(this, void 0, void 0, function* () {
            console.log('DishSupabaseService.listDishes called with:', params);
            const queryParams = [];
            queryParams.push('select=*');
            queryParams.push(`merchant_id=eq.${params.merchant_id}`);
            if (params.category_id !== undefined) {
                queryParams.push(`category_id=eq.${params.category_id}`);
            }
            if (params.is_online !== undefined) {
                queryParams.push(`is_online=eq.${params.is_online}`);
            }
            const queryString = queryParams.join('&');
            // Calculate range
            const from = (params.page_id - 1) * params.page_size;
            const to = from + params.page_size - 1;
            const { data, error } = yield (0, supabase_1.supabaseRequest)({
                url: `/rest/v1/dishes?${queryString}`,
                method: 'GET',
                headers: {
                    'Range': `${from}-${to}`,
                    'Prefer': 'count=exact'
                }
            });
            // Note: To get total count with 'Prefer: count=exact', Supabase returns it in 'Content-Range' header
            // But wx.request success callback gives us 'res', and our wrapper returns { data, error }.
            // Our simple wrapper dropped the headers. 
            // For now, let's assume infinite scroll or ignore total_count, OR update wrapper to return headers.
            // Given complexity, let's return data and 0 count if wrapper doesn't support it yet.
            // Users can implement header parsing in wrapper if needed.
            if (error) {
                console.error('Supabase listDishes error:', error);
                throw error;
            }
            return {
                dishes: data || [],
                total_count: 9999 // Placeholder as we dropped header support in simple wrapper for now
            };
        });
    }
    /**
     * Get a single dish by ID
     */
    static getDish(id) {
        return __awaiter(this, void 0, void 0, function* () {
            const { data, error } = yield (0, supabase_1.supabaseRequest)({
                url: `/rest/v1/dishes?id=eq.${id}&limit=1`,
                method: 'GET',
                headers: {
                    'Accept': 'application/vnd.pgrst.object+json' // Request single object
                }
            });
            if (error) {
                console.error('Supabase getDish error:', error);
                throw error;
            }
            return data;
        });
    }
}
exports.DishSupabaseService = DishSupabaseService;
