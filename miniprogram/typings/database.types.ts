export type Json =
  | string
  | number
  | boolean
  | null
  | { [key: string]: Json | undefined }
  | Json[]

export type Database = {
  graphql_public: {
    Tables: {
      [_ in never]: never
    }
    Views: {
      [_ in never]: never
    }
    Functions: {
      graphql: {
        Args: {
          extensions?: Json
          operationName?: string
          query?: string
          variables?: Json
        }
        Returns: Json
      }
    }
    Enums: {
      [_ in never]: never
    }
    CompositeTypes: {
      [_ in never]: never
    }
  }
  public: {
    Tables: {
      appeals: {
        Row: {
          appellant_id: string
          appellant_type: string
          claim_id: string
          compensated_at: string | null
          compensation_amount: number | null
          created_at: string
          evidence_urls: string[] | null
          id: string
          reason: string
          region_id: string
          review_notes: string | null
          reviewed_at: string | null
          reviewer_id: string | null
          status: string
        }
        Insert: {
          appellant_id: string
          appellant_type: string
          claim_id: string
          compensated_at?: string | null
          compensation_amount?: number | null
          created_at?: string
          evidence_urls?: string[] | null
          id?: string
          reason: string
          region_id: string
          review_notes?: string | null
          reviewed_at?: string | null
          reviewer_id?: string | null
          status?: string
        }
        Update: {
          appellant_id?: string
          appellant_type?: string
          claim_id?: string
          compensated_at?: string | null
          compensation_amount?: number | null
          created_at?: string
          evidence_urls?: string[] | null
          id?: string
          reason?: string
          region_id?: string
          review_notes?: string | null
          reviewed_at?: string | null
          reviewer_id?: string | null
          status?: string
        }
        Relationships: [
          {
            foreignKeyName: "appeals_claim_id_fkey"
            columns: ["claim_id"]
            isOneToOne: false
            referencedRelation: "claims"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "appeals_region_id_fkey"
            columns: ["region_id"]
            isOneToOne: false
            referencedRelation: "regions"
            referencedColumns: ["id"]
          },
        ]
      }
      browse_history: {
        Row: {
          id: string
          last_viewed_at: string
          target_id: string
          target_type: string
          user_id: string
          view_count: number
        }
        Insert: {
          id?: string
          last_viewed_at?: string
          target_id: string
          target_type: string
          user_id: string
          view_count?: number
        }
        Update: {
          id?: string
          last_viewed_at?: string
          target_id?: string
          target_type?: string
          user_id?: string
          view_count?: number
        }
        Relationships: [
          {
            foreignKeyName: "browse_history_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      cart_items: {
        Row: {
          cart_id: string
          combo_id: string | null
          created_at: string
          customizations: Json | null
          dish_id: string | null
          id: string
          quantity: number
          updated_at: string
        }
        Insert: {
          cart_id: string
          combo_id?: string | null
          created_at?: string
          customizations?: Json | null
          dish_id?: string | null
          id?: string
          quantity?: number
          updated_at?: string
        }
        Update: {
          cart_id?: string
          combo_id?: string | null
          created_at?: string
          customizations?: Json | null
          dish_id?: string | null
          id?: string
          quantity?: number
          updated_at?: string
        }
        Relationships: [
          {
            foreignKeyName: "cart_items_cart_id_fkey"
            columns: ["cart_id"]
            isOneToOne: false
            referencedRelation: "carts"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "cart_items_combo_id_fkey"
            columns: ["combo_id"]
            isOneToOne: false
            referencedRelation: "combo_sets"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "cart_items_dish_id_fkey"
            columns: ["dish_id"]
            isOneToOne: false
            referencedRelation: "dishes"
            referencedColumns: ["id"]
          },
        ]
      }
      carts: {
        Row: {
          created_at: string
          id: string
          merchant_id: string
          order_type: string
          reservation_id: string | null
          table_id: string | null
          updated_at: string
          user_id: string
        }
        Insert: {
          created_at?: string
          id?: string
          merchant_id: string
          order_type?: string
          reservation_id?: string | null
          table_id?: string | null
          updated_at?: string
          user_id: string
        }
        Update: {
          created_at?: string
          id?: string
          merchant_id?: string
          order_type?: string
          reservation_id?: string | null
          table_id?: string | null
          updated_at?: string
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "carts_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "carts_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      claims: {
        Row: {
          approval_type: string | null
          approved_amount: number | null
          auto_approval_reason: string | null
          claim_amount: number
          claim_type: string
          created_at: string
          description: string
          evidence_urls: string[] | null
          id: string
          is_malicious: boolean
          lookback_result: Json | null
          order_id: string
          paid_at: string | null
          rejection_reason: string | null
          review_notes: string | null
          reviewed_at: string | null
          reviewer_id: string | null
          status: string
          trust_score_snapshot: number | null
          user_id: string
        }
        Insert: {
          approval_type?: string | null
          approved_amount?: number | null
          auto_approval_reason?: string | null
          claim_amount: number
          claim_type: string
          created_at?: string
          description: string
          evidence_urls?: string[] | null
          id?: string
          is_malicious?: boolean
          lookback_result?: Json | null
          order_id: string
          paid_at?: string | null
          rejection_reason?: string | null
          review_notes?: string | null
          reviewed_at?: string | null
          reviewer_id?: string | null
          status?: string
          trust_score_snapshot?: number | null
          user_id: string
        }
        Update: {
          approval_type?: string | null
          approved_amount?: number | null
          auto_approval_reason?: string | null
          claim_amount?: number
          claim_type?: string
          created_at?: string
          description?: string
          evidence_urls?: string[] | null
          id?: string
          is_malicious?: boolean
          lookback_result?: Json | null
          order_id?: string
          paid_at?: string | null
          rejection_reason?: string | null
          review_notes?: string | null
          reviewed_at?: string | null
          reviewer_id?: string | null
          status?: string
          trust_score_snapshot?: number | null
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "claims_order_id_fkey"
            columns: ["order_id"]
            isOneToOne: false
            referencedRelation: "orders"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "claims_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      cloud_printers: {
        Row: {
          created_at: string
          id: string
          is_active: boolean
          merchant_id: string
          print_dine_in: boolean
          print_reservation: boolean
          print_takeout: boolean
          printer_key: string
          printer_name: string
          printer_sn: string
          printer_type: string
          updated_at: string | null
        }
        Insert: {
          created_at?: string
          id?: string
          is_active?: boolean
          merchant_id: string
          print_dine_in?: boolean
          print_reservation?: boolean
          print_takeout?: boolean
          printer_key: string
          printer_name: string
          printer_sn: string
          printer_type: string
          updated_at?: string | null
        }
        Update: {
          created_at?: string
          id?: string
          is_active?: boolean
          merchant_id?: string
          print_dine_in?: boolean
          print_reservation?: boolean
          print_takeout?: boolean
          printer_key?: string
          printer_name?: string
          printer_sn?: string
          printer_type?: string
          updated_at?: string | null
        }
        Relationships: [
          {
            foreignKeyName: "cloud_printers_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
        ]
      }
      combined_payment_orders: {
        Row: {
          combine_out_trade_no: string
          created_at: string
          expires_at: string | null
          id: string
          paid_at: string | null
          prepay_id: string | null
          status: string
          total_amount: number
          transaction_id: string | null
          user_id: string
        }
        Insert: {
          combine_out_trade_no: string
          created_at?: string
          expires_at?: string | null
          id?: string
          paid_at?: string | null
          prepay_id?: string | null
          status?: string
          total_amount: number
          transaction_id?: string | null
          user_id: string
        }
        Update: {
          combine_out_trade_no?: string
          created_at?: string
          expires_at?: string | null
          id?: string
          paid_at?: string | null
          prepay_id?: string | null
          status?: string
          total_amount?: number
          transaction_id?: string | null
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "combined_payment_orders_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      combined_payment_sub_orders: {
        Row: {
          amount: number
          combined_payment_id: string
          created_at: string
          description: string
          id: string
          merchant_id: string
          order_id: string
          out_trade_no: string
          profit_sharing_status: string
          sub_mchid: string
        }
        Insert: {
          amount: number
          combined_payment_id: string
          created_at?: string
          description: string
          id?: string
          merchant_id: string
          order_id: string
          out_trade_no: string
          profit_sharing_status?: string
          sub_mchid: string
        }
        Update: {
          amount?: number
          combined_payment_id?: string
          created_at?: string
          description?: string
          id?: string
          merchant_id?: string
          order_id?: string
          out_trade_no?: string
          profit_sharing_status?: string
          sub_mchid?: string
        }
        Relationships: [
          {
            foreignKeyName: "combined_payment_sub_orders_combined_payment_id_fkey"
            columns: ["combined_payment_id"]
            isOneToOne: false
            referencedRelation: "combined_payment_orders"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "combined_payment_sub_orders_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "combined_payment_sub_orders_order_id_fkey"
            columns: ["order_id"]
            isOneToOne: false
            referencedRelation: "orders"
            referencedColumns: ["id"]
          },
        ]
      }
      combo_dishes: {
        Row: {
          combo_id: string
          dish_id: string
          id: string
          quantity: number
        }
        Insert: {
          combo_id: string
          dish_id: string
          id?: string
          quantity?: number
        }
        Update: {
          combo_id?: string
          dish_id?: string
          id?: string
          quantity?: number
        }
        Relationships: [
          {
            foreignKeyName: "combo_dishes_combo_id_fkey"
            columns: ["combo_id"]
            isOneToOne: false
            referencedRelation: "combo_sets"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "combo_dishes_dish_id_fkey"
            columns: ["dish_id"]
            isOneToOne: false
            referencedRelation: "dishes"
            referencedColumns: ["id"]
          },
        ]
      }
      combo_sets: {
        Row: {
          combo_price: number
          created_at: string
          deleted_at: string | null
          description: string | null
          id: string
          image_url: string | null
          is_online: boolean
          merchant_id: string
          name: string
          original_price: number
          updated_at: string | null
        }
        Insert: {
          combo_price: number
          created_at?: string
          deleted_at?: string | null
          description?: string | null
          id?: string
          image_url?: string | null
          is_online?: boolean
          merchant_id: string
          name: string
          original_price: number
          updated_at?: string | null
        }
        Update: {
          combo_price?: number
          created_at?: string
          deleted_at?: string | null
          description?: string | null
          id?: string
          image_url?: string | null
          is_online?: boolean
          merchant_id?: string
          name?: string
          original_price?: number
          updated_at?: string | null
        }
        Relationships: [
          {
            foreignKeyName: "combo_sets_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
        ]
      }
      combo_tags: {
        Row: {
          combo_id: string
          created_at: string
          id: string
          tag_id: string
        }
        Insert: {
          combo_id: string
          created_at?: string
          id?: string
          tag_id: string
        }
        Update: {
          combo_id?: string
          created_at?: string
          id?: string
          tag_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "combo_tags_combo_id_fkey"
            columns: ["combo_id"]
            isOneToOne: false
            referencedRelation: "combo_sets"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "combo_tags_tag_id_fkey"
            columns: ["tag_id"]
            isOneToOne: false
            referencedRelation: "tags"
            referencedColumns: ["id"]
          },
        ]
      }
      daily_inventory: {
        Row: {
          created_at: string
          date: string
          dish_id: string
          id: string
          merchant_id: string
          sold_quantity: number
          total_quantity: number
          updated_at: string | null
        }
        Insert: {
          created_at?: string
          date: string
          dish_id: string
          id?: string
          merchant_id: string
          sold_quantity?: number
          total_quantity?: number
          updated_at?: string | null
        }
        Update: {
          created_at?: string
          date?: string
          dish_id?: string
          id?: string
          merchant_id?: string
          sold_quantity?: number
          total_quantity?: number
          updated_at?: string | null
        }
        Relationships: [
          {
            foreignKeyName: "daily_inventory_dish_id_fkey"
            columns: ["dish_id"]
            isOneToOne: false
            referencedRelation: "dishes"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "daily_inventory_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
        ]
      }
      deliveries: {
        Row: {
          assigned_at: string | null
          completed_at: string | null
          created_at: string
          damage_amount: number
          damage_reason: string | null
          delivered_at: string | null
          delivery_address: string
          delivery_contact: string | null
          delivery_fee: number
          delivery_latitude: number
          delivery_longitude: number
          delivery_phone: string | null
          distance: number
          estimated_delivery_at: string | null
          estimated_pickup_at: string | null
          id: string
          is_damaged: boolean
          is_delayed: boolean
          order_id: string
          picked_at: string | null
          pickup_address: string
          pickup_contact: string | null
          pickup_latitude: number
          pickup_longitude: number
          pickup_phone: string | null
          rider_earnings: number
          rider_id: string | null
          status: string
        }
        Insert: {
          assigned_at?: string | null
          completed_at?: string | null
          created_at?: string
          damage_amount?: number
          damage_reason?: string | null
          delivered_at?: string | null
          delivery_address: string
          delivery_contact?: string | null
          delivery_fee: number
          delivery_latitude: number
          delivery_longitude: number
          delivery_phone?: string | null
          distance: number
          estimated_delivery_at?: string | null
          estimated_pickup_at?: string | null
          id?: string
          is_damaged?: boolean
          is_delayed?: boolean
          order_id: string
          picked_at?: string | null
          pickup_address: string
          pickup_contact?: string | null
          pickup_latitude: number
          pickup_longitude: number
          pickup_phone?: string | null
          rider_earnings?: number
          rider_id?: string | null
          status?: string
        }
        Update: {
          assigned_at?: string | null
          completed_at?: string | null
          created_at?: string
          damage_amount?: number
          damage_reason?: string | null
          delivered_at?: string | null
          delivery_address?: string
          delivery_contact?: string | null
          delivery_fee?: number
          delivery_latitude?: number
          delivery_longitude?: number
          delivery_phone?: string | null
          distance?: number
          estimated_delivery_at?: string | null
          estimated_pickup_at?: string | null
          id?: string
          is_damaged?: boolean
          is_delayed?: boolean
          order_id?: string
          picked_at?: string | null
          pickup_address?: string
          pickup_contact?: string | null
          pickup_latitude?: number
          pickup_longitude?: number
          pickup_phone?: string | null
          rider_earnings?: number
          rider_id?: string | null
          status?: string
        }
        Relationships: [
          {
            foreignKeyName: "deliveries_order_id_fkey"
            columns: ["order_id"]
            isOneToOne: false
            referencedRelation: "orders"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "deliveries_rider_id_fkey"
            columns: ["rider_id"]
            isOneToOne: false
            referencedRelation: "riders"
            referencedColumns: ["id"]
          },
        ]
      }
      delivery_fee_configs: {
        Row: {
          base_distance: number
          base_fee: number
          created_at: string
          extra_fee_per_km: number
          id: string
          is_active: boolean
          max_fee: number | null
          min_fee: number
          region_id: string
          updated_at: string | null
          value_ratio: number
        }
        Insert: {
          base_distance: number
          base_fee: number
          created_at?: string
          extra_fee_per_km: number
          id?: string
          is_active?: boolean
          max_fee?: number | null
          min_fee?: number
          region_id: string
          updated_at?: string | null
          value_ratio?: number
        }
        Update: {
          base_distance?: number
          base_fee?: number
          created_at?: string
          extra_fee_per_km?: number
          id?: string
          is_active?: boolean
          max_fee?: number | null
          min_fee?: number
          region_id?: string
          updated_at?: string | null
          value_ratio?: number
        }
        Relationships: [
          {
            foreignKeyName: "delivery_fee_configs_region_id_fkey"
            columns: ["region_id"]
            isOneToOne: false
            referencedRelation: "regions"
            referencedColumns: ["id"]
          },
        ]
      }
      delivery_pool: {
        Row: {
          created_at: string
          delivery_fee: number
          delivery_latitude: number
          delivery_longitude: number
          distance: number
          expected_pickup_at: string
          expires_at: string
          id: string
          merchant_id: string
          order_id: string
          pickup_latitude: number
          pickup_longitude: number
          priority: number
        }
        Insert: {
          created_at?: string
          delivery_fee: number
          delivery_latitude: number
          delivery_longitude: number
          distance: number
          expected_pickup_at: string
          expires_at: string
          id?: string
          merchant_id: string
          order_id: string
          pickup_latitude: number
          pickup_longitude: number
          priority?: number
        }
        Update: {
          created_at?: string
          delivery_fee?: number
          delivery_latitude?: number
          delivery_longitude?: number
          distance?: number
          expected_pickup_at?: string
          expires_at?: string
          id?: string
          merchant_id?: string
          order_id?: string
          pickup_latitude?: number
          pickup_longitude?: number
          priority?: number
        }
        Relationships: [
          {
            foreignKeyName: "delivery_pool_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "delivery_pool_order_id_fkey"
            columns: ["order_id"]
            isOneToOne: false
            referencedRelation: "orders"
            referencedColumns: ["id"]
          },
        ]
      }
      discount_rules: {
        Row: {
          can_stack_with_membership: boolean
          can_stack_with_voucher: boolean
          created_at: string
          deleted_at: string | null
          description: string | null
          discount_amount: number
          id: string
          is_active: boolean
          merchant_id: string
          min_order_amount: number
          name: string
          updated_at: string | null
          valid_from: string
          valid_until: string
        }
        Insert: {
          can_stack_with_membership?: boolean
          can_stack_with_voucher?: boolean
          created_at?: string
          deleted_at?: string | null
          description?: string | null
          discount_amount: number
          id?: string
          is_active?: boolean
          merchant_id: string
          min_order_amount: number
          name: string
          updated_at?: string | null
          valid_from: string
          valid_until: string
        }
        Update: {
          can_stack_with_membership?: boolean
          can_stack_with_voucher?: boolean
          created_at?: string
          deleted_at?: string | null
          description?: string | null
          discount_amount?: number
          id?: string
          is_active?: boolean
          merchant_id?: string
          min_order_amount?: number
          name?: string
          updated_at?: string | null
          valid_from?: string
          valid_until?: string
        }
        Relationships: [
          {
            foreignKeyName: "discount_rules_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
        ]
      }
      dish_categories: {
        Row: {
          created_at: string
          deleted_at: string | null
          id: string
          name: string
        }
        Insert: {
          created_at?: string
          deleted_at?: string | null
          id?: string
          name: string
        }
        Update: {
          created_at?: string
          deleted_at?: string | null
          id?: string
          name?: string
        }
        Relationships: []
      }
      dish_customization_groups: {
        Row: {
          created_at: string
          dish_id: string
          id: string
          is_required: boolean
          name: string
          sort_order: number
        }
        Insert: {
          created_at?: string
          dish_id: string
          id?: string
          is_required?: boolean
          name: string
          sort_order?: number
        }
        Update: {
          created_at?: string
          dish_id?: string
          id?: string
          is_required?: boolean
          name?: string
          sort_order?: number
        }
        Relationships: [
          {
            foreignKeyName: "dish_customization_groups_dish_id_fkey"
            columns: ["dish_id"]
            isOneToOne: false
            referencedRelation: "dishes"
            referencedColumns: ["id"]
          },
        ]
      }
      dish_customization_options: {
        Row: {
          extra_price: number
          group_id: string
          id: string
          sort_order: number
          tag_id: string
        }
        Insert: {
          extra_price?: number
          group_id: string
          id?: string
          sort_order?: number
          tag_id: string
        }
        Update: {
          extra_price?: number
          group_id?: string
          id?: string
          sort_order?: number
          tag_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "dish_customization_options_group_id_fkey"
            columns: ["group_id"]
            isOneToOne: false
            referencedRelation: "dish_customization_groups"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "dish_customization_options_tag_id_fkey"
            columns: ["tag_id"]
            isOneToOne: false
            referencedRelation: "tags"
            referencedColumns: ["id"]
          },
        ]
      }
      dish_ingredients: {
        Row: {
          created_at: string
          dish_id: string
          id: string
          ingredient_id: string
        }
        Insert: {
          created_at?: string
          dish_id: string
          id?: string
          ingredient_id: string
        }
        Update: {
          created_at?: string
          dish_id?: string
          id?: string
          ingredient_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "dish_ingredients_dish_id_fkey"
            columns: ["dish_id"]
            isOneToOne: false
            referencedRelation: "dishes"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "dish_ingredients_ingredient_id_fkey"
            columns: ["ingredient_id"]
            isOneToOne: false
            referencedRelation: "ingredients"
            referencedColumns: ["id"]
          },
        ]
      }
      dish_tags: {
        Row: {
          created_at: string
          dish_id: string
          id: string
          tag_id: string
        }
        Insert: {
          created_at?: string
          dish_id: string
          id?: string
          tag_id: string
        }
        Update: {
          created_at?: string
          dish_id?: string
          id?: string
          tag_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "dish_tags_dish_id_fkey"
            columns: ["dish_id"]
            isOneToOne: false
            referencedRelation: "dishes"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "dish_tags_tag_id_fkey"
            columns: ["tag_id"]
            isOneToOne: false
            referencedRelation: "tags"
            referencedColumns: ["id"]
          },
        ]
      }
      dishes: {
        Row: {
          category_id: string | null
          created_at: string
          deleted_at: string | null
          description: string | null
          id: string
          image_url: string | null
          is_available: boolean
          is_online: boolean
          member_price: number | null
          merchant_id: string
          monthly_sales: number
          name: string
          prepare_time: number
          price: number
          repurchase_rate: number
          sort_order: number
          updated_at: string | null
        }
        Insert: {
          category_id?: string | null
          created_at?: string
          deleted_at?: string | null
          description?: string | null
          id?: string
          image_url?: string | null
          is_available?: boolean
          is_online?: boolean
          member_price?: number | null
          merchant_id: string
          monthly_sales?: number
          name: string
          prepare_time?: number
          price: number
          repurchase_rate?: number
          sort_order?: number
          updated_at?: string | null
        }
        Update: {
          category_id?: string | null
          created_at?: string
          deleted_at?: string | null
          description?: string | null
          id?: string
          image_url?: string | null
          is_available?: boolean
          is_online?: boolean
          member_price?: number | null
          merchant_id?: string
          monthly_sales?: number
          name?: string
          prepare_time?: number
          price?: number
          repurchase_rate?: number
          sort_order?: number
          updated_at?: string | null
        }
        Relationships: [
          {
            foreignKeyName: "dishes_category_id_fkey"
            columns: ["category_id"]
            isOneToOne: false
            referencedRelation: "dish_categories"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "dishes_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
        ]
      }
      ecommerce_applyments: {
        Row: {
          account_bank: string
          account_name: string
          account_number: string
          account_type: string
          applyment_id: string | null
          audited_at: string | null
          bank_address_code: string
          bank_name: string | null
          business_addition_desc: string | null
          business_addition_pics: string[] | null
          business_license_copy: string | null
          business_license_number: string | null
          contact_email: string | null
          contact_id_card_number: string | null
          contact_name: string
          created_at: string
          id: string
          id_card_back_copy: string
          id_card_front_copy: string
          id_card_name: string
          id_card_number: string
          id_card_valid_time: string
          legal_person: string
          merchant_name: string
          merchant_shortname: string
          mobile_phone: string
          organization_type: string
          out_request_no: string
          qualifications: Json | null
          reject_reason: string | null
          sign_state: string | null
          sign_url: string | null
          status: string
          sub_mch_id: string | null
          subject_id: string
          subject_type: string
          submitted_at: string | null
          updated_at: string
        }
        Insert: {
          account_bank: string
          account_name: string
          account_number: string
          account_type: string
          applyment_id?: string | null
          audited_at?: string | null
          bank_address_code: string
          bank_name?: string | null
          business_addition_desc?: string | null
          business_addition_pics?: string[] | null
          business_license_copy?: string | null
          business_license_number?: string | null
          contact_email?: string | null
          contact_id_card_number?: string | null
          contact_name: string
          created_at?: string
          id?: string
          id_card_back_copy: string
          id_card_front_copy: string
          id_card_name: string
          id_card_number: string
          id_card_valid_time: string
          legal_person: string
          merchant_name: string
          merchant_shortname: string
          mobile_phone: string
          organization_type: string
          out_request_no: string
          qualifications?: Json | null
          reject_reason?: string | null
          sign_state?: string | null
          sign_url?: string | null
          status?: string
          sub_mch_id?: string | null
          subject_id: string
          subject_type: string
          submitted_at?: string | null
          updated_at?: string
        }
        Update: {
          account_bank?: string
          account_name?: string
          account_number?: string
          account_type?: string
          applyment_id?: string | null
          audited_at?: string | null
          bank_address_code?: string
          bank_name?: string | null
          business_addition_desc?: string | null
          business_addition_pics?: string[] | null
          business_license_copy?: string | null
          business_license_number?: string | null
          contact_email?: string | null
          contact_id_card_number?: string | null
          contact_name?: string
          created_at?: string
          id?: string
          id_card_back_copy?: string
          id_card_front_copy?: string
          id_card_name?: string
          id_card_number?: string
          id_card_valid_time?: string
          legal_person?: string
          merchant_name?: string
          merchant_shortname?: string
          mobile_phone?: string
          organization_type?: string
          out_request_no?: string
          qualifications?: Json | null
          reject_reason?: string | null
          sign_state?: string | null
          sign_url?: string | null
          status?: string
          sub_mch_id?: string | null
          subject_id?: string
          subject_type?: string
          submitted_at?: string | null
          updated_at?: string
        }
        Relationships: []
      }
      favorites: {
        Row: {
          created_at: string
          dish_id: string | null
          favorite_type: string
          id: string
          merchant_id: string | null
          user_id: string
        }
        Insert: {
          created_at?: string
          dish_id?: string | null
          favorite_type: string
          id?: string
          merchant_id?: string | null
          user_id: string
        }
        Update: {
          created_at?: string
          dish_id?: string | null
          favorite_type?: string
          id?: string
          merchant_id?: string | null
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "favorites_dish_id_fkey"
            columns: ["dish_id"]
            isOneToOne: false
            referencedRelation: "dishes"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "favorites_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "favorites_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      food_safety_incidents: {
        Row: {
          created_at: string
          description: string
          evidence_urls: string[]
          id: string
          incident_type: string
          investigation_report: string | null
          merchant_id: string
          merchant_snapshot: Json
          order_id: string
          order_snapshot: Json
          resolution: string | null
          resolved_at: string | null
          rider_snapshot: Json | null
          status: string
          user_id: string
        }
        Insert: {
          created_at?: string
          description: string
          evidence_urls: string[]
          id?: string
          incident_type: string
          investigation_report?: string | null
          merchant_id: string
          merchant_snapshot: Json
          order_id: string
          order_snapshot: Json
          resolution?: string | null
          resolved_at?: string | null
          rider_snapshot?: Json | null
          status?: string
          user_id: string
        }
        Update: {
          created_at?: string
          description?: string
          evidence_urls?: string[]
          id?: string
          incident_type?: string
          investigation_report?: string | null
          merchant_id?: string
          merchant_snapshot?: Json
          order_id?: string
          order_snapshot?: Json
          resolution?: string | null
          resolved_at?: string | null
          rider_snapshot?: Json | null
          status?: string
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "food_safety_incidents_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "food_safety_incidents_order_id_fkey"
            columns: ["order_id"]
            isOneToOne: false
            referencedRelation: "orders"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "food_safety_incidents_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      fraud_patterns: {
        Row: {
          action_taken: string | null
          address_ids: number[] | null
          confirmed_at: string | null
          detected_at: string
          device_fingerprints: string[] | null
          id: string
          ip_addresses: string[] | null
          is_confirmed: boolean
          match_count: number
          pattern_description: string | null
          pattern_type: string
          related_claim_ids: number[] | null
          related_order_ids: number[] | null
          related_user_ids: number[]
          review_notes: string | null
          reviewed_at: string | null
          reviewer_id: string | null
        }
        Insert: {
          action_taken?: string | null
          address_ids?: number[] | null
          confirmed_at?: string | null
          detected_at?: string
          device_fingerprints?: string[] | null
          id?: string
          ip_addresses?: string[] | null
          is_confirmed?: boolean
          match_count?: number
          pattern_description?: string | null
          pattern_type: string
          related_claim_ids?: number[] | null
          related_order_ids?: number[] | null
          related_user_ids: number[]
          review_notes?: string | null
          reviewed_at?: string | null
          reviewer_id?: string | null
        }
        Update: {
          action_taken?: string | null
          address_ids?: number[] | null
          confirmed_at?: string | null
          detected_at?: string
          device_fingerprints?: string[] | null
          id?: string
          ip_addresses?: string[] | null
          is_confirmed?: boolean
          match_count?: number
          pattern_description?: string | null
          pattern_type?: string
          related_claim_ids?: number[] | null
          related_order_ids?: number[] | null
          related_user_ids?: number[]
          review_notes?: string | null
          reviewed_at?: string | null
          reviewer_id?: string | null
        }
        Relationships: []
      }
      ingredients: {
        Row: {
          category: string | null
          created_at: string
          created_by: string | null
          id: string
          is_allergen: boolean
          is_system: boolean
          name: string
        }
        Insert: {
          category?: string | null
          created_at?: string
          created_by?: string | null
          id?: string
          is_allergen?: boolean
          is_system?: boolean
          name: string
        }
        Update: {
          category?: string | null
          created_at?: string
          created_by?: string | null
          id?: string
          is_allergen?: boolean
          is_system?: boolean
          name?: string
        }
        Relationships: [
          {
            foreignKeyName: "ingredients_created_by_fkey"
            columns: ["created_by"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      membership_transactions: {
        Row: {
          amount: number
          balance_after: number
          created_at: string
          id: string
          membership_id: string
          notes: string | null
          recharge_rule_id: string | null
          related_order_id: string | null
          type: string
        }
        Insert: {
          amount: number
          balance_after: number
          created_at?: string
          id?: string
          membership_id: string
          notes?: string | null
          recharge_rule_id?: string | null
          related_order_id?: string | null
          type: string
        }
        Update: {
          amount?: number
          balance_after?: number
          created_at?: string
          id?: string
          membership_id?: string
          notes?: string | null
          recharge_rule_id?: string | null
          related_order_id?: string | null
          type?: string
        }
        Relationships: [
          {
            foreignKeyName: "membership_transactions_membership_id_fkey"
            columns: ["membership_id"]
            isOneToOne: false
            referencedRelation: "merchant_memberships"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "membership_transactions_recharge_rule_id_fkey"
            columns: ["recharge_rule_id"]
            isOneToOne: false
            referencedRelation: "recharge_rules"
            referencedColumns: ["id"]
          },
        ]
      }
      merchant_applications: {
        Row: {
          business_address: string
          business_license_image_url: string
          business_license_number: string
          business_license_ocr: Json | null
          business_scope: string | null
          contact_phone: string
          created_at: string
          environment_images: Json | null
          food_permit_ocr: Json | null
          food_permit_url: string | null
          id: string
          id_card_back_ocr: Json | null
          id_card_front_ocr: Json | null
          latitude: number | null
          legal_person_id_back_url: string
          legal_person_id_front_url: string
          legal_person_id_number: string
          legal_person_name: string
          longitude: number | null
          merchant_name: string
          region_id: string | null
          reject_reason: string | null
          reviewed_at: string | null
          reviewed_by: string | null
          status: string
          storefront_images: Json | null
          updated_at: string
          user_id: string
        }
        Insert: {
          business_address: string
          business_license_image_url: string
          business_license_number: string
          business_license_ocr?: Json | null
          business_scope?: string | null
          contact_phone: string
          created_at?: string
          environment_images?: Json | null
          food_permit_ocr?: Json | null
          food_permit_url?: string | null
          id?: string
          id_card_back_ocr?: Json | null
          id_card_front_ocr?: Json | null
          latitude?: number | null
          legal_person_id_back_url: string
          legal_person_id_front_url: string
          legal_person_id_number: string
          legal_person_name: string
          longitude?: number | null
          merchant_name: string
          region_id?: string | null
          reject_reason?: string | null
          reviewed_at?: string | null
          reviewed_by?: string | null
          status?: string
          storefront_images?: Json | null
          updated_at?: string
          user_id: string
        }
        Update: {
          business_address?: string
          business_license_image_url?: string
          business_license_number?: string
          business_license_ocr?: Json | null
          business_scope?: string | null
          contact_phone?: string
          created_at?: string
          environment_images?: Json | null
          food_permit_ocr?: Json | null
          food_permit_url?: string | null
          id?: string
          id_card_back_ocr?: Json | null
          id_card_front_ocr?: Json | null
          latitude?: number | null
          legal_person_id_back_url?: string
          legal_person_id_front_url?: string
          legal_person_id_number?: string
          legal_person_name?: string
          longitude?: number | null
          merchant_name?: string
          region_id?: string | null
          reject_reason?: string | null
          reviewed_at?: string | null
          reviewed_by?: string | null
          status?: string
          storefront_images?: Json | null
          updated_at?: string
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "fk_merchant_applications_region"
            columns: ["region_id"]
            isOneToOne: false
            referencedRelation: "regions"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "merchant_applications_reviewed_by_fkey"
            columns: ["reviewed_by"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "merchant_applications_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      merchant_bosses: {
        Row: {
          created_at: string
          id: string
          merchant_id: string
          status: string
          updated_at: string | null
          user_id: string
        }
        Insert: {
          created_at?: string
          id?: string
          merchant_id: string
          status?: string
          updated_at?: string | null
          user_id: string
        }
        Update: {
          created_at?: string
          id?: string
          merchant_id?: string
          status?: string
          updated_at?: string | null
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "merchant_bosses_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "merchant_bosses_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      merchant_business_hours: {
        Row: {
          close_time: string
          created_at: string
          day_of_week: number
          id: string
          is_closed: boolean
          merchant_id: string
          open_time: string
          special_date: string | null
          updated_at: string
        }
        Insert: {
          close_time: string
          created_at?: string
          day_of_week: number
          id?: string
          is_closed?: boolean
          merchant_id: string
          open_time: string
          special_date?: string | null
          updated_at?: string
        }
        Update: {
          close_time?: string
          created_at?: string
          day_of_week?: number
          id?: string
          is_closed?: boolean
          merchant_id?: string
          open_time?: string
          special_date?: string | null
          updated_at?: string
        }
        Relationships: [
          {
            foreignKeyName: "merchant_business_hours_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
        ]
      }
      merchant_delivery_promotions: {
        Row: {
          created_at: string
          discount_amount: number
          id: string
          is_active: boolean
          merchant_id: string
          min_order_amount: number
          name: string
          updated_at: string | null
          valid_from: string
          valid_until: string
        }
        Insert: {
          created_at?: string
          discount_amount: number
          id?: string
          is_active?: boolean
          merchant_id: string
          min_order_amount: number
          name: string
          updated_at?: string | null
          valid_from: string
          valid_until: string
        }
        Update: {
          created_at?: string
          discount_amount?: number
          id?: string
          is_active?: boolean
          merchant_id?: string
          min_order_amount?: number
          name?: string
          updated_at?: string | null
          valid_from?: string
          valid_until?: string
        }
        Relationships: [
          {
            foreignKeyName: "merchant_delivery_promotions_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
        ]
      }
      merchant_dish_categories: {
        Row: {
          category_id: string
          created_at: string
          merchant_id: string
          sort_order: number
        }
        Insert: {
          category_id: string
          created_at?: string
          merchant_id: string
          sort_order?: number
        }
        Update: {
          category_id?: string
          created_at?: string
          merchant_id?: string
          sort_order?: number
        }
        Relationships: [
          {
            foreignKeyName: "merchant_dish_categories_category_id_fkey"
            columns: ["category_id"]
            isOneToOne: false
            referencedRelation: "dish_categories"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "merchant_dish_categories_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
        ]
      }
      merchant_membership_settings: {
        Row: {
          allow_with_discount: boolean
          allow_with_voucher: boolean
          balance_usable_scenes: string[]
          bonus_usable_scenes: string[]
          created_at: string
          id: string
          max_deduction_percent: number
          merchant_id: string
          updated_at: string | null
        }
        Insert: {
          allow_with_discount?: boolean
          allow_with_voucher?: boolean
          balance_usable_scenes?: string[]
          bonus_usable_scenes?: string[]
          created_at?: string
          id?: string
          max_deduction_percent?: number
          merchant_id: string
          updated_at?: string | null
        }
        Update: {
          allow_with_discount?: boolean
          allow_with_voucher?: boolean
          balance_usable_scenes?: string[]
          bonus_usable_scenes?: string[]
          created_at?: string
          id?: string
          max_deduction_percent?: number
          merchant_id?: string
          updated_at?: string | null
        }
        Relationships: [
          {
            foreignKeyName: "merchant_membership_settings_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: true
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
        ]
      }
      merchant_memberships: {
        Row: {
          balance: number
          created_at: string
          id: string
          merchant_id: string
          total_consumed: number
          total_recharged: number
          updated_at: string | null
          user_id: string
        }
        Insert: {
          balance?: number
          created_at?: string
          id?: string
          merchant_id: string
          total_consumed?: number
          total_recharged?: number
          updated_at?: string | null
          user_id: string
        }
        Update: {
          balance?: number
          created_at?: string
          id?: string
          merchant_id?: string
          total_consumed?: number
          total_recharged?: number
          updated_at?: string | null
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "merchant_memberships_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "merchant_memberships_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      merchant_payment_configs: {
        Row: {
          created_at: string
          id: string
          merchant_id: string
          status: string
          sub_mch_id: string
          updated_at: string
        }
        Insert: {
          created_at?: string
          id?: string
          merchant_id: string
          status?: string
          sub_mch_id: string
          updated_at?: string
        }
        Update: {
          created_at?: string
          id?: string
          merchant_id?: string
          status?: string
          sub_mch_id?: string
          updated_at?: string
        }
        Relationships: [
          {
            foreignKeyName: "merchant_payment_configs_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: true
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
        ]
      }
      merchant_profiles: {
        Row: {
          completed_orders: number
          food_safety_incidents: number
          foreign_object_claims: number
          id: string
          is_suspended: boolean
          merchant_id: string
          recent_30d_claims: number
          recent_30d_incidents: number
          recent_30d_timeouts: number
          recent_7d_claims: number
          recent_7d_incidents: number
          recent_90d_claims: number
          recent_90d_incidents: number
          refuse_order_count: number
          suspend_reason: string | null
          suspend_until: string | null
          suspended_at: string | null
          timeout_count: number
          total_claims: number
          total_orders: number
          total_sales: number
          trust_score: number
          updated_at: string
        }
        Insert: {
          completed_orders?: number
          food_safety_incidents?: number
          foreign_object_claims?: number
          id?: string
          is_suspended?: boolean
          merchant_id: string
          recent_30d_claims?: number
          recent_30d_incidents?: number
          recent_30d_timeouts?: number
          recent_7d_claims?: number
          recent_7d_incidents?: number
          recent_90d_claims?: number
          recent_90d_incidents?: number
          refuse_order_count?: number
          suspend_reason?: string | null
          suspend_until?: string | null
          suspended_at?: string | null
          timeout_count?: number
          total_claims?: number
          total_orders?: number
          total_sales?: number
          trust_score?: number
          updated_at?: string
        }
        Update: {
          completed_orders?: number
          food_safety_incidents?: number
          foreign_object_claims?: number
          id?: string
          is_suspended?: boolean
          merchant_id?: string
          recent_30d_claims?: number
          recent_30d_incidents?: number
          recent_30d_timeouts?: number
          recent_7d_claims?: number
          recent_7d_incidents?: number
          recent_90d_claims?: number
          recent_90d_incidents?: number
          refuse_order_count?: number
          suspend_reason?: string | null
          suspend_until?: string | null
          suspended_at?: string | null
          timeout_count?: number
          total_claims?: number
          total_orders?: number
          total_sales?: number
          trust_score?: number
          updated_at?: string
        }
        Relationships: [
          {
            foreignKeyName: "merchant_profiles_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: true
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
        ]
      }
      merchant_staff: {
        Row: {
          created_at: string
          id: string
          invited_by: string | null
          merchant_id: string
          role: string
          status: string
          updated_at: string | null
          user_id: string
        }
        Insert: {
          created_at?: string
          id?: string
          invited_by?: string | null
          merchant_id: string
          role: string
          status?: string
          updated_at?: string | null
          user_id: string
        }
        Update: {
          created_at?: string
          id?: string
          invited_by?: string | null
          merchant_id?: string
          role?: string
          status?: string
          updated_at?: string | null
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "merchant_staff_invited_by_fkey"
            columns: ["invited_by"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "merchant_staff_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "merchant_staff_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      merchant_tags: {
        Row: {
          created_at: string
          merchant_id: string
          tag_id: string
        }
        Insert: {
          created_at?: string
          merchant_id: string
          tag_id: string
        }
        Update: {
          created_at?: string
          merchant_id?: string
          tag_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "merchant_tags_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "merchant_tags_tag_id_fkey"
            columns: ["tag_id"]
            isOneToOne: false
            referencedRelation: "tags"
            referencedColumns: ["id"]
          },
        ]
      }
      merchants: {
        Row: {
          address: string
          application_data: Json | null
          auto_close_at: string | null
          bind_code: string | null
          bind_code_expires_at: string | null
          boss_bind_code: string | null
          boss_bind_code_expires_at: string | null
          created_at: string
          deleted_at: string | null
          description: string | null
          id: string
          is_open: boolean
          latitude: number | null
          logo_url: string | null
          longitude: number | null
          name: string
          owner_user_id: string
          pending_owner_bind: boolean | null
          phone: string
          region_id: string
          status: string
          updated_at: string
          version: number
        }
        Insert: {
          address: string
          application_data?: Json | null
          auto_close_at?: string | null
          bind_code?: string | null
          bind_code_expires_at?: string | null
          boss_bind_code?: string | null
          boss_bind_code_expires_at?: string | null
          created_at?: string
          deleted_at?: string | null
          description?: string | null
          id?: string
          is_open?: boolean
          latitude?: number | null
          logo_url?: string | null
          longitude?: number | null
          name: string
          owner_user_id: string
          pending_owner_bind?: boolean | null
          phone: string
          region_id: string
          status?: string
          updated_at?: string
          version?: number
        }
        Update: {
          address?: string
          application_data?: Json | null
          auto_close_at?: string | null
          bind_code?: string | null
          bind_code_expires_at?: string | null
          boss_bind_code?: string | null
          boss_bind_code_expires_at?: string | null
          created_at?: string
          deleted_at?: string | null
          description?: string | null
          id?: string
          is_open?: boolean
          latitude?: number | null
          logo_url?: string | null
          longitude?: number | null
          name?: string
          owner_user_id?: string
          pending_owner_bind?: boolean | null
          phone?: string
          region_id?: string
          status?: string
          updated_at?: string
          version?: number
        }
        Relationships: [
          {
            foreignKeyName: "merchants_owner_user_id_fkey"
            columns: ["owner_user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "merchants_region_id_fkey"
            columns: ["region_id"]
            isOneToOne: false
            referencedRelation: "regions"
            referencedColumns: ["id"]
          },
        ]
      }
      notifications: {
        Row: {
          content: string
          created_at: string
          expires_at: string | null
          extra_data: Json | null
          id: string
          is_pushed: boolean
          is_read: boolean
          pushed_at: string | null
          read_at: string | null
          related_id: string | null
          related_type: string | null
          title: string
          type: string
          user_id: string
        }
        Insert: {
          content: string
          created_at?: string
          expires_at?: string | null
          extra_data?: Json | null
          id?: string
          is_pushed?: boolean
          is_read?: boolean
          pushed_at?: string | null
          read_at?: string | null
          related_id?: string | null
          related_type?: string | null
          title: string
          type: string
          user_id: string
        }
        Update: {
          content?: string
          created_at?: string
          expires_at?: string | null
          extra_data?: Json | null
          id?: string
          is_pushed?: boolean
          is_read?: boolean
          pushed_at?: string | null
          read_at?: string | null
          related_id?: string | null
          related_type?: string | null
          title?: string
          type?: string
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "notifications_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      operator_applications: {
        Row: {
          business_license_number: string | null
          business_license_ocr: Json | null
          business_license_url: string | null
          contact_name: string | null
          contact_phone: string | null
          created_at: string
          id: string
          id_card_back_ocr: Json | null
          id_card_back_url: string | null
          id_card_front_ocr: Json | null
          id_card_front_url: string | null
          legal_person_id_number: string | null
          legal_person_name: string | null
          name: string | null
          region_id: string
          reject_reason: string | null
          requested_contract_years: number
          reviewed_at: string | null
          reviewed_by: string | null
          status: string
          submitted_at: string | null
          updated_at: string
          user_id: string
        }
        Insert: {
          business_license_number?: string | null
          business_license_ocr?: Json | null
          business_license_url?: string | null
          contact_name?: string | null
          contact_phone?: string | null
          created_at?: string
          id?: string
          id_card_back_ocr?: Json | null
          id_card_back_url?: string | null
          id_card_front_ocr?: Json | null
          id_card_front_url?: string | null
          legal_person_id_number?: string | null
          legal_person_name?: string | null
          name?: string | null
          region_id: string
          reject_reason?: string | null
          requested_contract_years?: number
          reviewed_at?: string | null
          reviewed_by?: string | null
          status?: string
          submitted_at?: string | null
          updated_at?: string
          user_id: string
        }
        Update: {
          business_license_number?: string | null
          business_license_ocr?: Json | null
          business_license_url?: string | null
          contact_name?: string | null
          contact_phone?: string | null
          created_at?: string
          id?: string
          id_card_back_ocr?: Json | null
          id_card_back_url?: string | null
          id_card_front_ocr?: Json | null
          id_card_front_url?: string | null
          legal_person_id_number?: string | null
          legal_person_name?: string | null
          name?: string | null
          region_id?: string
          reject_reason?: string | null
          requested_contract_years?: number
          reviewed_at?: string | null
          reviewed_by?: string | null
          status?: string
          submitted_at?: string | null
          updated_at?: string
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "operator_applications_region_id_fkey"
            columns: ["region_id"]
            isOneToOne: false
            referencedRelation: "regions"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "operator_applications_reviewed_by_fkey"
            columns: ["reviewed_by"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "operator_applications_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: true
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      operator_regions: {
        Row: {
          created_at: string
          id: string
          operator_id: string
          region_id: string
          status: string
        }
        Insert: {
          created_at?: string
          id?: string
          operator_id: string
          region_id: string
          status?: string
        }
        Update: {
          created_at?: string
          id?: string
          operator_id?: string
          region_id?: string
          status?: string
        }
        Relationships: [
          {
            foreignKeyName: "operator_regions_operator_id_fkey"
            columns: ["operator_id"]
            isOneToOne: false
            referencedRelation: "operators"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "operator_regions_region_id_fkey"
            columns: ["region_id"]
            isOneToOne: false
            referencedRelation: "regions"
            referencedColumns: ["id"]
          },
        ]
      }
      operators: {
        Row: {
          commission_rate: number
          contact_name: string
          contact_phone: string
          contract_end_date: string | null
          contract_start_date: string | null
          contract_years: number
          created_at: string
          id: string
          name: string
          region_id: string
          status: string
          sub_mch_id: string | null
          updated_at: string | null
          user_id: string
          wechat_mch_id: string | null
        }
        Insert: {
          commission_rate?: number
          contact_name: string
          contact_phone: string
          contract_end_date?: string | null
          contract_start_date?: string | null
          contract_years?: number
          created_at?: string
          id?: string
          name: string
          region_id: string
          status?: string
          sub_mch_id?: string | null
          updated_at?: string | null
          user_id: string
          wechat_mch_id?: string | null
        }
        Update: {
          commission_rate?: number
          contact_name?: string
          contact_phone?: string
          contract_end_date?: string | null
          contract_start_date?: string | null
          contract_years?: number
          created_at?: string
          id?: string
          name?: string
          region_id?: string
          status?: string
          sub_mch_id?: string | null
          updated_at?: string | null
          user_id?: string
          wechat_mch_id?: string | null
        }
        Relationships: [
          {
            foreignKeyName: "operators_region_id_fkey"
            columns: ["region_id"]
            isOneToOne: false
            referencedRelation: "regions"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "operators_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: true
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      order_display_configs: {
        Row: {
          created_at: string
          enable_kds: boolean
          enable_print: boolean
          enable_voice: boolean
          id: string
          kds_url: string | null
          merchant_id: string
          print_dine_in: boolean
          print_reservation: boolean
          print_takeout: boolean
          updated_at: string | null
          voice_dine_in: boolean
          voice_takeout: boolean
        }
        Insert: {
          created_at?: string
          enable_kds?: boolean
          enable_print?: boolean
          enable_voice?: boolean
          id?: string
          kds_url?: string | null
          merchant_id: string
          print_dine_in?: boolean
          print_reservation?: boolean
          print_takeout?: boolean
          updated_at?: string | null
          voice_dine_in?: boolean
          voice_takeout?: boolean
        }
        Update: {
          created_at?: string
          enable_kds?: boolean
          enable_print?: boolean
          enable_voice?: boolean
          id?: string
          kds_url?: string | null
          merchant_id?: string
          print_dine_in?: boolean
          print_reservation?: boolean
          print_takeout?: boolean
          updated_at?: string | null
          voice_dine_in?: boolean
          voice_takeout?: boolean
        }
        Relationships: [
          {
            foreignKeyName: "order_display_configs_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: true
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
        ]
      }
      order_items: {
        Row: {
          combo_id: string | null
          created_at: string
          customizations: Json | null
          dish_id: string | null
          id: string
          name: string
          order_id: string
          quantity: number
          subtotal: number
          unit_price: number
        }
        Insert: {
          combo_id?: string | null
          created_at?: string
          customizations?: Json | null
          dish_id?: string | null
          id?: string
          name: string
          order_id: string
          quantity: number
          subtotal: number
          unit_price: number
        }
        Update: {
          combo_id?: string | null
          created_at?: string
          customizations?: Json | null
          dish_id?: string | null
          id?: string
          name?: string
          order_id?: string
          quantity?: number
          subtotal?: number
          unit_price?: number
        }
        Relationships: [
          {
            foreignKeyName: "order_items_combo_id_fkey"
            columns: ["combo_id"]
            isOneToOne: false
            referencedRelation: "combo_sets"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "order_items_dish_id_fkey"
            columns: ["dish_id"]
            isOneToOne: false
            referencedRelation: "dishes"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "order_items_order_id_fkey"
            columns: ["order_id"]
            isOneToOne: false
            referencedRelation: "orders"
            referencedColumns: ["id"]
          },
        ]
      }
      order_status_logs: {
        Row: {
          created_at: string
          from_status: string | null
          id: string
          notes: string | null
          operator_id: string | null
          operator_type: string | null
          order_id: string
          to_status: string
        }
        Insert: {
          created_at?: string
          from_status?: string | null
          id?: string
          notes?: string | null
          operator_id?: string | null
          operator_type?: string | null
          order_id: string
          to_status: string
        }
        Update: {
          created_at?: string
          from_status?: string | null
          id?: string
          notes?: string | null
          operator_id?: string | null
          operator_type?: string | null
          order_id?: string
          to_status?: string
        }
        Relationships: [
          {
            foreignKeyName: "order_status_logs_operator_id_fkey"
            columns: ["operator_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "order_status_logs_order_id_fkey"
            columns: ["order_id"]
            isOneToOne: false
            referencedRelation: "orders"
            referencedColumns: ["id"]
          },
        ]
      }
      orders: {
        Row: {
          address_id: string | null
          balance_paid: number
          cancel_reason: string | null
          cancelled_at: string | null
          completed_at: string | null
          created_at: string
          delivery_distance: number | null
          delivery_fee: number
          delivery_fee_discount: number
          discount_amount: number
          final_amount: number | null
          id: string
          membership_id: string | null
          merchant_id: string
          notes: string | null
          order_no: string
          order_type: string
          paid_at: string | null
          payment_method: string | null
          platform_commission: number | null
          reservation_id: string | null
          status: string
          subtotal: number
          table_id: string | null
          total_amount: number
          updated_at: string | null
          user_id: string
          user_voucher_id: string | null
          voucher_amount: number
        }
        Insert: {
          address_id?: string | null
          balance_paid?: number
          cancel_reason?: string | null
          cancelled_at?: string | null
          completed_at?: string | null
          created_at?: string
          delivery_distance?: number | null
          delivery_fee?: number
          delivery_fee_discount?: number
          discount_amount?: number
          final_amount?: number | null
          id?: string
          membership_id?: string | null
          merchant_id: string
          notes?: string | null
          order_no: string
          order_type: string
          paid_at?: string | null
          payment_method?: string | null
          platform_commission?: number | null
          reservation_id?: string | null
          status?: string
          subtotal: number
          table_id?: string | null
          total_amount: number
          updated_at?: string | null
          user_id: string
          user_voucher_id?: string | null
          voucher_amount?: number
        }
        Update: {
          address_id?: string | null
          balance_paid?: number
          cancel_reason?: string | null
          cancelled_at?: string | null
          completed_at?: string | null
          created_at?: string
          delivery_distance?: number | null
          delivery_fee?: number
          delivery_fee_discount?: number
          discount_amount?: number
          final_amount?: number | null
          id?: string
          membership_id?: string | null
          merchant_id?: string
          notes?: string | null
          order_no?: string
          order_type?: string
          paid_at?: string | null
          payment_method?: string | null
          platform_commission?: number | null
          reservation_id?: string | null
          status?: string
          subtotal?: number
          table_id?: string | null
          total_amount?: number
          updated_at?: string | null
          user_id?: string
          user_voucher_id?: string | null
          voucher_amount?: number
        }
        Relationships: [
          {
            foreignKeyName: "orders_address_id_fkey"
            columns: ["address_id"]
            isOneToOne: false
            referencedRelation: "user_addresses"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "orders_membership_id_fkey"
            columns: ["membership_id"]
            isOneToOne: false
            referencedRelation: "merchant_memberships"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "orders_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "orders_reservation_id_fkey"
            columns: ["reservation_id"]
            isOneToOne: false
            referencedRelation: "table_reservations"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "orders_table_id_fkey"
            columns: ["table_id"]
            isOneToOne: false
            referencedRelation: "tables"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "orders_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "orders_user_voucher_id_fkey"
            columns: ["user_voucher_id"]
            isOneToOne: false
            referencedRelation: "user_vouchers"
            referencedColumns: ["id"]
          },
        ]
      }
      payment_orders: {
        Row: {
          amount: number
          attach: string | null
          business_type: string
          combined_payment_id: string | null
          created_at: string
          expires_at: string | null
          id: string
          order_id: string | null
          out_trade_no: string
          paid_at: string | null
          payment_type: string
          prepay_id: string | null
          reservation_id: string | null
          status: string
          transaction_id: string | null
          user_id: string
        }
        Insert: {
          amount: number
          attach?: string | null
          business_type: string
          combined_payment_id?: string | null
          created_at?: string
          expires_at?: string | null
          id?: string
          order_id?: string | null
          out_trade_no: string
          paid_at?: string | null
          payment_type: string
          prepay_id?: string | null
          reservation_id?: string | null
          status?: string
          transaction_id?: string | null
          user_id: string
        }
        Update: {
          amount?: number
          attach?: string | null
          business_type?: string
          combined_payment_id?: string | null
          created_at?: string
          expires_at?: string | null
          id?: string
          order_id?: string | null
          out_trade_no?: string
          paid_at?: string | null
          payment_type?: string
          prepay_id?: string | null
          reservation_id?: string | null
          status?: string
          transaction_id?: string | null
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "payment_orders_combined_payment_id_fkey"
            columns: ["combined_payment_id"]
            isOneToOne: false
            referencedRelation: "combined_payment_orders"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "payment_orders_order_id_fkey"
            columns: ["order_id"]
            isOneToOne: false
            referencedRelation: "orders"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "payment_orders_reservation_id_fkey"
            columns: ["reservation_id"]
            isOneToOne: false
            referencedRelation: "table_reservations"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "payment_orders_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      peak_hour_configs: {
        Row: {
          coefficient: number
          created_at: string
          days_of_week: number[]
          end_time: string
          id: string
          is_active: boolean
          name: string
          region_id: string
          start_time: string
          updated_at: string | null
        }
        Insert: {
          coefficient: number
          created_at?: string
          days_of_week: number[]
          end_time: string
          id?: string
          is_active?: boolean
          name: string
          region_id: string
          start_time: string
          updated_at?: string | null
        }
        Update: {
          coefficient?: number
          created_at?: string
          days_of_week?: number[]
          end_time?: string
          id?: string
          is_active?: boolean
          name?: string
          region_id?: string
          start_time?: string
          updated_at?: string | null
        }
        Relationships: [
          {
            foreignKeyName: "peak_hour_configs_region_id_fkey"
            columns: ["region_id"]
            isOneToOne: false
            referencedRelation: "regions"
            referencedColumns: ["id"]
          },
        ]
      }
      print_logs: {
        Row: {
          created_at: string
          error_message: string | null
          id: string
          order_id: string
          print_content: string
          printed_at: string | null
          printer_id: string
          status: string
        }
        Insert: {
          created_at?: string
          error_message?: string | null
          id?: string
          order_id: string
          print_content: string
          printed_at?: string | null
          printer_id: string
          status?: string
        }
        Update: {
          created_at?: string
          error_message?: string | null
          id?: string
          order_id?: string
          print_content?: string
          printed_at?: string | null
          printer_id?: string
          status?: string
        }
        Relationships: [
          {
            foreignKeyName: "print_logs_order_id_fkey"
            columns: ["order_id"]
            isOneToOne: false
            referencedRelation: "orders"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "print_logs_printer_id_fkey"
            columns: ["printer_id"]
            isOneToOne: false
            referencedRelation: "cloud_printers"
            referencedColumns: ["id"]
          },
        ]
      }
      profit_sharing_orders: {
        Row: {
          created_at: string
          delivery_fee: number
          distributable_amount: number
          finished_at: string | null
          id: string
          merchant_amount: number
          merchant_id: string
          operator_commission: number
          operator_id: string | null
          operator_rate: number
          order_source: string
          out_order_no: string
          payment_order_id: string
          platform_commission: number
          platform_rate: number
          rider_amount: number
          rider_id: string | null
          sharing_order_id: string | null
          status: string
          total_amount: number
        }
        Insert: {
          created_at?: string
          delivery_fee?: number
          distributable_amount?: number
          finished_at?: string | null
          id?: string
          merchant_amount: number
          merchant_id: string
          operator_commission?: number
          operator_id?: string | null
          operator_rate?: number
          order_source: string
          out_order_no: string
          payment_order_id: string
          platform_commission?: number
          platform_rate?: number
          rider_amount?: number
          rider_id?: string | null
          sharing_order_id?: string | null
          status?: string
          total_amount: number
        }
        Update: {
          created_at?: string
          delivery_fee?: number
          distributable_amount?: number
          finished_at?: string | null
          id?: string
          merchant_amount?: number
          merchant_id?: string
          operator_commission?: number
          operator_id?: string | null
          operator_rate?: number
          order_source?: string
          out_order_no?: string
          payment_order_id?: string
          platform_commission?: number
          platform_rate?: number
          rider_amount?: number
          rider_id?: string | null
          sharing_order_id?: string | null
          status?: string
          total_amount?: number
        }
        Relationships: [
          {
            foreignKeyName: "profit_sharing_orders_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "profit_sharing_orders_operator_id_fkey"
            columns: ["operator_id"]
            isOneToOne: false
            referencedRelation: "operators"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "profit_sharing_orders_payment_order_id_fkey"
            columns: ["payment_order_id"]
            isOneToOne: false
            referencedRelation: "payment_orders"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "profit_sharing_orders_rider_id_fkey"
            columns: ["rider_id"]
            isOneToOne: false
            referencedRelation: "riders"
            referencedColumns: ["id"]
          },
        ]
      }
      recharge_rules: {
        Row: {
          bonus_amount: number
          created_at: string
          id: string
          is_active: boolean
          merchant_id: string
          recharge_amount: number
          updated_at: string | null
          valid_from: string
          valid_until: string
        }
        Insert: {
          bonus_amount: number
          created_at?: string
          id?: string
          is_active?: boolean
          merchant_id: string
          recharge_amount: number
          updated_at?: string | null
          valid_from: string
          valid_until: string
        }
        Update: {
          bonus_amount?: number
          created_at?: string
          id?: string
          is_active?: boolean
          merchant_id?: string
          recharge_amount?: number
          updated_at?: string | null
          valid_from?: string
          valid_until?: string
        }
        Relationships: [
          {
            foreignKeyName: "recharge_rules_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
        ]
      }
      recommend_configs: {
        Row: {
          created_at: string
          distance_weight: number
          id: string
          is_active: boolean
          max_distance: number
          max_results: number
          name: string
          profit_weight: number
          route_weight: number
          updated_at: string | null
          urgency_weight: number
        }
        Insert: {
          created_at?: string
          distance_weight?: number
          id?: string
          is_active?: boolean
          max_distance?: number
          max_results?: number
          name: string
          profit_weight?: number
          route_weight?: number
          updated_at?: string | null
          urgency_weight?: number
        }
        Update: {
          created_at?: string
          distance_weight?: number
          id?: string
          is_active?: boolean
          max_distance?: number
          max_results?: number
          name?: string
          profit_weight?: number
          route_weight?: number
          updated_at?: string | null
          urgency_weight?: number
        }
        Relationships: []
      }
      recommendation_configs: {
        Row: {
          auto_adjust: boolean
          exploitation_ratio: number
          exploration_ratio: number
          id: string
          random_ratio: number
          region_id: string
          updated_at: string
        }
        Insert: {
          auto_adjust?: boolean
          exploitation_ratio?: number
          exploration_ratio?: number
          id?: string
          random_ratio?: number
          region_id: string
          updated_at?: string
        }
        Update: {
          auto_adjust?: boolean
          exploitation_ratio?: number
          exploration_ratio?: number
          id?: string
          random_ratio?: number
          region_id?: string
          updated_at?: string
        }
        Relationships: [
          {
            foreignKeyName: "recommendation_configs_region_id_fkey"
            columns: ["region_id"]
            isOneToOne: true
            referencedRelation: "regions"
            referencedColumns: ["id"]
          },
        ]
      }
      recommendations: {
        Row: {
          algorithm: string
          combo_ids: number[] | null
          dish_ids: number[] | null
          expired_at: string
          generated_at: string
          id: string
          merchant_ids: number[] | null
          score: number | null
          user_id: string
        }
        Insert: {
          algorithm: string
          combo_ids?: number[] | null
          dish_ids?: number[] | null
          expired_at: string
          generated_at?: string
          id?: string
          merchant_ids?: number[] | null
          score?: number | null
          user_id: string
        }
        Update: {
          algorithm?: string
          combo_ids?: number[] | null
          dish_ids?: number[] | null
          expired_at?: string
          generated_at?: string
          id?: string
          merchant_ids?: number[] | null
          score?: number | null
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "recommendations_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      refund_orders: {
        Row: {
          created_at: string
          id: string
          merchant_refund: number | null
          operator_refund: number | null
          out_refund_no: string
          payment_order_id: string
          platform_refund: number | null
          refund_amount: number
          refund_id: string | null
          refund_reason: string | null
          refund_type: string
          refunded_at: string | null
          status: string
        }
        Insert: {
          created_at?: string
          id?: string
          merchant_refund?: number | null
          operator_refund?: number | null
          out_refund_no: string
          payment_order_id: string
          platform_refund?: number | null
          refund_amount: number
          refund_id?: string | null
          refund_reason?: string | null
          refund_type: string
          refunded_at?: string | null
          status?: string
        }
        Update: {
          created_at?: string
          id?: string
          merchant_refund?: number | null
          operator_refund?: number | null
          out_refund_no?: string
          payment_order_id?: string
          platform_refund?: number | null
          refund_amount?: number
          refund_id?: string | null
          refund_reason?: string | null
          refund_type?: string
          refunded_at?: string | null
          status?: string
        }
        Relationships: [
          {
            foreignKeyName: "refund_orders_payment_order_id_fkey"
            columns: ["payment_order_id"]
            isOneToOne: false
            referencedRelation: "payment_orders"
            referencedColumns: ["id"]
          },
        ]
      }
      regions: {
        Row: {
          code: string
          created_at: string
          id: string
          latitude: number | null
          level: number
          longitude: number | null
          name: string
          parent_id: string | null
          qweather_location_id: string | null
        }
        Insert: {
          code: string
          created_at?: string
          id?: string
          latitude?: number | null
          level: number
          longitude?: number | null
          name: string
          parent_id?: string | null
          qweather_location_id?: string | null
        }
        Update: {
          code?: string
          created_at?: string
          id?: string
          latitude?: number | null
          level?: number
          longitude?: number | null
          name?: string
          parent_id?: string | null
          qweather_location_id?: string | null
        }
        Relationships: [
          {
            foreignKeyName: "regions_parent_id_fkey"
            columns: ["parent_id"]
            isOneToOne: false
            referencedRelation: "regions"
            referencedColumns: ["id"]
          },
        ]
      }
      reservation_items: {
        Row: {
          combo_id: string | null
          created_at: string
          dish_id: string | null
          id: string
          quantity: number
          reservation_id: string
          total_price: number
          unit_price: number
        }
        Insert: {
          combo_id?: string | null
          created_at?: string
          dish_id?: string | null
          id?: string
          quantity: number
          reservation_id: string
          total_price: number
          unit_price: number
        }
        Update: {
          combo_id?: string | null
          created_at?: string
          dish_id?: string | null
          id?: string
          quantity?: number
          reservation_id?: string
          total_price?: number
          unit_price?: number
        }
        Relationships: [
          {
            foreignKeyName: "reservation_items_combo_id_fkey"
            columns: ["combo_id"]
            isOneToOne: false
            referencedRelation: "combo_sets"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "reservation_items_dish_id_fkey"
            columns: ["dish_id"]
            isOneToOne: false
            referencedRelation: "dishes"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "reservation_items_reservation_id_fkey"
            columns: ["reservation_id"]
            isOneToOne: false
            referencedRelation: "table_reservations"
            referencedColumns: ["id"]
          },
        ]
      }
      reviews: {
        Row: {
          content: string
          created_at: string
          id: string
          images: string[] | null
          is_visible: boolean
          merchant_id: string
          merchant_reply: string | null
          order_id: string
          replied_at: string | null
          user_id: string
        }
        Insert: {
          content: string
          created_at?: string
          id?: string
          images?: string[] | null
          is_visible?: boolean
          merchant_id: string
          merchant_reply?: string | null
          order_id: string
          replied_at?: string | null
          user_id: string
        }
        Update: {
          content?: string
          created_at?: string
          id?: string
          images?: string[] | null
          is_visible?: boolean
          merchant_id?: string
          merchant_reply?: string | null
          order_id?: string
          replied_at?: string | null
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "reviews_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "reviews_order_id_fkey"
            columns: ["order_id"]
            isOneToOne: false
            referencedRelation: "orders"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "reviews_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      rider_applications: {
        Row: {
          created_at: string
          health_cert_ocr: Json | null
          health_cert_url: string | null
          id: string
          id_card_back_url: string | null
          id_card_front_url: string | null
          id_card_ocr: Json | null
          phone: string | null
          real_name: string | null
          reject_reason: string | null
          reviewed_at: string | null
          reviewed_by: string | null
          status: string
          submitted_at: string | null
          updated_at: string | null
          user_id: string
        }
        Insert: {
          created_at?: string
          health_cert_ocr?: Json | null
          health_cert_url?: string | null
          id?: string
          id_card_back_url?: string | null
          id_card_front_url?: string | null
          id_card_ocr?: Json | null
          phone?: string | null
          real_name?: string | null
          reject_reason?: string | null
          reviewed_at?: string | null
          reviewed_by?: string | null
          status?: string
          submitted_at?: string | null
          updated_at?: string | null
          user_id: string
        }
        Update: {
          created_at?: string
          health_cert_ocr?: Json | null
          health_cert_url?: string | null
          id?: string
          id_card_back_url?: string | null
          id_card_front_url?: string | null
          id_card_ocr?: Json | null
          phone?: string | null
          real_name?: string | null
          reject_reason?: string | null
          reviewed_at?: string | null
          reviewed_by?: string | null
          status?: string
          submitted_at?: string | null
          updated_at?: string | null
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "rider_applications_reviewed_by_fkey"
            columns: ["reviewed_by"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "rider_applications_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: true
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      rider_deposits: {
        Row: {
          amount: number
          balance_after: number
          created_at: string
          id: string
          related_order_id: string | null
          remark: string | null
          rider_id: string
          type: string
        }
        Insert: {
          amount: number
          balance_after: number
          created_at?: string
          id?: string
          related_order_id?: string | null
          remark?: string | null
          rider_id: string
          type: string
        }
        Update: {
          amount?: number
          balance_after?: number
          created_at?: string
          id?: string
          related_order_id?: string | null
          remark?: string | null
          rider_id?: string
          type?: string
        }
        Relationships: [
          {
            foreignKeyName: "rider_deposits_related_order_id_fkey"
            columns: ["related_order_id"]
            isOneToOne: false
            referencedRelation: "orders"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "rider_deposits_rider_id_fkey"
            columns: ["rider_id"]
            isOneToOne: false
            referencedRelation: "riders"
            referencedColumns: ["id"]
          },
        ]
      }
      rider_locations: {
        Row: {
          accuracy: number | null
          delivery_id: string | null
          heading: number | null
          id: string
          latitude: number
          longitude: number
          recorded_at: string
          rider_id: string
          speed: number | null
        }
        Insert: {
          accuracy?: number | null
          delivery_id?: string | null
          heading?: number | null
          id?: string
          latitude: number
          longitude: number
          recorded_at?: string
          rider_id: string
          speed?: number | null
        }
        Update: {
          accuracy?: number | null
          delivery_id?: string | null
          heading?: number | null
          id?: string
          latitude?: number
          longitude?: number
          recorded_at?: string
          rider_id?: string
          speed?: number | null
        }
        Relationships: [
          {
            foreignKeyName: "rider_locations_delivery_id_fkey"
            columns: ["delivery_id"]
            isOneToOne: false
            referencedRelation: "deliveries"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "rider_locations_rider_id_fkey"
            columns: ["rider_id"]
            isOneToOne: false
            referencedRelation: "riders"
            referencedColumns: ["id"]
          },
        ]
      }
      rider_premium_score_logs: {
        Row: {
          change_amount: number
          change_type: string
          created_at: string
          id: string
          new_score: number
          old_score: number
          related_delivery_id: string | null
          related_order_id: string | null
          remark: string | null
          rider_id: string
        }
        Insert: {
          change_amount: number
          change_type: string
          created_at?: string
          id?: string
          new_score: number
          old_score: number
          related_delivery_id?: string | null
          related_order_id?: string | null
          remark?: string | null
          rider_id: string
        }
        Update: {
          change_amount?: number
          change_type?: string
          created_at?: string
          id?: string
          new_score?: number
          old_score?: number
          related_delivery_id?: string | null
          related_order_id?: string | null
          remark?: string | null
          rider_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "rider_premium_score_logs_related_delivery_id_fkey"
            columns: ["related_delivery_id"]
            isOneToOne: false
            referencedRelation: "deliveries"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "rider_premium_score_logs_related_order_id_fkey"
            columns: ["related_order_id"]
            isOneToOne: false
            referencedRelation: "orders"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "rider_premium_score_logs_rider_id_fkey"
            columns: ["rider_id"]
            isOneToOne: false
            referencedRelation: "riders"
            referencedColumns: ["id"]
          },
        ]
      }
      rider_profiles: {
        Row: {
          cancelled_deliveries: number
          completed_deliveries: number
          customer_complaints: number
          delayed_deliveries: number
          id: string
          is_suspended: boolean
          on_time_deliveries: number
          premium_score: number
          recent_30d_complaints: number
          recent_30d_damages: number
          recent_30d_delays: number
          recent_7d_damages: number
          recent_7d_delays: number
          recent_90d_damages: number
          recent_90d_delays: number
          rider_id: string
          suspend_reason: string | null
          suspend_until: string | null
          suspended_at: string | null
          timeout_incidents: number
          total_damage_incidents: number
          total_deliveries: number
          total_online_hours: number
          trust_score: number
          updated_at: string
        }
        Insert: {
          cancelled_deliveries?: number
          completed_deliveries?: number
          customer_complaints?: number
          delayed_deliveries?: number
          id?: string
          is_suspended?: boolean
          on_time_deliveries?: number
          premium_score?: number
          recent_30d_complaints?: number
          recent_30d_damages?: number
          recent_30d_delays?: number
          recent_7d_damages?: number
          recent_7d_delays?: number
          recent_90d_damages?: number
          recent_90d_delays?: number
          rider_id: string
          suspend_reason?: string | null
          suspend_until?: string | null
          suspended_at?: string | null
          timeout_incidents?: number
          total_damage_incidents?: number
          total_deliveries?: number
          total_online_hours?: number
          trust_score?: number
          updated_at?: string
        }
        Update: {
          cancelled_deliveries?: number
          completed_deliveries?: number
          customer_complaints?: number
          delayed_deliveries?: number
          id?: string
          is_suspended?: boolean
          on_time_deliveries?: number
          premium_score?: number
          recent_30d_complaints?: number
          recent_30d_damages?: number
          recent_30d_delays?: number
          recent_7d_damages?: number
          recent_7d_delays?: number
          recent_90d_damages?: number
          recent_90d_delays?: number
          rider_id?: string
          suspend_reason?: string | null
          suspend_until?: string | null
          suspended_at?: string | null
          timeout_incidents?: number
          total_damage_incidents?: number
          total_deliveries?: number
          total_online_hours?: number
          trust_score?: number
          updated_at?: string
        }
        Relationships: [
          {
            foreignKeyName: "rider_profiles_rider_id_fkey"
            columns: ["rider_id"]
            isOneToOne: true
            referencedRelation: "riders"
            referencedColumns: ["id"]
          },
        ]
      }
      riders: {
        Row: {
          application_id: string | null
          created_at: string
          credit_score: number
          current_latitude: number | null
          current_longitude: number | null
          deposit_amount: number
          frozen_deposit: number
          id: string
          id_card_no: string
          is_online: boolean
          location_updated_at: string | null
          online_duration: number
          phone: string
          real_name: string
          region_id: string | null
          status: string
          sub_mch_id: string | null
          total_earnings: number
          total_orders: number
          updated_at: string | null
          user_id: string
        }
        Insert: {
          application_id?: string | null
          created_at?: string
          credit_score?: number
          current_latitude?: number | null
          current_longitude?: number | null
          deposit_amount?: number
          frozen_deposit?: number
          id?: string
          id_card_no: string
          is_online?: boolean
          location_updated_at?: string | null
          online_duration?: number
          phone: string
          real_name: string
          region_id?: string | null
          status?: string
          sub_mch_id?: string | null
          total_earnings?: number
          total_orders?: number
          updated_at?: string | null
          user_id: string
        }
        Update: {
          application_id?: string | null
          created_at?: string
          credit_score?: number
          current_latitude?: number | null
          current_longitude?: number | null
          deposit_amount?: number
          frozen_deposit?: number
          id?: string
          id_card_no?: string
          is_online?: boolean
          location_updated_at?: string | null
          online_duration?: number
          phone?: string
          real_name?: string
          region_id?: string | null
          status?: string
          sub_mch_id?: string | null
          total_earnings?: number
          total_orders?: number
          updated_at?: string | null
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "riders_application_id_fkey"
            columns: ["application_id"]
            isOneToOne: false
            referencedRelation: "rider_applications"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "riders_region_id_fkey"
            columns: ["region_id"]
            isOneToOne: false
            referencedRelation: "regions"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "riders_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      schema_migrations: {
        Row: {
          dirty: boolean
          version: number
        }
        Insert: {
          dirty: boolean
          version: number
        }
        Update: {
          dirty?: boolean
          version?: number
        }
        Relationships: []
      }
      sessions: {
        Row: {
          access_token: string
          access_token_expires_at: string
          client_ip: string
          created_at: string
          id: string
          is_revoked: boolean
          refresh_token: string
          refresh_token_expires_at: string
          user_agent: string
          user_id: string
        }
        Insert: {
          access_token: string
          access_token_expires_at: string
          client_ip: string
          created_at?: string
          id?: string
          is_revoked?: boolean
          refresh_token: string
          refresh_token_expires_at: string
          user_agent: string
          user_id: string
        }
        Update: {
          access_token?: string
          access_token_expires_at?: string
          client_ip?: string
          created_at?: string
          id?: string
          is_revoked?: boolean
          refresh_token?: string
          refresh_token_expires_at?: string
          user_agent?: string
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "sessions_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      table_images: {
        Row: {
          created_at: string
          id: string
          image_url: string
          is_primary: boolean
          sort_order: number
          table_id: string
        }
        Insert: {
          created_at?: string
          id?: string
          image_url: string
          is_primary?: boolean
          sort_order?: number
          table_id: string
        }
        Update: {
          created_at?: string
          id?: string
          image_url?: string
          is_primary?: boolean
          sort_order?: number
          table_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "table_images_table_id_fkey"
            columns: ["table_id"]
            isOneToOne: false
            referencedRelation: "tables"
            referencedColumns: ["id"]
          },
        ]
      }
      table_reservations: {
        Row: {
          cancel_reason: string | null
          cancelled_at: string | null
          checked_in_at: string | null
          completed_at: string | null
          confirmed_at: string | null
          contact_name: string
          contact_phone: string
          cooking_started_at: string | null
          created_at: string
          deposit_amount: number
          guest_count: number
          id: string
          merchant_id: string
          notes: string | null
          paid_at: string | null
          payment_deadline: string
          payment_mode: string
          prepaid_amount: number
          refund_deadline: string
          reservation_date: string
          reservation_time: string
          source: string | null
          status: string
          table_id: string
          updated_at: string | null
          user_id: string
        }
        Insert: {
          cancel_reason?: string | null
          cancelled_at?: string | null
          checked_in_at?: string | null
          completed_at?: string | null
          confirmed_at?: string | null
          contact_name: string
          contact_phone: string
          cooking_started_at?: string | null
          created_at?: string
          deposit_amount?: number
          guest_count: number
          id?: string
          merchant_id: string
          notes?: string | null
          paid_at?: string | null
          payment_deadline: string
          payment_mode?: string
          prepaid_amount?: number
          refund_deadline: string
          reservation_date: string
          reservation_time: string
          source?: string | null
          status?: string
          table_id: string
          updated_at?: string | null
          user_id: string
        }
        Update: {
          cancel_reason?: string | null
          cancelled_at?: string | null
          checked_in_at?: string | null
          completed_at?: string | null
          confirmed_at?: string | null
          contact_name?: string
          contact_phone?: string
          cooking_started_at?: string | null
          created_at?: string
          deposit_amount?: number
          guest_count?: number
          id?: string
          merchant_id?: string
          notes?: string | null
          paid_at?: string | null
          payment_deadline?: string
          payment_mode?: string
          prepaid_amount?: number
          refund_deadline?: string
          reservation_date?: string
          reservation_time?: string
          source?: string | null
          status?: string
          table_id?: string
          updated_at?: string | null
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "table_reservations_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "table_reservations_table_id_fkey"
            columns: ["table_id"]
            isOneToOne: false
            referencedRelation: "tables"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "table_reservations_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      table_tags: {
        Row: {
          created_at: string
          id: string
          table_id: string
          tag_id: string
        }
        Insert: {
          created_at?: string
          id?: string
          table_id: string
          tag_id: string
        }
        Update: {
          created_at?: string
          id?: string
          table_id?: string
          tag_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "table_tags_table_id_fkey"
            columns: ["table_id"]
            isOneToOne: false
            referencedRelation: "tables"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "table_tags_tag_id_fkey"
            columns: ["tag_id"]
            isOneToOne: false
            referencedRelation: "tags"
            referencedColumns: ["id"]
          },
        ]
      }
      tables: {
        Row: {
          capacity: number
          created_at: string
          current_reservation_id: string | null
          description: string | null
          id: string
          merchant_id: string
          minimum_spend: number | null
          qr_code_url: string | null
          status: string
          table_no: string
          table_type: string
          updated_at: string | null
        }
        Insert: {
          capacity: number
          created_at?: string
          current_reservation_id?: string | null
          description?: string | null
          id?: string
          merchant_id: string
          minimum_spend?: number | null
          qr_code_url?: string | null
          status?: string
          table_no: string
          table_type?: string
          updated_at?: string | null
        }
        Update: {
          capacity?: number
          created_at?: string
          current_reservation_id?: string | null
          description?: string | null
          id?: string
          merchant_id?: string
          minimum_spend?: number | null
          qr_code_url?: string | null
          status?: string
          table_no?: string
          table_type?: string
          updated_at?: string | null
        }
        Relationships: [
          {
            foreignKeyName: "tables_current_reservation_fk"
            columns: ["current_reservation_id"]
            isOneToOne: false
            referencedRelation: "table_reservations"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "tables_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
        ]
      }
      tags: {
        Row: {
          created_at: string
          id: string
          name: string
          sort_order: number
          status: string
          type: string
        }
        Insert: {
          created_at?: string
          id?: string
          name: string
          sort_order?: number
          status?: string
          type: string
        }
        Update: {
          created_at?: string
          id?: string
          name?: string
          sort_order?: number
          status?: string
          type?: string
        }
        Relationships: []
      }
      trust_score_changes: {
        Row: {
          created_at: string
          entity_id: string
          entity_type: string
          id: string
          is_auto: boolean
          new_score: number
          old_score: number
          operator_id: string | null
          reason_description: string
          reason_type: string
          related_id: string | null
          related_type: string | null
          score_change: number
        }
        Insert: {
          created_at?: string
          entity_id: string
          entity_type: string
          id?: string
          is_auto?: boolean
          new_score: number
          old_score: number
          operator_id?: string | null
          reason_description: string
          reason_type: string
          related_id?: string | null
          related_type?: string | null
          score_change: number
        }
        Update: {
          created_at?: string
          entity_id?: string
          entity_type?: string
          id?: string
          is_auto?: boolean
          new_score?: number
          old_score?: number
          operator_id?: string | null
          reason_description?: string
          reason_type?: string
          related_id?: string | null
          related_type?: string | null
          score_change?: number
        }
        Relationships: []
      }
      user_addresses: {
        Row: {
          contact_name: string
          contact_phone: string
          created_at: string
          detail_address: string
          id: string
          is_default: boolean
          latitude: number
          longitude: number
          region_id: string
          user_id: string
        }
        Insert: {
          contact_name: string
          contact_phone: string
          created_at?: string
          detail_address: string
          id?: string
          is_default?: boolean
          latitude: number
          longitude: number
          region_id: string
          user_id: string
        }
        Update: {
          contact_name?: string
          contact_phone?: string
          created_at?: string
          detail_address?: string
          id?: string
          is_default?: boolean
          latitude?: number
          longitude?: number
          region_id?: string
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "user_addresses_region_id_fkey"
            columns: ["region_id"]
            isOneToOne: false
            referencedRelation: "regions"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "user_addresses_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      user_balance_logs: {
        Row: {
          amount: number
          balance_after: number
          balance_before: number
          created_at: string
          id: string
          related_id: string | null
          related_type: string | null
          remark: string | null
          source_id: string | null
          source_type: string | null
          type: string
          user_id: string
        }
        Insert: {
          amount: number
          balance_after: number
          balance_before: number
          created_at?: string
          id?: string
          related_id?: string | null
          related_type?: string | null
          remark?: string | null
          source_id?: string | null
          source_type?: string | null
          type: string
          user_id: string
        }
        Update: {
          amount?: number
          balance_after?: number
          balance_before?: number
          created_at?: string
          id?: string
          related_id?: string | null
          related_type?: string | null
          remark?: string | null
          source_id?: string | null
          source_type?: string | null
          type?: string
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "user_balance_logs_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      user_balances: {
        Row: {
          balance: number
          created_at: string
          frozen_balance: number
          total_expense: number
          total_income: number
          total_withdraw: number
          updated_at: string
          user_id: string
        }
        Insert: {
          balance?: number
          created_at?: string
          frozen_balance?: number
          total_expense?: number
          total_income?: number
          total_withdraw?: number
          updated_at?: string
          user_id: string
        }
        Update: {
          balance?: number
          created_at?: string
          frozen_balance?: number
          total_expense?: number
          total_income?: number
          total_withdraw?: number
          updated_at?: string
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "user_balances_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: true
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      user_behaviors: {
        Row: {
          behavior_type: string
          combo_id: string | null
          created_at: string
          dish_id: string | null
          duration: number | null
          id: string
          merchant_id: string | null
          user_id: string
        }
        Insert: {
          behavior_type: string
          combo_id?: string | null
          created_at?: string
          dish_id?: string | null
          duration?: number | null
          id?: string
          merchant_id?: string | null
          user_id: string
        }
        Update: {
          behavior_type?: string
          combo_id?: string | null
          created_at?: string
          dish_id?: string | null
          duration?: number | null
          id?: string
          merchant_id?: string | null
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "user_behaviors_combo_id_fkey"
            columns: ["combo_id"]
            isOneToOne: false
            referencedRelation: "combo_sets"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "user_behaviors_dish_id_fkey"
            columns: ["dish_id"]
            isOneToOne: false
            referencedRelation: "dishes"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "user_behaviors_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "user_behaviors_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      user_claim_warnings: {
        Row: {
          created_at: string
          id: string
          last_warning_at: string | null
          last_warning_reason: string | null
          platform_pay_count: number
          requires_evidence: boolean
          updated_at: string
          user_id: string
          warning_count: number
        }
        Insert: {
          created_at?: string
          id?: string
          last_warning_at?: string | null
          last_warning_reason?: string | null
          platform_pay_count?: number
          requires_evidence?: boolean
          updated_at?: string
          user_id: string
          warning_count?: number
        }
        Update: {
          created_at?: string
          id?: string
          last_warning_at?: string | null
          last_warning_reason?: string | null
          platform_pay_count?: number
          requires_evidence?: boolean
          updated_at?: string
          user_id?: string
          warning_count?: number
        }
        Relationships: [
          {
            foreignKeyName: "user_claim_warnings_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: true
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      user_devices: {
        Row: {
          app_version: string | null
          created_at: string
          device_id: string
          device_model: string | null
          device_type: string
          first_seen: string
          id: string
          ip_address: string | null
          last_login_at: string
          last_seen: string
          os_version: string | null
          updated_at: string
          user_agent: string | null
          user_id: string
        }
        Insert: {
          app_version?: string | null
          created_at?: string
          device_id: string
          device_model?: string | null
          device_type: string
          first_seen?: string
          id?: string
          ip_address?: string | null
          last_login_at?: string
          last_seen?: string
          os_version?: string | null
          updated_at?: string
          user_agent?: string | null
          user_id: string
        }
        Update: {
          app_version?: string | null
          created_at?: string
          device_id?: string
          device_model?: string | null
          device_type?: string
          first_seen?: string
          id?: string
          ip_address?: string | null
          last_login_at?: string
          last_seen?: string
          os_version?: string | null
          updated_at?: string
          user_agent?: string | null
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "user_devices_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      user_notification_preferences: {
        Row: {
          created_at: string
          do_not_disturb_end: string | null
          do_not_disturb_start: string | null
          enable_delivery_notifications: boolean
          enable_food_safety_notifications: boolean
          enable_order_notifications: boolean
          enable_payment_notifications: boolean
          enable_system_notifications: boolean
          id: string
          updated_at: string | null
          user_id: string
        }
        Insert: {
          created_at?: string
          do_not_disturb_end?: string | null
          do_not_disturb_start?: string | null
          enable_delivery_notifications?: boolean
          enable_food_safety_notifications?: boolean
          enable_order_notifications?: boolean
          enable_payment_notifications?: boolean
          enable_system_notifications?: boolean
          id?: string
          updated_at?: string | null
          user_id: string
        }
        Update: {
          created_at?: string
          do_not_disturb_end?: string | null
          do_not_disturb_start?: string | null
          enable_delivery_notifications?: boolean
          enable_food_safety_notifications?: boolean
          enable_order_notifications?: boolean
          enable_payment_notifications?: boolean
          enable_system_notifications?: boolean
          id?: string
          updated_at?: string | null
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "user_notification_preferences_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: true
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      user_preferences: {
        Row: {
          avg_order_amount: number | null
          cuisine_preferences: Json | null
          favorite_time_slots: number[] | null
          id: string
          last_order_date: string | null
          price_range_max: number | null
          price_range_min: number | null
          purchase_frequency: number
          top_cuisines: Json | null
          updated_at: string
          user_id: string
        }
        Insert: {
          avg_order_amount?: number | null
          cuisine_preferences?: Json | null
          favorite_time_slots?: number[] | null
          id?: string
          last_order_date?: string | null
          price_range_max?: number | null
          price_range_min?: number | null
          purchase_frequency?: number
          top_cuisines?: Json | null
          updated_at?: string
          user_id: string
        }
        Update: {
          avg_order_amount?: number | null
          cuisine_preferences?: Json | null
          favorite_time_slots?: number[] | null
          id?: string
          last_order_date?: string | null
          price_range_max?: number | null
          price_range_min?: number | null
          purchase_frequency?: number
          top_cuisines?: Json | null
          updated_at?: string
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "user_preferences_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: true
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      user_profiles: {
        Row: {
          blacklist_reason: string | null
          blacklisted_at: string | null
          cancelled_orders: number
          completed_orders: number
          food_safety_reports: number
          id: string
          is_blacklisted: boolean
          malicious_claims: number
          recent_30d_cancels: number
          recent_30d_claims: number
          recent_30d_orders: number
          recent_7d_claims: number
          recent_7d_orders: number
          recent_90d_claims: number
          recent_90d_orders: number
          role: string
          total_claims: number
          total_orders: number
          trust_score: number
          updated_at: string
          user_id: string
          verified_violations: number
        }
        Insert: {
          blacklist_reason?: string | null
          blacklisted_at?: string | null
          cancelled_orders?: number
          completed_orders?: number
          food_safety_reports?: number
          id?: string
          is_blacklisted?: boolean
          malicious_claims?: number
          recent_30d_cancels?: number
          recent_30d_claims?: number
          recent_30d_orders?: number
          recent_7d_claims?: number
          recent_7d_orders?: number
          recent_90d_claims?: number
          recent_90d_orders?: number
          role: string
          total_claims?: number
          total_orders?: number
          trust_score?: number
          updated_at?: string
          user_id: string
          verified_violations?: number
        }
        Update: {
          blacklist_reason?: string | null
          blacklisted_at?: string | null
          cancelled_orders?: number
          completed_orders?: number
          food_safety_reports?: number
          id?: string
          is_blacklisted?: boolean
          malicious_claims?: number
          recent_30d_cancels?: number
          recent_30d_claims?: number
          recent_30d_orders?: number
          recent_7d_claims?: number
          recent_7d_orders?: number
          recent_90d_claims?: number
          recent_90d_orders?: number
          role?: string
          total_claims?: number
          total_orders?: number
          trust_score?: number
          updated_at?: string
          user_id?: string
          verified_violations?: number
        }
        Relationships: [
          {
            foreignKeyName: "user_profiles_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      user_roles: {
        Row: {
          created_at: string
          id: string
          related_entity_id: string | null
          role: string
          status: string
          user_id: string
        }
        Insert: {
          created_at?: string
          id?: string
          related_entity_id?: string | null
          role: string
          status?: string
          user_id: string
        }
        Update: {
          created_at?: string
          id?: string
          related_entity_id?: string | null
          role?: string
          status?: string
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "user_roles_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
        ]
      }
      user_vouchers: {
        Row: {
          expires_at: string
          id: string
          obtained_at: string
          order_id: string | null
          status: string
          used_at: string | null
          user_id: string
          voucher_id: string
        }
        Insert: {
          expires_at: string
          id?: string
          obtained_at?: string
          order_id?: string | null
          status?: string
          used_at?: string | null
          user_id: string
          voucher_id: string
        }
        Update: {
          expires_at?: string
          id?: string
          obtained_at?: string
          order_id?: string | null
          status?: string
          used_at?: string | null
          user_id?: string
          voucher_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "user_vouchers_user_id_fkey"
            columns: ["user_id"]
            isOneToOne: false
            referencedRelation: "users"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "user_vouchers_voucher_id_fkey"
            columns: ["voucher_id"]
            isOneToOne: false
            referencedRelation: "vouchers"
            referencedColumns: ["id"]
          },
        ]
      }
      users: {
        Row: {
          avatar_url: string | null
          created_at: string
          full_name: string
          id: string
          phone: string | null
          wechat_openid: string
          wechat_unionid: string | null
        }
        Insert: {
          avatar_url?: string | null
          created_at?: string
          full_name: string
          id?: string
          phone?: string | null
          wechat_openid: string
          wechat_unionid?: string | null
        }
        Update: {
          avatar_url?: string | null
          created_at?: string
          full_name?: string
          id?: string
          phone?: string | null
          wechat_openid?: string
          wechat_unionid?: string | null
        }
        Relationships: []
      }
      vouchers: {
        Row: {
          allowed_order_types: string[]
          amount: number
          claimed_quantity: number
          code: string
          created_at: string
          deleted_at: string | null
          description: string | null
          id: string
          is_active: boolean
          merchant_id: string
          min_order_amount: number
          name: string
          total_quantity: number
          updated_at: string | null
          used_quantity: number
          valid_from: string
          valid_until: string
        }
        Insert: {
          allowed_order_types?: string[]
          amount: number
          claimed_quantity?: number
          code: string
          created_at?: string
          deleted_at?: string | null
          description?: string | null
          id?: string
          is_active?: boolean
          merchant_id: string
          min_order_amount?: number
          name: string
          total_quantity: number
          updated_at?: string | null
          used_quantity?: number
          valid_from: string
          valid_until: string
        }
        Update: {
          allowed_order_types?: string[]
          amount?: number
          claimed_quantity?: number
          code?: string
          created_at?: string
          deleted_at?: string | null
          description?: string | null
          id?: string
          is_active?: boolean
          merchant_id?: string
          min_order_amount?: number
          name?: string
          total_quantity?: number
          updated_at?: string | null
          used_quantity?: number
          valid_from?: string
          valid_until?: string
        }
        Relationships: [
          {
            foreignKeyName: "vouchers_merchant_id_fkey"
            columns: ["merchant_id"]
            isOneToOne: false
            referencedRelation: "merchants"
            referencedColumns: ["id"]
          },
        ]
      }
      weather_coefficients: {
        Row: {
          created_at: string
          delivery_suspended: boolean
          feels_like: number | null
          final_coefficient: number
          has_warning: boolean
          humidity: number | null
          id: string
          precip: number | null
          recorded_at: string
          region_id: string
          suspend_reason: string | null
          temperature: number | null
          visibility: number | null
          warning_coefficient: number
          warning_data: Json | null
          warning_level: string | null
          warning_severity: string | null
          warning_text: string | null
          warning_type: string | null
          weather_code: string | null
          weather_coefficient: number
          weather_data: Json | null
          weather_type: string
          wind_scale: string | null
          wind_speed: number | null
        }
        Insert: {
          created_at?: string
          delivery_suspended?: boolean
          feels_like?: number | null
          final_coefficient?: number
          has_warning?: boolean
          humidity?: number | null
          id?: string
          precip?: number | null
          recorded_at: string
          region_id: string
          suspend_reason?: string | null
          temperature?: number | null
          visibility?: number | null
          warning_coefficient?: number
          warning_data?: Json | null
          warning_level?: string | null
          warning_severity?: string | null
          warning_text?: string | null
          warning_type?: string | null
          weather_code?: string | null
          weather_coefficient?: number
          weather_data?: Json | null
          weather_type: string
          wind_scale?: string | null
          wind_speed?: number | null
        }
        Update: {
          created_at?: string
          delivery_suspended?: boolean
          feels_like?: number | null
          final_coefficient?: number
          has_warning?: boolean
          humidity?: number | null
          id?: string
          precip?: number | null
          recorded_at?: string
          region_id?: string
          suspend_reason?: string | null
          temperature?: number | null
          visibility?: number | null
          warning_coefficient?: number
          warning_data?: Json | null
          warning_level?: string | null
          warning_severity?: string | null
          warning_text?: string | null
          warning_type?: string | null
          weather_code?: string | null
          weather_coefficient?: number
          weather_data?: Json | null
          weather_type?: string
          wind_scale?: string | null
          wind_speed?: number | null
        }
        Relationships: [
          {
            foreignKeyName: "weather_coefficients_region_id_fkey"
            columns: ["region_id"]
            isOneToOne: false
            referencedRelation: "regions"
            referencedColumns: ["id"]
          },
        ]
      }
      wechat_access_tokens: {
        Row: {
          access_token: string
          app_type: string
          created_at: string
          expires_at: string
          id: string
        }
        Insert: {
          access_token: string
          app_type: string
          created_at?: string
          expires_at: string
          id?: string
        }
        Update: {
          access_token?: string
          app_type?: string
          created_at?: string
          expires_at?: string
          id?: string
        }
        Relationships: []
      }
      wechat_notifications: {
        Row: {
          created_at: string
          event_type: string
          id: string
          out_trade_no: string | null
          processed_at: string
          resource_type: string | null
          summary: string | null
          transaction_id: string | null
        }
        Insert: {
          created_at?: string
          event_type: string
          id: string
          out_trade_no?: string | null
          processed_at?: string
          resource_type?: string | null
          summary?: string | null
          transaction_id?: string | null
        }
        Update: {
          created_at?: string
          event_type?: string
          id?: string
          out_trade_no?: string | null
          processed_at?: string
          resource_type?: string | null
          summary?: string | null
          transaction_id?: string | null
        }
        Relationships: []
      }
    }
    Views: {
      [_ in never]: never
    }
    Functions: {
      earth: { Args: never; Returns: number }
    }
    Enums: {
      [_ in never]: never
    }
    CompositeTypes: {
      [_ in never]: never
    }
  }
}

type DatabaseWithoutInternals = Omit<Database, "__InternalSupabase">

type DefaultSchema = DatabaseWithoutInternals[Extract<keyof Database, "public">]

export type Tables<
  DefaultSchemaTableNameOrOptions extends
    | keyof (DefaultSchema["Tables"] & DefaultSchema["Views"])
    | { schema: keyof DatabaseWithoutInternals },
  TableName extends DefaultSchemaTableNameOrOptions extends {
    schema: keyof DatabaseWithoutInternals
  }
    ? keyof (DatabaseWithoutInternals[DefaultSchemaTableNameOrOptions["schema"]]["Tables"] &
        DatabaseWithoutInternals[DefaultSchemaTableNameOrOptions["schema"]]["Views"])
    : never = never,
> = DefaultSchemaTableNameOrOptions extends {
  schema: keyof DatabaseWithoutInternals
}
  ? (DatabaseWithoutInternals[DefaultSchemaTableNameOrOptions["schema"]]["Tables"] &
      DatabaseWithoutInternals[DefaultSchemaTableNameOrOptions["schema"]]["Views"])[TableName] extends {
      Row: infer R
    }
    ? R
    : never
  : DefaultSchemaTableNameOrOptions extends keyof (DefaultSchema["Tables"] &
        DefaultSchema["Views"])
    ? (DefaultSchema["Tables"] &
        DefaultSchema["Views"])[DefaultSchemaTableNameOrOptions] extends {
        Row: infer R
      }
      ? R
      : never
    : never

export type TablesInsert<
  DefaultSchemaTableNameOrOptions extends
    | keyof DefaultSchema["Tables"]
    | { schema: keyof DatabaseWithoutInternals },
  TableName extends DefaultSchemaTableNameOrOptions extends {
    schema: keyof DatabaseWithoutInternals
  }
    ? keyof DatabaseWithoutInternals[DefaultSchemaTableNameOrOptions["schema"]]["Tables"]
    : never = never,
> = DefaultSchemaTableNameOrOptions extends {
  schema: keyof DatabaseWithoutInternals
}
  ? DatabaseWithoutInternals[DefaultSchemaTableNameOrOptions["schema"]]["Tables"][TableName] extends {
      Insert: infer I
    }
    ? I
    : never
  : DefaultSchemaTableNameOrOptions extends keyof DefaultSchema["Tables"]
    ? DefaultSchema["Tables"][DefaultSchemaTableNameOrOptions] extends {
        Insert: infer I
      }
      ? I
      : never
    : never

export type TablesUpdate<
  DefaultSchemaTableNameOrOptions extends
    | keyof DefaultSchema["Tables"]
    | { schema: keyof DatabaseWithoutInternals },
  TableName extends DefaultSchemaTableNameOrOptions extends {
    schema: keyof DatabaseWithoutInternals
  }
    ? keyof DatabaseWithoutInternals[DefaultSchemaTableNameOrOptions["schema"]]["Tables"]
    : never = never,
> = DefaultSchemaTableNameOrOptions extends {
  schema: keyof DatabaseWithoutInternals
}
  ? DatabaseWithoutInternals[DefaultSchemaTableNameOrOptions["schema"]]["Tables"][TableName] extends {
      Update: infer U
    }
    ? U
    : never
  : DefaultSchemaTableNameOrOptions extends keyof DefaultSchema["Tables"]
    ? DefaultSchema["Tables"][DefaultSchemaTableNameOrOptions] extends {
        Update: infer U
      }
      ? U
      : never
    : never

export type Enums<
  DefaultSchemaEnumNameOrOptions extends
    | keyof DefaultSchema["Enums"]
    | { schema: keyof DatabaseWithoutInternals },
  EnumName extends DefaultSchemaEnumNameOrOptions extends {
    schema: keyof DatabaseWithoutInternals
  }
    ? keyof DatabaseWithoutInternals[DefaultSchemaEnumNameOrOptions["schema"]]["Enums"]
    : never = never,
> = DefaultSchemaEnumNameOrOptions extends {
  schema: keyof DatabaseWithoutInternals
}
  ? DatabaseWithoutInternals[DefaultSchemaEnumNameOrOptions["schema"]]["Enums"][EnumName]
  : DefaultSchemaEnumNameOrOptions extends keyof DefaultSchema["Enums"]
    ? DefaultSchema["Enums"][DefaultSchemaEnumNameOrOptions]
    : never

export type CompositeTypes<
  PublicCompositeTypeNameOrOptions extends
    | keyof DefaultSchema["CompositeTypes"]
    | { schema: keyof DatabaseWithoutInternals },
  CompositeTypeName extends PublicCompositeTypeNameOrOptions extends {
    schema: keyof DatabaseWithoutInternals
  }
    ? keyof DatabaseWithoutInternals[PublicCompositeTypeNameOrOptions["schema"]]["CompositeTypes"]
    : never = never,
> = PublicCompositeTypeNameOrOptions extends {
  schema: keyof DatabaseWithoutInternals
}
  ? DatabaseWithoutInternals[PublicCompositeTypeNameOrOptions["schema"]]["CompositeTypes"][CompositeTypeName]
  : PublicCompositeTypeNameOrOptions extends keyof DefaultSchema["CompositeTypes"]
    ? DefaultSchema["CompositeTypes"][PublicCompositeTypeNameOrOptions]
    : never

export const Constants = {
  graphql_public: {
    Enums: {},
  },
  public: {
    Enums: {},
  },
} as const

