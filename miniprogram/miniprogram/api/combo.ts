/**
 * 套餐管理接口
 * 重新导出dish.ts中的套餐相关功能，保持向后兼容
 */

// 重新导出套餐相关的类型和服务
export {
    ComboSetResponse,
    ComboSetWithDetailsResponse,
    UpdateComboSetRequest,
    ComboManagementService,
    getRecommendedCombos,
    searchCombos,
    SearchComboItem,
    ComboSearchResult
} from './dish'

// 兼容性别名
export { ComboManagementService as default } from './dish'