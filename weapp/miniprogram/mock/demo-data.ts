/**
 * Demo Mock 数据
 * 以 平乡县牛门宴餐饮服务有限公司 为示例
 * 位置：宁晋县 (与物流追踪页面一致)
 */

// ==================== 本地菜品图片资源 ====================
// 图片来自 miniprogram/assets/demo 目录
const DEMO_IMAGES = {
  dish1: '/assets/demo/ScreenShot_2025-11-18_122325_450.png',
  dish2: '/assets/demo/ScreenShot_2025-11-18_122336_267.png',
  dish3: '/assets/demo/ScreenShot_2025-11-18_122346_885.png',
  dish4: '/assets/demo/ScreenShot_2025-11-18_122356_806.png',
  dish5: '/assets/demo/ScreenShot_2025-11-18_122405_845.png',
  dish6: '/assets/demo/ScreenShot_2025-11-18_122429_499.png',
  dish7: '/assets/demo/ScreenShot_2025-11-18_122440_526.png',
  dish8: '/assets/demo/微信图片_2025-10-12_124830_784.png'
}

// ==================== 商家信息 ====================

export interface MockMerchant {
  id: string
  name: string
  short_name: string
  image_url: string
  cover_image: string
  address: string
  phone: string
  latitude: number
  longitude: number
  rating: number
  review_count: number
  business_hours: {
    open: string
    close: string
  }
  tags: string[]
  distance_meters: number
  delivery_fee: number
  delivery_time_minutes: number
  discount_threshold: number
  biz_status: 'OPEN' | 'CLOSED'
  license_no: string
  description: string
}

export interface MockCategory {
  id: string
  name: string
  sort: number
}

export interface MockDish {
  id: string
  name: string
  image_url: string
  images: string[]
  price: number
  original_price: number
  description: string
  category_id: string
  category_name: string
  merchant_id: string
  merchant_name: string
  merchant_short_name: string
  month_sales: number
  rating: number
  tags: string[]
  attributes: string[]
  spicy_level: number
  is_premade: boolean
  spec_groups: {
    id: string,
    name: string,
    specs: { id: string; name: string; price_diff: number }[]
  }[]
}

export interface MockReview {
  id: string
  user_name: string
  rating: number
  content: string
  images: string[]
  created_at: string
  dish_name?: string
}

// ==================== 牛门宴餐饮 - 商家数据 ====================

export const NIUMENYAN_MERCHANT: MockMerchant = {
  id: 'merchant_niumenyan_001',
  name: '平乡县牛门宴餐饮服务有限公司',
  short_name: '牛门宴',
  image_url: DEMO_IMAGES.dish1,
  cover_image: DEMO_IMAGES.dish1,
  address: '河北省邢台市平乡县平乡镇光明路北段路西',
  phone: '0319-7888888',
  latitude: 37.063889,  // 平乡县大致位置
  longitude: 115.030556,
  rating: 48, // 4.8分 (后端格式)
  review_count: 1256,
  business_hours: {
    open: '10:30',
    close: '21:30'
  },
  tags: ['河北菜', '本地特色', '非预制', '牛肉专营'],
  distance_meters: 1200,
  delivery_fee: 300, // 3元
  delivery_time_minutes: 35,
  discount_threshold: 5000, // 满50减5
  biz_status: 'OPEN',
  license_no: '91130532MACP8QGQ7P',
  description: '牛门宴专注牛肉美食，精选优质牛肉，传统工艺烹制，味道正宗。主打牛肉火锅、牛肉面、卤牛肉等特色菜品。'
}

// ==================== 牛门宴 - 分类数据 ====================

export const NIUMENYAN_CATEGORIES: MockCategory[] = [
  { id: 'cat_hot', name: '热销', sort: 0 },
  { id: 'cat_beef', name: '招牌牛肉', sort: 1 },
  { id: 'cat_hotpot', name: '牛肉火锅', sort: 2 },
  { id: 'cat_noodle', name: '牛肉面食', sort: 3 },
  { id: 'cat_cold', name: '凉菜小食', sort: 4 },
  { id: 'cat_drink', name: '饮品', sort: 5 }
]

// ==================== 牛门宴 - 菜品数据 ====================

