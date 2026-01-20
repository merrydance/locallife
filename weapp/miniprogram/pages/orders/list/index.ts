import {
  getOrders,
  cancelOrder,
  OrderStatus,
  getOrderDetail,
  OrderResponse,
  OrderType,
  ListOrdersParams,
} from "../../../api/order";
import { logger } from "../../../utils/logger";
import { OrderCardAdapter } from "../../../adapters/order-card";
import type { OrderCardViewModel } from "../../../adapters/order-card";
import CartService from "../../../services/cart";
import { OrderAdapter } from "../../../adapters/order";

// 不同订单类型的状态筛选选项
const STATUS_TABS_MAP: Record<
  string,
  Array<{ label: string; value: OrderStatus | "" }>
> = {
  takeout: [
    { label: "全部", value: "" },
    { label: "待支付", value: "pending" },
    { label: "待接单", value: "paid" },
    { label: "制作中", value: "preparing" },
    { label: "已接单", value: "courier_accepted" },
    { label: "已取餐", value: "picked" },
    { label: "配送中", value: "delivering" },
    { label: "待确认", value: "rider_delivered" },
    { label: "已送达", value: "user_delivered" },
    { label: "已完成", value: "completed" },
    { label: "已取消", value: "cancelled" },
  ],
  dine_in: [
    { label: "全部", value: "" },
    { label: "待支付", value: "pending" },
    { label: "待确认", value: "paid" },
    { label: "制作中", value: "preparing" },
    { label: "已完成", value: "completed" },
    { label: "已取消", value: "cancelled" },
  ],
  reservation: [
    { label: "全部", value: "" },
    { label: "待支付", value: "pending" },
    { label: "待确认", value: "paid" },
    { label: "制作中", value: "preparing" },
    { label: "已完成", value: "completed" },
    { label: "已取消", value: "cancelled" },
  ],
  takeaway: [
    { label: "全部", value: "" },
    { label: "待支付", value: "pending" },
    { label: "待接单", value: "paid" },
    { label: "制作中", value: "preparing" },
    { label: "已取餐", value: "picked" },
    { label: "已完成", value: "completed" },
    { label: "已送达", value: "user_delivered" },
    { label: "已取消", value: "cancelled" },
  ],
  default: [
    { label: "全部", value: "" },
    { label: "待支付", value: "pending" },
    { label: "已完成", value: "completed" },
    { label: "已取消", value: "cancelled" },
  ],
};

// 取消原因选项
const CANCEL_REASONS = [
  "不想要了",
  "信息填写错误",
  "商品价格较贵",
  "配送时间太长",
  "其他原因",
];

type OrderTypeFilter = OrderType | "";

const VALID_ORDER_TYPES: OrderType[] = ["takeout", "reservation", "dine_in", "takeaway"];

const normalizeOrderType = (value?: string): OrderTypeFilter => {
  if (value && VALID_ORDER_TYPES.includes(value as OrderType)) {
    return value as OrderType;
  }
  return "";
};

const getDatasetId = (event: WechatMiniprogram.BaseEvent): number | null => {
  const dataset = event.currentTarget.dataset as { id?: string | number };
  const id = dataset.id;
  const numericId = typeof id === "number" ? id : Number(id);
  return Number.isFinite(numericId) ? numericId : null;
};

const isOrderResponse = (value: unknown): value is OrderResponse => {
  return !!value && typeof value === "object" && "id" in value && "order_no" in value;
};

