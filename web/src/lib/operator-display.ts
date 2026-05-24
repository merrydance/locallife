export type SelectOption = {
  value: string;
  label: string;
};

const MERCHANT_STATUS_LABELS: Record<string, string> = {
  pending: "待审核",
  approved: "已通过",
  rejected: "已驳回",
  suspended: "已暂停",
  active: "营业中",
  deactivated: "已停用",
};

const RIDER_STATUS_LABELS: Record<string, string> = {
  pending: "待审核",
  active: "已激活",
  suspended: "已暂停",
  deactivated: "已停用",
};

const APPEAL_STATUS_LABELS: Record<string, string> = {
  pending: "待处理",
  approved: "已通过",
  rejected: "已驳回",
};

const SAFETY_STATUS_LABELS: Record<string, string> = {
  "merchant-suspended": "已熔断",
  investigating: "调查中",
  resolved: "已处置",
};

const SAFETY_LEVEL_LABELS: Record<string, string> = {
  low: "低",
  medium: "中",
  high: "高",
  critical: "紧急",
};

const WITHDRAWAL_STATUS_LABELS: Record<string, string> = {
  pending: "处理中",
  processing: "处理中",
  submitted: "已提交",
  success: "成功",
  failed: "失败",
  rejected: "已拒绝",
};

const RECOVERY_STATUS_LABELS: Record<string, string> = {
  pending: "待追偿",
  appealed: "申诉中",
  paid: "已支付",
  overdue: "已逾期",
  waived: "已核销",
};

const CLAIM_STATUS_LABELS: Record<string, string> = {
  pending: "待处理",
  approved: "已通过",
  "auto-approved": "自动通过",
  rejected: "已驳回",
  paid: "已赔付",
  appealed: "申诉中",
  closed: "已关闭",
};

const ORDER_STATUS_LABELS: Record<string, string> = {
  pending: "待支付",
  paid: "已支付",
  accepted: "已接单",
  preparing: "备餐中",
  delivering: "代取中",
  completed: "已完成",
  cancelled: "已取消",
  refunded: "已退款",
};

const PROFIT_CONFIG_STATUS_LABELS: Record<string, string> = {
  active: "生效中",
  inactive: "未生效",
  pending: "待生效",
  suspended: "已暂停",
};

export const merchantStatusOptions: SelectOption[] = [
  { value: "all", label: "全部状态" },
  { value: "pending", label: "待审核" },
  { value: "approved", label: "已通过" },
  { value: "rejected", label: "已驳回" },
  { value: "suspended", label: "已暂停" },
];

export const riderStatusOptions: SelectOption[] = [
  { value: "all", label: "全部状态" },
  { value: "pending", label: "待审核" },
  { value: "active", label: "已激活" },
  { value: "suspended", label: "已暂停" },
  { value: "deactivated", label: "已停用" },
];

export const appealStatusOptions: SelectOption[] = [
  { value: "all", label: "全部状态" },
  { value: "pending", label: "待处理" },
  { value: "approved", label: "已通过" },
  { value: "rejected", label: "已驳回" },
];

export const safetyStatusOptions: SelectOption[] = [
  { value: "all", label: "全部状态" },
  { value: "merchant-suspended", label: "已熔断" },
  { value: "investigating", label: "调查中" },
  { value: "resolved", label: "已处置" },
];

export const safetyLevelOptions: SelectOption[] = [
  { value: "low", label: "低" },
  { value: "medium", label: "中" },
  { value: "high", label: "高" },
  { value: "critical", label: "紧急" },
];

export const appealReviewOptions: SelectOption[] = [
  { value: "approved", label: "通过" },
  { value: "rejected", label: "驳回" },
];

export const safetyResolveOptions: SelectOption[] = [
  { value: "resolved", label: "处置完成" },
];

export function formatFoodSafetyIncidentType(type: string): string {
  const labels: Record<string, string> = {
    "foreign-object": "异物",
    contamination: "污染变质",
    expired: "过期变质",
  };

  return labels[type] ?? type;
}

export function formatMerchantStatus(status: string): string {
  return MERCHANT_STATUS_LABELS[status] ?? status;
}

export function formatRiderStatus(status: string): string {
  return RIDER_STATUS_LABELS[status] ?? status;
}

export function formatAppealStatus(status: string): string {
  return APPEAL_STATUS_LABELS[status] ?? status;
}

export function formatSafetyStatus(status: string): string {
  return SAFETY_STATUS_LABELS[status] ?? status;
}

export function formatSafetyLevel(level: string): string {
  return SAFETY_LEVEL_LABELS[level] ?? level;
}

export function formatWithdrawalStatus(status: string): string {
  return WITHDRAWAL_STATUS_LABELS[status] ?? status;
}

export function formatProfitConfigStatus(status: string): string {
  return PROFIT_CONFIG_STATUS_LABELS[status] ?? status;
}

export function formatRecoveryStatus(status: string): string {
  return RECOVERY_STATUS_LABELS[status] ?? status;
}

export function formatClaimStatus(status: string): string {
  return CLAIM_STATUS_LABELS[status] ?? status;
}

export function formatOrderStatus(status: string): string {
  return ORDER_STATUS_LABELS[status] ?? status;
}

export function formatWeekdays(days: number[]): string {
  const labels = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"];
  return days
    .filter((day) => day >= 0 && day <= 6)
    .map((day) => labels[day])
    .join("、");
}