export const NIUMENYAN_DISHES: MockDish[] = [
  // 热销
  {
    id: 'dish_001',
    name: '秘制卤牛肉',
    image_url: DEMO_IMAGES.dish1,
    images: [
      DEMO_IMAGES.dish1,
      DEMO_IMAGES.dish2
    ],
    price: 5800, // 58元
    original_price: 6800,
    description: '精选优质牛腱子肉，秘制卤汁浸泡12小时，肉质紧实入味，切片后肉香四溢。每日限量供应，售完即止。',
    category_id: 'cat_hot',
    category_name: '热销',
    merchant_id: NIUMENYAN_MERCHANT.id,
    merchant_name: NIUMENYAN_MERCHANT.name,
    merchant_short_name: NIUMENYAN_MERCHANT.short_name,
    month_sales: 856,
    rating: 49, // 4.9分
    tags: ['招牌', '人气Top1'],
    attributes: ['精选牛腱', '12小时卤制', '限量供应'],
    spicy_level: 0,
    is_premade: false,
    spec_groups: [
      {
        id: 'spec_weight',
        name: '份量',
        specs: [
          { id: 'spec_weight_half', name: '半斤装', price_diff: 0 },
          { id: 'spec_weight_full', name: '一斤装', price_diff: 4800 }
        ]
      }
    ]
  },
  {
    id: 'dish_002',
    name: '红烧牛腩',
    image_url: DEMO_IMAGES.dish2,
    images: [
      DEMO_IMAGES.dish2,
      DEMO_IMAGES.dish1
    ],
    price: 4800,
    original_price: 5800,
    description: '牛腩肉配以土豆、胡萝卜，大火收汁，软烂入味，汤汁浓郁下饭。',
    category_id: 'cat_hot',
    category_name: '热销',
    merchant_id: NIUMENYAN_MERCHANT.id,
    merchant_name: NIUMENYAN_MERCHANT.name,
    merchant_short_name: NIUMENYAN_MERCHANT.short_name,
    month_sales: 623,
    rating: 48,
    tags: ['下饭神器'],
    attributes: ['肥瘦相间', '软烂入味'],
    spicy_level: 1,
    is_premade: false,
    spec_groups: [
      {
        id: 'spec_size',
        name: '份量',
        specs: [
          { id: 'spec_size_small', name: '小份', price_diff: 0 },
          { id: 'spec_size_large', name: '大份', price_diff: 1500 }
        ]
      },
      {
        id: 'spec_spicy',
        name: '口味',
        specs: [
          { id: 'spec_spicy_none', name: '不辣', price_diff: 0 },
          { id: 'spec_spicy_little', name: '微辣', price_diff: 0 },
          { id: 'spec_spicy_medium', name: '中辣', price_diff: 0 }
        ]
      }
    ]
  },
  // 招牌牛肉
  {
    id: 'dish_003',
    name: '酱牛肉拼盘',
    image_url: DEMO_IMAGES.dish3,
    images: [DEMO_IMAGES.dish3],
    price: 8800,
    original_price: 10800,
    description: '包含秘制卤牛肉、五香牛筋、酱牛肚三种，一盘尝遍牛门宴招牌。适合2-3人分享。',
    category_id: 'cat_beef',
    category_name: '招牌牛肉',
    merchant_id: NIUMENYAN_MERCHANT.id,
    merchant_name: NIUMENYAN_MERCHANT.name,
    merchant_short_name: NIUMENYAN_MERCHANT.short_name,
    month_sales: 312,
    rating: 49,
    tags: ['拼盘', '适合分享'],
    attributes: ['三种口味', '2-3人份'],
    spicy_level: 0,
    is_premade: false,
    spec_groups: []
  },
  {
    id: 'dish_004',
    name: '葱爆牛肉',
    image_url: DEMO_IMAGES.dish4,
    images: [DEMO_IMAGES.dish4],
    price: 4200,
    original_price: 4200,
    description: '新鲜牛里脊配大葱爆炒，鲜嫩多汁，葱香四溢。',
    category_id: 'cat_beef',
    category_name: '招牌牛肉',
    merchant_id: NIUMENYAN_MERCHANT.id,
    merchant_name: NIUMENYAN_MERCHANT.name,
    merchant_short_name: NIUMENYAN_MERCHANT.short_name,
    month_sales: 445,
    rating: 47,
    tags: [],
    attributes: ['牛里脊', '鲜嫩多汁'],
    spicy_level: 0,
    is_premade: false,
    spec_groups: []
  },
  // 牛肉火锅
  {
    id: 'dish_005',
    name: '牛门宴招牌火锅（2-3人）',
    image_url: DEMO_IMAGES.dish5,
    images: [DEMO_IMAGES.dish5],
    price: 16800,
    original_price: 19800,
    description: '精选牛骨熬制8小时高汤底，配送牛肉片、牛肚、牛筋、时蔬拼盘。锅底可选清汤/麻辣。',
    category_id: 'cat_hotpot',
    category_name: '牛肉火锅',
    merchant_id: NIUMENYAN_MERCHANT.id,
    merchant_name: NIUMENYAN_MERCHANT.name,
    merchant_short_name: NIUMENYAN_MERCHANT.short_name,
    month_sales: 198,
    rating: 50,
    tags: ['套餐', '推荐'],
    attributes: ['8小时牛骨汤', '含配菜', '2-3人份'],
    spicy_level: 2,
    is_premade: false,
    spec_groups: [
      {
        id: 'spec_base',
        name: '锅底',
        specs: [
          { id: 'spec_base_clear', name: '清汤锅底', price_diff: 0 },
          { id: 'spec_base_spicy', name: '麻辣锅底', price_diff: 500 },
          { id: 'spec_base_yuanyang', name: '鸳鸯锅底', price_diff: 1000 }
        ]
      }
    ]
  },
  // 牛肉面食
  {
    id: 'dish_006',
    name: '红烧牛肉面',
    image_url: DEMO_IMAGES.dish6,
    images: [DEMO_IMAGES.dish6],
    price: 2200,
    original_price: 2200,
    description: '手工拉面配红烧牛肉块，汤头浓郁，面条劲道。',
    category_id: 'cat_noodle',
    category_name: '牛肉面食',
    merchant_id: NIUMENYAN_MERCHANT.id,
    merchant_name: NIUMENYAN_MERCHANT.name,
    merchant_short_name: NIUMENYAN_MERCHANT.short_name,
    month_sales: 1023,
    rating: 48,
    tags: ['人气王'],
    attributes: ['手工拉面', '大块牛肉'],
    spicy_level: 1,
    is_premade: false,
    spec_groups: [
      {
        id: 'spec_noodle',
        name: '面条',
        specs: [
          { id: 'spec_noodle_thin', name: '细面', price_diff: 0 },
          { id: 'spec_noodle_wide', name: '宽面', price_diff: 0 },
          { id: 'spec_noodle_knife', name: '刀削面', price_diff: 200 }
        ]
      },
      {
        id: 'spec_extra',
        name: '加料',
        specs: [
          { id: 'spec_extra_none', name: '不加料', price_diff: 0 },
          { id: 'spec_extra_egg', name: '加煎蛋', price_diff: 300 },
          { id: 'spec_extra_meat', name: '加肉', price_diff: 800 }
        ]
      }
    ]
  },
  {
    id: 'dish_007',
    name: '牛杂面',
    image_url: DEMO_IMAGES.dish7,
    images: [DEMO_IMAGES.dish7],
    price: 2500,
    original_price: 2500,
    description: '牛肚、牛筋、牛肠三样牛杂，配清汤面，鲜而不腻。',
    category_id: 'cat_noodle',
    category_name: '牛肉面食',
    merchant_id: NIUMENYAN_MERCHANT.id,
    merchant_name: NIUMENYAN_MERCHANT.name,
    merchant_short_name: NIUMENYAN_MERCHANT.short_name,
    month_sales: 567,
    rating: 47,
    tags: [],
    attributes: ['三种牛杂', '清汤底'],
    spicy_level: 0,
    is_premade: false,
    spec_groups: []
  },
  // 凉菜小食
  {
    id: 'dish_008',
    name: '凉拌牛肚丝',
    image_url: DEMO_IMAGES.dish8,
    images: [DEMO_IMAGES.dish8],
    price: 2800,
    original_price: 2800,
    description: '爽脆牛肚切丝，配以黄瓜丝、香菜，麻辣鲜香开胃。',
    category_id: 'cat_cold',
    category_name: '凉菜小食',
    merchant_id: NIUMENYAN_MERCHANT.id,
    merchant_name: NIUMENYAN_MERCHANT.name,
    merchant_short_name: NIUMENYAN_MERCHANT.short_name,
    month_sales: 234,
    rating: 46,
    tags: ['开胃'],
    attributes: ['爽脆', '麻辣'],
    spicy_level: 2,
    is_premade: false,
    spec_groups: []
  },
  {
    id: 'dish_009',
    name: '夫妻肺片',
    image_url: DEMO_IMAGES.dish3,
    images: [DEMO_IMAGES.dish3],
    price: 3200,
    original_price: 3200,
    description: '经典川味凉菜，牛肉、牛肚、牛舌薄片，红油麻辣香。',
    category_id: 'cat_cold',
    category_name: '凉菜小食',
    merchant_id: NIUMENYAN_MERCHANT.id,
    merchant_name: NIUMENYAN_MERCHANT.name,
    merchant_short_name: NIUMENYAN_MERCHANT.short_name,
    month_sales: 189,
    rating: 48,
    tags: ['川味'],
    attributes: ['三种牛杂', '红油香辣'],
    spicy_level: 3,
    is_premade: false,
    spec_groups: []
  },
  // 饮品
  {
    id: 'dish_010',
    name: '酸梅汤',
    image_url: DEMO_IMAGES.dish4,
    images: [DEMO_IMAGES.dish4],
    price: 800,
    original_price: 800,
    description: '自制酸梅汤，解腻消暑，冰镇更佳。',
    category_id: 'cat_drink',
    category_name: '饮品',
    merchant_id: NIUMENYAN_MERCHANT.id,
    merchant_name: NIUMENYAN_MERCHANT.name,
    merchant_short_name: NIUMENYAN_MERCHANT.short_name,
    month_sales: 456,
    rating: 45,
    tags: ['解腻'],
    attributes: ['自制', '冰镇'],
    spicy_level: 0,
    is_premade: false,
    spec_groups: [
      {
        id: 'spec_temp',
        name: '温度',
        specs: [
          { id: 'spec_temp_ice', name: '冰', price_diff: 0 },
          { id: 'spec_temp_normal', name: '常温', price_diff: 0 }
        ]
      }
    ]
  },
  {
    id: 'dish_011',
    name: '王老吉',
    image_url: DEMO_IMAGES.dish5,
    images: [DEMO_IMAGES.dish5],
    price: 600,
    original_price: 600,
    description: '罐装王老吉，怕上火喝王老吉。',
    category_id: 'cat_drink',
    category_name: '饮品',
    merchant_id: NIUMENYAN_MERCHANT.id,
    merchant_name: NIUMENYAN_MERCHANT.name,
    merchant_short_name: NIUMENYAN_MERCHANT.short_name,
    month_sales: 234,
    rating: 50,
    tags: [],
    attributes: [],
    spicy_level: 0,
    is_premade: false,
    spec_groups: []
  }
]