Page({
  data: {
    orders: [] as OrderCardViewModel[],
    navBarHeight: 88,
    loading: false,
    page: 1,
    pageSize: 10,
    hasMore: true,
    statusTabs: STATUS_TABS_MAP.default,
    currentStatus: "" as OrderStatus | "",
    orderType: "" as OrderTypeFilter,
    pageTitle: "我的订单",
  },

  onLoad(options: { order_type?: string }) {
    const orderType = normalizeOrderType(options?.order_type);
    const titleMap: Record<string, string> = {
      takeout: "外卖订单",
      reservation: "预订订单",
      dine_in: "堂食订单",
      takeaway: "自取订单",
    };
    this.setData({
      orderType,
      pageTitle: titleMap[orderType] || "我的订单",
      statusTabs: STATUS_TABS_MAP[orderType] || STATUS_TABS_MAP.default,
    });
    this.loadOrders(true);
  },

  onShow() {
    // 返回时刷新列表
    if (this.data.orders.length > 0) {
      this.loadOrders(true);
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight });
  },

  onReachBottom() {
    if (this.data.hasMore && !this.data.loading) {
      this.setData({ page: this.data.page + 1 });
      this.loadOrders(false);
    }
  },

  async loadOrders(reset = false) {
    if (this.data.loading) return;
    this.setData({ loading: true });

    if (reset) {
      this.setData({ page: 1, orders: [], hasMore: true });
    }

    try {
      const { currentStatus, page, pageSize, orderType } = this.data;
      // API Call with status filter
      const params: ListOrdersParams = {
        page_id: page,
        page_size: pageSize,
        ...(currentStatus ? { status: currentStatus } : {}),
        ...(orderType ? { order_type: orderType } : {}),
      };
      const result = await getOrders(params);

      // 兼容不同返回结构：数组 / {orders} / {list} / {items} / {data: {...}}
      const unwrap = (payload: unknown): unknown[] => {
        if (Array.isArray(payload)) return payload;
        if (payload && typeof payload === "object") {
          const record = payload as Record<string, unknown>;
          if (Array.isArray(record.orders)) return record.orders;
          if (Array.isArray(record.list)) return record.list;
          if (Array.isArray(record.items)) return record.items;
          if (record.data) return unwrap(record.data);
        }
        return [];
      };

      const orderDTOsRaw = unwrap(result);

      // 过滤掉空值或非对象；并在 map 阶段做单条 try/catch，避免坏数据导致整页崩溃
      const orderDTOs = orderDTOsRaw
        .filter(isOrderResponse)
        .map((item) => {
          try {
            return OrderCardAdapter.toCardViewModel(item);
          } catch (err) {
            logger.error("Order map failed", { err, item }, "Orders.List");
            return null;
          }
        })
        .filter(Boolean) as OrderCardViewModel[];

      // Sort by priority (preparing > delivering > completed)
      const sortedOrders = OrderCardAdapter.sortByPriority(orderDTOs);

      const orders = reset
        ? sortedOrders
        : [...this.data.orders, ...sortedOrders];

      const totalCount = typeof result.total_count === 'number' ? result.total_count : orders.length;

      this.setData({
        orders,
        hasMore: page * pageSize < totalCount,
      });
    } catch (error) {
      logger.error("Load orders failed:", error, "List");
      wx.showToast({ title: "加载失败", icon: "error" });
    } finally {
      this.setData({ loading: false });
    }
  },

  // 状态筛选切换
  onStatusChange(e: WechatMiniprogram.CustomEvent<{ value: OrderStatus | "" }>) {
    const status = e.detail.value || "";
    if (status === this.data.currentStatus) return;
    this.setData({ currentStatus: status });
    this.loadOrders(true);
  },

  onViewOrder(e: WechatMiniprogram.BaseEvent) {
    const id = getDatasetId(e);
    if (!id) return;
    wx.navigateTo({ url: `/pages/orders/detail/index?id=${id}` });
  },

  // 快速取消订单
  onCancelOrder(e: WechatMiniprogram.BaseEvent) {
    const id = getDatasetId(e);
    if (!id) return;

    wx.showActionSheet({
      itemList: CANCEL_REASONS,
      success: async (res) => {
        const reason = CANCEL_REASONS[res.tapIndex];
        await this.doCancelOrder(Number(id), reason);
      },
    });
  },

  async doCancelOrder(orderId: number, reason: string) {
    wx.showLoading({ title: "取消中..." });
    try {
      await cancelOrder(orderId, { reason });
      wx.hideLoading();
      wx.showToast({ title: "已取消", icon: "success" });
      setTimeout(() => this.loadOrders(true), 1500);
    } catch (error) {
      wx.hideLoading();
      logger.error("取消订单失败", error, "List.doCancelOrder");
      wx.showToast({ title: "取消失败", icon: "error" });
    }
  },

  // 去支付
  onPayOrder(e: WechatMiniprogram.BaseEvent) {
    const id = getDatasetId(e);
    if (!id) {
      wx.showToast({ title: "订单信息缺失", icon: "none" });
      return;
    }
    wx.navigateTo({
      url: `/pages/user_center/payment-detail/index?orderId=${id}`,
    });
  },

  onReorder(e: WechatMiniprogram.BaseEvent) {
    const orderId = getDatasetId(e);
    if (!orderId) {
      wx.showToast({ title: "订单信息缺失", icon: "none" });
      return;
    }

    wx.showLoading({ title: "再次购买中..." });
    (async () => {
      try {
        const orderDTO = await getOrderDetail(orderId);
        const orderDetail = OrderAdapter.toDetailViewModel(orderDTO);

        const orderType: OrderType = orderDetail.type || "takeout";
        const cartContext: {
          orderType: OrderType
          tableId?: number
          reservationId?: number
        } = { orderType }

        // 根据订单类型只传递对应的上下文，避免不相关字段干扰
        if (orderType === 'dine_in' && orderDetail.tableId) {
          cartContext.tableId = orderDetail.tableId
        }
        if (orderType === 'reservation' && orderDetail.reservationId) {
          cartContext.reservationId = orderDetail.reservationId
        }

        await CartService.loadCart(orderDetail.merchantId, cartContext)

        // 直接累加到当前购物车，避免覆盖已有商品
        const addResults = await Promise.all(
          orderDetail.items.map((item) =>
            CartService.addItem({
              merchantId: orderDetail.merchantId,
              dishId: item.dishId,
              comboId: item.comboId,
              quantity: item.quantity,
            }),
          ),
        );

        if (addResults.some((ok) => !ok)) {
          wx.hideLoading();
          wx.showToast({ title: "部分商品添加失败", icon: "none" });
          return;
        }

        wx.hideLoading();
        wx.showToast({ title: "已加入购物车", icon: "success" });
        setTimeout(() => {
          wx.navigateTo({ url: "/pages/takeout/cart/index" });
        }, 300);
      } catch (error) {
        wx.hideLoading();
        logger.error("再次购买失败", error, "List.onReorder");
        wx.showToast({ title: "操作失败", icon: "error" });
      }
    })();
  },
});
