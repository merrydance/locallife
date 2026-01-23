
export interface ReviewResponse {
  id: number;
  order_id: number;
  user_id: number;
  merchant_id: number;
  content: string;
  images: string[];
  is_visible: boolean;
  merchant_reply?: string;
  replied_at?: string;
  created_at: string;
}

export interface ReviewListResponse {
  reviews: ReviewResponse[];
  total_count: number;
  page_id: number;
  page_size: number;
}