// ==================== 评价数据 ====================

export const NIUMENYAN_REVIEWS: MockReview[] = [
  {
    id: 'review_001',
    user_name: '张**',
    rating: 5,
    content: '卤牛肉超级好吃！肉质紧实入味，切得很薄，配白酒绝了。下次还来！',
    images: [DEMO_IMAGES.dish1],
    created_at: '2025-12-01',
    dish_name: '秘制卤牛肉'
  },
  {
    id: 'review_002',
    user_name: '李**',
    rating: 5,
    content: '牛肉面的牛肉给的很足，面条也很劲道，汤头浓郁。性价比很高！',
    images: [],
    created_at: '2025-11-28',
    dish_name: '红烧牛肉面'
  },
  {
    id: 'review_003',
    user_name: '王**',
    rating: 4,
    content: '火锅味道不错，牛骨汤很鲜。就是配送时间有点长，希望改进。',
    images: [DEMO_IMAGES.dish5],
    created_at: '2025-11-25',
    dish_name: '牛门宴招牌火锅'
  },
  {
    id: 'review_004',
    user_name: '赵**',
    rating: 5,
    content: '每次来宁晋必吃牛门宴，味道一如既往地好，推荐酱牛肉拼盘！',
    images: [],
    created_at: '2025-11-20'
  },
  {
    id: 'review_005',
    user_name: '刘**',
    rating: 5,
    content: '夫妻肺片太下饭了，麻辣鲜香，牛肚很脆。',
    images: [],
    created_at: '2025-11-15',
    dish_name: '夫妻肺片'
  }
]

