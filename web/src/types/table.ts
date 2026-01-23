export type TableType = "table" | "room";
export type TableStatus = "available" | "occupied" | "reserved" | "disabled";

export interface TableTag {
  id: number;
  name: string;
}

export interface TableReservation {
  id: number;
  contact_name: string;
  contact_phone: string;
  guest_count: number;
  reservation_time: string;
  notes?: string;
}

export interface TableResponse {
  id: number;
  merchant_id: number;
  table_no: string;
  table_type: TableType;
  capacity: number;
  description?: string;
  minimum_spend?: number;
  qr_code_url?: string;
  status: TableStatus;
  current_reservation_id?: number;
  current_reservation?: TableReservation;
  created_at: string;
  updated_at?: string;
  tags?: TableTag[];
}

export interface ListTablesResponse {
  tables: TableResponse[];
  count: number;
  total: number;
  total_count: number;
}

export interface CreateTableRequest {
  table_no: string;
  table_type: TableType;
  capacity: number;
  description?: string;
  minimum_spend?: number;
  qr_code_url?: string;
  access_code?: string;
  tag_ids?: number[];
}

export interface UpdateTableRequest {
  table_no?: string;
  table_type?: TableType;
  capacity?: number;
  description?: string;
  minimum_spend?: number;
  qr_code_url?: string;
  access_code?: string;
  status?: TableStatus;
  tag_ids?: number[];
}

export interface TableImageResponse {
  id: number;
  table_id: number;
  image_url: string;
  sort_order: number;
  is_primary: boolean;
}
