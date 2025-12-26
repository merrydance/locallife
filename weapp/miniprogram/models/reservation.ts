// Room Model (包间)
export interface Room {
    id: string
    type: 'room'
    name: string
    restaurantId: string
    restaurantName: string
    imageUrl: string
    capacity: {
        min: number
        max: number
    }
    price: number                   // 最低消费(分)
    priceDisplay: string            // ViewModel: "¥800"
    facilities: string[]            // ['投影仪', 'KTV', '独立卫生间']
    availablePeriods: string[]      // ['lunch', 'dinner']
    rating: number
    ratingDisplay: string           // ViewModel: "4.8"
    bookingCount: number
    bookingBadge: string            // ViewModel: "本月预订45次"
    address: string
    distance: string                // ViewModel: "850m"
}

// Restaurant Model (餐厅)
export interface Restaurant {
    id: string
    type: 'restaurant'
    name: string
    imageUrl: string
    cuisineType: string[]           // ['川菜', '粤菜']
    avgPrice: number                // 人均消费(分)
    avgPriceDisplay: string         // ViewModel: "人均¥120"
    rating: number
    ratingDisplay: string           // ViewModel: "4.8"
    reviewCount: number
    reviewBadge: string             // ViewModel: "1200条评价"
    businessHours: {
        open: string                  // "10:00"
        close: string                 // "22:00"
    }
    businessHoursDisplay: string    // ViewModel: "10:00-22:00"
    facilities: string[]            // ['停车场', '包间', 'WiFi']
    availableRooms: number
    availableRoomsBadge: string     // ViewModel: "5个包间可预订"
    address: string
    distance: string                // ViewModel: "1.2km"
    tags: string[]                  // ['适合聚会', '环境好']
}

// Backend DTO (Raw API Response)
export interface RoomDTO {
    id: string
    type: 'room'
    name: string
    restaurant_id: string
    restaurant_name: string
    image_url: string
    capacity: {
        min: number
        max: number
    }
    price: number
    facilities: string[]
    available_periods: string[]
    rating: number
    booking_count: number
    address: string
    distance_meters: number
}

export interface RestaurantDTO {
    id: string
    type: 'restaurant'
    name: string
    image_url: string
    cuisine_type: string[]
    avg_price: number
    rating: number
    review_count: number
    business_hours: {
        open: string
        close: string
    }
    facilities: string[]
    available_rooms: number
    address: string
    distance_meters: number
    tags: string[]
}

export type ReservationItemDTO = RoomDTO | RestaurantDTO
export type ReservationItem = Room | Restaurant