// ==================== 其他商家数据（用于列表展示） ====================

export const OTHER_MERCHANTS: MockMerchant[] = [
  {
    id: 'merchant_fukela',
    name: '福客来餐厅',
    short_name: '福客来',
    image_url: DEMO_IMAGES.dish2,
    cover_image: DEMO_IMAGES.dish2,
    address: '河北省邢台市宁晋县凤凰路123号',
    phone: '0319-5666666',
    latitude: 37.611038,
    longitude: 114.912825,
    rating: 46,
    review_count: 856,
    business_hours: { open: '10:00', close: '21:00' },
    tags: ['家常菜', '实惠', '快餐'],
    distance_meters: 650,
    delivery_fee: 200,
    delivery_time_minutes: 25,
    discount_threshold: 2000,
    biz_status: 'OPEN',
    license_no: '91130527XXXXX',
    description: '家常菜馆，实惠美味'
  },
  {
    id: 'merchant_feitengxian',
    name: '沸腾鲜火锅',
    short_name: '沸腾鲜',
    image_url: DEMO_IMAGES.dish3,
    cover_image: DEMO_IMAGES.dish3,
    address: '河北省邢台市宁晋县建设路88号',
    phone: '0319-5777777',
    latitude: 37.625,
    longitude: 114.920,
    rating: 47,
    review_count: 523,
    business_hours: { open: '11:00', close: '22:00' },
    tags: ['火锅', '鲜货', '适合聚会'],
    distance_meters: 1200,
    delivery_fee: 500,
    delivery_time_minutes: 40,
    discount_threshold: 8000,
    biz_status: 'OPEN',
    license_no: '91130527XXXXX',
    description: '新鲜食材，沸腾美味'
  },
  {
    id: 'merchant_huaxi',
    name: '华西饭店',
    short_name: '华西',
    image_url: DEMO_IMAGES.dish4,
    cover_image: DEMO_IMAGES.dish4,
    address: '河北省邢台市宁晋县人民路56号',
    phone: '0319-5888888',
    latitude: 37.618,
    longitude: 114.925,
    rating: 48,
    review_count: 712,
    business_hours: { open: '10:00', close: '21:00' },
    tags: ['中餐', '包间', '宴请'],
    distance_meters: 850,
    delivery_fee: 300,
    delivery_time_minutes: 30,
    discount_threshold: 5000,
    biz_status: 'OPEN',
    license_no: '91130527XXXXX',
    description: '品质中餐，宴请首选'
  },
  {
    id: 'merchant_liuyanghe',
    name: '浏阳河湘菜馆',
    short_name: '浏阳河',
    image_url: DEMO_IMAGES.dish5,
    cover_image: DEMO_IMAGES.dish5,
    address: '河北省邢台市宁晋县光明路200号',
    phone: '0319-5999999',
    latitude: 37.620,
    longitude: 114.918,
    rating: 46,
    review_count: 634,
    business_hours: { open: '11:00', close: '22:00' },
    tags: ['湘菜', '辣味', '地道'],
    distance_meters: 980,
    delivery_fee: 400,
    delivery_time_minutes: 35,
    discount_threshold: 3000,
    biz_status: 'OPEN',
    license_no: '91130527XXXXX',
    description: '正宗湘菜，麻辣鲜香'
  }
]

