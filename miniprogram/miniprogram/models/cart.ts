export interface CartItem {
  merchantId: string
  dishId: string
  dishName: string
  shopName: string
  imageUrl: string
  price: number
  priceDisplay: string
  quantity: number
  specs?: string
}

export interface Cart {
  items: CartItem[]
  totalCount: number
  totalPrice: number
  totalPriceDisplay: string
}
