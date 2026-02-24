export type ConsolePortalKey = "merchant" | "operator" | "platform";

export type ConsolePortal = {
  key: ConsolePortalKey;
  label: string;
  href: string;
  activePrefix: string;
};

const LAST_PORTAL_KEY = "ll_web_last_portal";

const portalRegistry: Record<ConsolePortalKey, ConsolePortal> = {
  merchant: {
    key: "merchant",
    label: "商户控制台",
    href: "/merchant/dashboard",
    activePrefix: "/merchant",
  },
  operator: {
    key: "operator",
    label: "运营商控制台",
    href: "/operator",
    activePrefix: "/operator",
  },
  platform: {
    key: "platform",
    label: "平台控制台",
    href: "/platform",
    activePrefix: "/platform",
  },
};

export function buildConsolePortals(
  roles: string[],
  options: { hasMerchantAccess?: boolean } = {}
): ConsolePortal[] {
  const roleSet = new Set((roles || []).map((role) => role.toLowerCase()));
  const hasMerchant = !!options.hasMerchantAccess || roleSet.has("merchant");
  const hasOperator = roleSet.has("operator");
  const hasAdmin = roleSet.has("admin");
  const hasPlatform = hasAdmin;

  const result: ConsolePortal[] = [];
  if (hasMerchant) result.push(portalRegistry.merchant);
  if (hasOperator) result.push(portalRegistry.operator);
  if (hasPlatform) result.push(portalRegistry.platform);

  return result;
}

export function getLastPortal(): ConsolePortalKey | null {
  if (typeof window === "undefined") return null;
  const value = window.localStorage.getItem(LAST_PORTAL_KEY);
  if (value === "merchant" || value === "operator" || value === "platform") {
    return value;
  }
  return null;
}

export function setLastPortal(portal: ConsolePortalKey) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(LAST_PORTAL_KEY, portal);
}

export function pickLandingPortal(portals: ConsolePortal[]): ConsolePortal | null {
  if (!portals.length) return null;
  const lastPortal = getLastPortal();
  if (lastPortal) {
    const matched = portals.find((portal) => portal.key === lastPortal);
    if (matched) return matched;
  }
  return portals[0];
}