// ==================== 其他商家菜品数据 ====================

export const FUKELA_DISHES: MockDish[] = [
  {
    id: 'dish_fkl_001',
    name: '红烧排骨',
    image_url: DEMO_IMAGES.dish6,
    images: [DEMO_IMAGES.dish6],
    price: 3200,
    original_price: 3800,
    description: '精选猪肋排，红烧入味，肉质软嫩。',
    category_id: 'cat_hot',
    category_name: '热销',
    merchant_id: 'merchant_fukela',
    merchant_name: '福客来餐厅',
    merchant_short_name: '福客来',
    month_sales: 523,
    rating: 47,
    tags: ['人气菜'],
    attributes: ['肉质软嫩'],
    spicy_level: 0,
    is_premade: false,
    spec_groups: []
  },
  {
    id: 'dish_fkl_002',
    name: '西红柿炒鸡蛋',
    image_url: DEMO_IMAGES.dish7,
    images: [DEMO_IMAGES.dish7],
    price: 1800,
    original_price: 1800,
    description: '经典家常菜，酸甜可口。',
    category_id: 'cat_hot',
    category_name: '热销',
    merchant_id: 'merchant_fukela',
    merchant_name: '福客来餐厅',
    merchant_short_name: '福客来',
    month_sales: 892,
    rating: 48,
    tags: [],
    attributes: ['下饭'],
    spicy_level: 0,
    is_premade: false,
    spec_groups: []
  },
  {
    id: 'dish_fkl_003',
    name: '宫保鸡丁',
    image_url: DEMO_IMAGES.dish8,
    images: [DEMO_IMAGES.dish8],
    price: 2800,
    original_price: 2800,
    description: '鸡丁滑嫩，花生酥脆，微辣开胃。',
    category_id: 'cat_hot',
    category_name: '热销',
    merchant_id: 'merchant_fukela',
    merchant_name: '福客来餐厅',
    merchant_short_name: '福客来',
    month_sales: 678,
    rating: 46,
    tags: [],
    attributes: ['微辣', '下饭'],
    spicy_level: 1,
    is_premade: false,
    spec_groups: []
  }
]

