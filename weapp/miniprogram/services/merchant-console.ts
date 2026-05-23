// Protected tombstone module.
// Merchant console owners were split into task-domain services:
// - services/merchant-dashboard.ts
// - services/merchant-open-status.ts
// - services/merchant-app-bind.ts
// Keep this file only as a protected boundary so future code cannot silently rebuild the old super-service import path.

export {}
