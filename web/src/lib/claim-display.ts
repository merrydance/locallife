const DECISION_REASON_CODE_LABELS: Record<string, string> = {
  instant: "秒赔通过",
  auto: "自动赔付",
  manual: "人工复核",
  "platform-pay": "平台垫付",
  normal: "行为正常",
  warned: "行为预警",
  "reject-service": "限制服务",
  merchant: "商户责任",
  rider: "骑手责任",
  platform: "平台责任",
  platform_fallback: "平台兜底",
  "foreign-object": "异物问题",
  damage: "餐品受损",
  quality: "质量问题",
  timeout: "超时问题",
  delay: "配送延误",
  "missing-item": "缺少商品",
  "food-safety": "食品安全",
  other: "其他原因",
  reservation_no_show: "预订未到店",
};

const DECISION_RESPONSIBLE_PARTY_LABELS: Record<string, string> = {
  merchant: "商户",
  rider: "骑手",
  platform: "平台",
  platform_fallback: "平台兜底",
  unknown: "未知",
};

const DECISION_COMPENSATION_SOURCE_LABELS: Record<string, string> = {
  merchant: "商户承担",
  rider: "骑手承担",
  platform: "平台垫付",
  unknown: "未知",
};

const DECISION_STATUS_LABELS: Record<string, string> = {
  active: "生效中",
  pending: "待处理",
  approved: "已通过",
  rejected: "已驳回",
  waived: "已核销",
  paid: "已支付",
  appealed: "申诉中",
  closed: "已关闭",
  resolved: "已处理",
};

const CLAIM_TYPE_LABELS: Record<string, string> = {
  "foreign-object": "异物问题",
  damage: "餐品受损",
  delay: "配送延误",
  timeout: "超时问题",
  quality: "质量问题",
  "missing-item": "缺少商品",
  "food-safety": "食品安全",
  other: "其他",
};

export function formatDecisionReasonCode(code: string): string {
  if (!code) return "-";
  return DECISION_REASON_CODE_LABELS[code] ?? code;
}

export function formatDecisionResponsibleParty(value: string): string {
  if (!value) return "-";
  return DECISION_RESPONSIBLE_PARTY_LABELS[value] ?? value;
}

export function formatDecisionCompensationSource(value: string): string {
  if (!value) return "-";
  return DECISION_COMPENSATION_SOURCE_LABELS[value] ?? value;
}

export function formatDecisionStatus(value: string): string {
  if (!value) return "-";
  return DECISION_STATUS_LABELS[value] ?? value;
}

export function formatClaimType(value: string): string {
  if (!value) return "-";
  return CLAIM_TYPE_LABELS[value] ?? value;
}