export const FEITENGXIAN_DISHES: MockDish[] = [
  {
    id: 'dish_ftx_001',
    name: '鲜牛肉火锅套餐',
    image_url: DEMO_IMAGES.dish1,
    images: [DEMO_IMAGES.dish1],
    price: 12800,
    original_price: 15800,
    description: '新鲜牛肉现切，搭配秘制锅底，2-3人份。',
    category_id: 'cat_hot',
    category_name: '热销',
    merchant_id: 'merchant_feitengxian',
    merchant_name: '沸腾鲜火锅',
    merchant_short_name: '沸腾鲜',
    month_sales: 345,
    rating: 49,
    tags: ['套餐', '推荐'],
    attributes: ['新鲜现切', '2-3人份'],
    spicy_level: 2,
    is_premade: false,
    spec_groups: []
  },
  {
    id: 'dish_ftx_002',
    name: '鲜毛肚',
    image_url: DEMO_IMAGES.dish2,
    images: [DEMO_IMAGES.dish2],
    price: 4800,
    original_price: 4800,
    description: '七上八下，爽脆鲜嫩。',
    category_id: 'cat_hot',
    category_name: '热销',
    merchant_id: 'merchant_feitengxian',
    merchant_name: '沸腾鲜火锅',
    merchant_short_name: '沸腾鲜',
    month_sales: 567,
    rating: 48,
    tags: ['必点'],
    attributes: ['爽脆'],
    spicy_level: 0,
    is_premade: false,
    spec_groups: []
  }
]

export const HUAXI_DISHES: MockDish[] = [
  {
    id: 'dish_hx_001',
    name: '糖醋里脊',
    image_url: DEMO_IMAGES.dish3,
    images: [DEMO_IMAGES.dish3],
    price: 3800,
    original_price: 4200,
    description: '外酥里嫩，酸甜适口，老少皆宜。',
    category_id: 'cat_hot',
    category_name: '热销',
    merchant_id: 'merchant_huaxi',
    merchant_name: '华西饭店',
    merchant_short_name: '华西',
    month_sales: 456,
    rating: 48,
    tags: ['招牌'],
    attributes: ['外酥里嫩', '酸甜'],
    spicy_level: 0,
    is_premade: false,
    spec_groups: []
  },
  {
    id: 'dish_hx_002',
    name: '鱼香肉丝',
    image_url: DEMO_IMAGES.dish4,
    images: [DEMO_IMAGES.dish4],
    price: 2800,
    original_price: 2800,
    description: '咸鲜微酸，肉丝滑嫩。',
    category_id: 'cat_hot',
    category_name: '热销',
    merchant_id: 'merchant_huaxi',
    merchant_name: '华西饭店',
    merchant_short_name: '华西',
    month_sales: 623,
    rating: 47,
    tags: [],
    attributes: ['下饭神器'],
    spicy_level: 1,
    is_premade: false,
    spec_groups: []
  },
  {
    id: 'dish_hx_003',
    name: '清蒸鲈鱼',
    image_url: DEMO_IMAGES.dish5,
    images: [DEMO_IMAGES.dish5],
    price: 5800,
    original_price: 6800,
    description: '鲜活鲈鱼，清蒸保留鲜味，肉质细嫩。',
    category_id: 'cat_hot',
    category_name: '热销',
    merchant_id: 'merchant_huaxi',
    merchant_name: '华西饭店',
    merchant_short_name: '华西',
    month_sales: 234,
    rating: 49,
    tags: ['鲜'],
    attributes: ['鲜活', '清淡'],
    spicy_level: 0,
    is_premade: false,
    spec_groups: []
  }
]

