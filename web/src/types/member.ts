export interface MemberResponse {
  user_id: number;
  full_name: string;
  phone: string;
  avatar_url: string;
  membership_id: number;
  balance: number;
  total_recharged: number;
  total_consumed: number;
  created_at: string;
}

export interface ListMerchantMembersResponse {
  members: MemberResponse[];
  total_count: number;
  total: number;
  page_id: number;
  page_size: number;
}

export interface TransactionResponse {
  id: number;
  membership_id: number;
  type: string;
  amount: number;
  balance_after: number;
  related_order_id?: number;
  notes?: string;
  created_at: string;
}

export interface MemberDetailResponse extends MemberResponse {
  transactions: TransactionResponse[];
}

export interface RechargeRuleResponse {
  id: number;
  merchant_id: number;
  recharge_amount: number;
  bonus_amount: number;
  is_active: boolean;
  valid_from: string;
  valid_until: string;
  created_at: string;
}

export interface MembershipSettings {
  merchant_id: number;
  balance_usable_scenes: string[];
  bonus_usable_scenes: string[];
  allow_with_voucher: boolean;
  allow_with_discount: boolean;
  max_deduction_percent: number;
}
