
export interface MerchantProfile {
  id: number;
  name: string;
  description?: string;
  logo_url?: string;
  phone: string;
  address: string;
  latitude?: string;
  longitude?: string;
  status: string;
  is_open: boolean;
  version: number;
  group_id?: number;
  brand_id?: number;
}

export interface BusinessHour {
  id?: number;
  day_of_week: number;
  day_name: string;
  open_time: string;
  close_time: string;
  is_closed: boolean;
  special_date?: string;
}

export interface CloudPrinter {
  id: number;
  printer_name: string;
  printer_sn: string;
  printer_key: string;
  printer_type: "feieyun" | "yilianyun" | "other";
  print_takeout: boolean;
  print_dine_in: boolean;
  print_reservation: boolean;
  is_active: boolean;
}

export interface DisplayConfig {
  id?: number;
  enable_print: boolean;
  print_takeout: boolean;
  print_dine_in: boolean;
  print_reservation: boolean;
  enable_voice: boolean;
  voice_takeout: boolean;
  voice_dine_in: boolean;
  enable_kds: boolean;
  kds_url?: string;
}

export interface Group {
  id: number;
  name: string;
  address: string;
  contact_phone: string;
  status: string;
}

export interface GroupJoinRequest {
  id: number;
  group_id: number;
  merchant_id: number;
  status: "pending" | "approved" | "rejected" | "cancelled";
  reason?: string;
  created_at: string;
}
