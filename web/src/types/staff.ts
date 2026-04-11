export interface StaffResponse {
  id: number;
  merchant_id: number;
  user_id: number;
  role: "owner" | "manager" | "chef" | "cashier" | "pending";
  status: "active" | "disabled" | "pending";
  full_name: string;
  avatar_url?: string;
  created_at: string;
}

export interface ListMerchantStaffResponse {
  staff: StaffResponse[];
  count: number;
}

export interface InviteCodeResponse {
  invite_code: string;
  expires_at: string;
}

export interface UpdateStaffRoleRequest {
  role: "manager" | "chef" | "cashier";
}
