export type ClaimRecoveryStatus =
  | "pending"
  | "paid"
  | "overdue"
  | "waived"
  | "appealed";

export type ClaimStatus =
  | "pending"
  | "approved"
  | "auto-approved"
  | "rejected"
  | "manual-review";

export interface ClaimRecoveryResponse {
  id: number;
  claim_id: number;
  order_id: number;
  responsible_party: string;
  recovery_target?: string;
  recovery_amount: number;
  status: ClaimRecoveryStatus;
  due_at: string;
  updated_at: string;
}

export interface MerchantClaimItem {
  id: number;
  order_id: number;
  order_no: string;
  order_amount: number;
  user_phone: string;
  user_name: string;
  claim_type: string;
  claim_amount: number;
  approved_amount?: number;
  description: string;
  status: ClaimStatus;
  created_at: string;
  reviewed_at?: string;
  appeal_id?: number;
  appeal_status?: string;
}

export interface MerchantClaimsResponse {
  claims: MerchantClaimItem[];
  total: number;
  total_count: number;
  page_id: number;
  page_size: number;
}

export interface MerchantClaimDetailResponse {
  id: number;
  order_id: number;
  order_no: string;
  order_amount: number;
  user_phone: string;
  user_name: string;
  claim_type: string;
  claim_amount: number;
  approved_amount?: number;
  description: string;
  status: ClaimStatus;
  created_at: string;
  reviewed_at?: string;
  appeal_id?: number;
  appeal_status?: string;
  appeal_reason?: string;
  appeal_review_notes?: string;
}