export const LIUYANGHE_DISHES: MockDish[] = [
  {
    id: 'dish_lyh_001',
    name: '剁椒鱼头',
    image_url: DEMO_IMAGES.dish6,
    images: [DEMO_IMAGES.dish6],
    price: 6800,
    original_price: 7800,
    description: '正宗湘味，鱼头肥美，剁椒鲜辣。',
    category_id: 'cat_hot',
    category_name: '热销',
    merchant_id: 'merchant_liuyanghe',
    merchant_name: '浏阳河湘菜馆',
    merchant_short_name: '浏阳河',
    month_sales: 412,
    rating: 48,
    tags: ['招牌', '湘菜'],
    attributes: ['鲜辣', '肥美'],
    spicy_level: 3,
    is_premade: false,
    spec_groups: []
  },
  {
    id: 'dish_lyh_002',
    name: '小炒肉',
    image_url: DEMO_IMAGES.dish7,
    images: [DEMO_IMAGES.dish7],
    price: 3200,
    original_price: 3200,
    description: '农家小炒，香辣下饭。',
    category_id: 'cat_hot',
    category_name: '热销',
    merchant_id: 'merchant_liuyanghe',
    merchant_name: '浏阳河湘菜馆',
    merchant_short_name: '浏阳河',
    month_sales: 756,
    rating: 47,
    tags: ['下饭'],
    attributes: ['香辣'],
    spicy_level: 2,
    is_premade: false,
    spec_groups: []
  },
  {
    id: 'dish_lyh_003',
    name: '干锅花菜',
    image_url: DEMO_IMAGES.dish8,
    images: [DEMO_IMAGES.dish8],
    price: 2200,
    original_price: 2200,
    description: '干锅炒制，香脆可口。',
    category_id: 'cat_hot',
    category_name: '热销',
    merchant_id: 'merchant_liuyanghe',
    merchant_name: '浏阳河湘菜馆',
    merchant_short_name: '浏阳河',
    month_sales: 534,
    rating: 46,
    tags: [],
    attributes: ['香脆'],
    spicy_level: 1,
    is_premade: false,
    spec_groups: []
  }
]

// ==================== 套餐数据 ====================

export interface MockPackage {
  id: string
  name: string
  shop_name: string
  shop_id: string
  image_url: string
  price: number
  original_price: number
  items: string[]
  rating: number
  month_sales: number
  description: string
}

export const MOCK_PACKAGES: MockPackage[] = [
  {
    id: 'pkg_001',
    name: '双人牛肉套餐',
    shop_name: '牛门宴',
    shop_id: NIUMENYAN_MERCHANT.id,
    image_url: DEMO_IMAGES.dish1,
    price: 9800,
    original_price: 12800,
    items: ['秘制卤牛肉半斤', '红烧牛腩', '米饭x2', '酸梅汤x2'],
    rating: 49,
    month_sales: 156,
    description: '牛门宴双人套餐，两人享用刚刚好'
  },
  {
    id: 'pkg_002',
    name: '单人牛肉面套餐',
    shop_name: '牛门宴',
    shop_id: NIUMENYAN_MERCHANT.id,
    image_url: DEMO_IMAGES.dish6,
    price: 2800,
    original_price: 3500,
    items: ['红烧牛肉面', '凉拌牛肚丝（小份）'],
    rating: 48,
    month_sales: 289,
    description: '一人食首选，面+凉菜超满足'
  },
  {
    id: 'pkg_003',
    name: '家庭火锅套餐（4人）',
    shop_name: '牛门宴',
    shop_id: NIUMENYAN_MERCHANT.id,
    image_url: DEMO_IMAGES.dish5,
    price: 25800,
    original_price: 32800,
    items: ['招牌火锅锅底', '肥牛卷', '牛肚', '牛筋', '时蔬拼盘', '酸梅汤x4'],
    rating: 50,
    month_sales: 87,
    description: '家庭聚餐首选，4人份量，超值享用'
  },
  {
    id: 'pkg_004',
    name: '湘味双人餐',
    shop_name: '浏阳河',
    shop_id: 'merchant_002',
    image_url: DEMO_IMAGES.dish7,
    price: 8800,
    original_price: 11800,
    items: ['剁椒鱼头', '小炒肉', '米饭x2'],
    rating: 46,
    month_sales: 112,
    description: '正宗湘菜双人餐'
  }
]

// ==================== 工具函数 ====================

