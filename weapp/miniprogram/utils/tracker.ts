import { logger } from './logger'

export enum EventType {
  APP_OPEN = 'APP_OPEN',
  VIEW_DISH = 'VIEW_DISH',
  ADD_CART = 'ADD_CART',
  SUBMIT_ORDER = 'SUBMIT_ORDER',
  PAY_ORDER = 'PAY_ORDER'
}

export enum Role {
  CUSTOMER = 'customer',
  MERCHANT = 'merchant',
  RIDER = 'rider'
}

export class BehaviorTracker {
  private static instance: BehaviorTracker

  private constructor() { }

  public static getInstance(): BehaviorTracker {
    if (!BehaviorTracker.instance) {
      BehaviorTracker.instance = new BehaviorTracker()
    }
    return BehaviorTracker.instance
  }

  /**
     * Log a user behavior event
     * @param eventType The type of event
     * @param targetID The ID of the target object (dish_id, order_id, etc.)
     * @param metaData Additional metadata
     */
  public log(eventType: EventType, targetID: string = '', metaData: Record<string, unknown> = {}) {
    // Get user role from global data or storage
    // For MVP, we assume customer role mostly
    const role = Role.CUSTOMER

    logger.debug('用户行为追踪', { eventType, targetID, metaData, role }, 'BehaviorTracker')

    // Fire and forget - don't await
    // Backend endpoint /rank/behavior not available yet
    /*
        request({
            url: '/rank/behavior',
            method: 'POST',
            loading: false,
            data: {
                role: role,
                event_type: eventType,
                target_id: targetID,
                meta_data: metaData
            }
        }).catch(err => {
            // Silently fail for tracker events
            logger.warn('行为追踪上报失败', err, 'BehaviorTracker')
        })
        */
  }
}

export const tracker = BehaviorTracker.getInstance()