/**
 * 获取所有商家列表（用于外卖首页展示）
 */
export function getAllMerchants(): MockMerchant[] {
  return [NIUMENYAN_MERCHANT, ...OTHER_MERCHANTS]
}

/**
 * 获取商家菜品映射
 */
function getMerchantDishesMap(): Record<string, MockDish[]> {
  return {
    [NIUMENYAN_MERCHANT.id]: NIUMENYAN_DISHES,
    'merchant_fukela': FUKELA_DISHES,
    'merchant_feitengxian': FEITENGXIAN_DISHES,
    'merchant_huaxi': HUAXI_DISHES,
    'merchant_liuyanghe': LIUYANGHE_DISHES
  }
}

/**
 * 获取商家满返规则（针对代取费的返现）
 */
function getMerchantDiscountRule(merchant: MockMerchant): string {
  if (merchant.discount_threshold > 0) {
    const threshold = (merchant.discount_threshold / 100).toFixed(0)
    const discount = Math.floor(merchant.discount_threshold / 10 / 100)
    return `满${threshold}返${discount}元`
  }
  return ''
}

/**
 * 获取所有菜品（Feed格式）
 */
export function getAllDishesForFeed() {
  const allDishes: ReturnType<typeof mapDishToFeed>[] = []
  const merchants = getAllMerchants()
  const dishesMap = getMerchantDishesMap()

  function mapDishToFeed(dish: MockDish, merchant: MockMerchant) {
    return {
      id: dish.id,
      name: dish.name,
      image_url: dish.image_url,
      price: dish.price,
      category_name: dish.category_name,
      merchant_id: dish.merchant_id,
      merchant_name: dish.merchant_name,
      merchant_short_name: dish.merchant_short_name,
      delivery_fee: merchant.delivery_fee,
      discount_threshold: merchant.discount_threshold,
      discount_rule: getMerchantDiscountRule(merchant),
      prep_minutes: 15,
      rating_score: dish.rating,
      recent_sold_count: dish.month_sales,
      tags: dish.tags,
      merchant_latitude: merchant.latitude,
      merchant_longitude: merchant.longitude,
      distance_meters: merchant.distance_meters
    }
  }

  for (const merchant of merchants) {
    const dishes = dishesMap[merchant.id] || []
    for (const dish of dishes) {
      allDishes.push(mapDishToFeed(dish, merchant))
    }
  }

  // 打乱顺序，让首页展示更加多样
  return allDishes.sort(() => Math.random() - 0.5)
}

/**
 * 根据ID获取商家详情
 */
export function getMerchantById(id: string): MockMerchant | undefined {
  return getAllMerchants().find((m) => m.id === id)
}

/**
 * 根据商家ID获取分类
 */
export function getCategoriesByMerchantId(merchantId: string): MockCategory[] {
  if (merchantId === NIUMENYAN_MERCHANT.id) {
    return NIUMENYAN_CATEGORIES
  }
  // 其他商家返回默认分类
  return [
    { id: 'cat_hot', name: '热销', sort: 0 },
    { id: 'cat_main', name: '主食', sort: 1 },
    { id: 'cat_side', name: '小吃', sort: 2 },
    { id: 'cat_drink', name: '饮品', sort: 3 }
  ]
}

/**
 * 根据商家ID获取菜品
 */
export function getDishesByMerchantId(merchantId: string): MockDish[] {
  const dishesMap = getMerchantDishesMap()
  return dishesMap[merchantId] || []
}

/**
 * 根据ID获取菜品详情
 */
export function getDishById(id: string): MockDish | undefined {
  const allDishes = [
    ...NIUMENYAN_DISHES,
    ...FUKELA_DISHES,
    ...FEITENGXIAN_DISHES,
    ...HUAXI_DISHES,
    ...LIUYANGHE_DISHES
  ]
  return allDishes.find((d) => d.id === id)
}

/**
 * 根据商家ID获取评价
 */
export function getReviewsByMerchantId(merchantId: string): MockReview[] {
  if (merchantId === NIUMENYAN_MERCHANT.id) {
    return NIUMENYAN_REVIEWS
  }
  return []
}

/**
 * 获取所有套餐
 */
export function getAllPackages(): MockPackage[] {
  return MOCK_PACKAGES
}

// 导出默认的演示商家（牛门宴）
export const DEFAULT_DEMO_MERCHANT = NIUMENYAN_MERCHANT
