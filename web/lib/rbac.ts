export const PERMISSIONS = {
  USERS_VIEW: 'users:view',
  USERS_CREATE: 'users:create',
  USERS_EDIT: 'users:edit',
  USERS_DELETE: 'users:delete',
  PRODUCTS_VIEW: 'products:view',
  PRODUCTS_MANAGE: 'products:manage',
  CATEGORIES_VIEW: 'categories:view',
  CATEGORIES_MANAGE: 'categories:manage',
  INVENTORY_VIEW: 'inventory:view',
  INVENTORY_MANAGE: 'inventory:manage',
  ORDERS_VIEW: 'orders:view',
  ORDERS_CREATE: 'orders:create',
  ORDERS_MANAGE: 'orders:manage',

  LOCATIONS_MANAGE: 'locations:manage',
  ROLES_MANAGE: 'roles:manage',

  // Admin tooling — separate from users:delete so the bits can be granted
  // independently (e.g. ops manager gets audit + notifications but not
  // raw webhook replay).
  WEBHOOKS_VIEW: 'webhooks:view',
  WEBHOOKS_MANAGE: 'webhooks:manage',
  NOTIFICATIONS_VIEW: 'notifications:view',
  NOTIFICATIONS_MANAGE: 'notifications:manage',
  AUDIT_VIEW: 'audit:view',
} as const

export type Permission = typeof PERMISSIONS[keyof typeof PERMISSIONS]

export const ROLE_PERMISSIONS: Record<string, Permission[]> = {
  admin: Object.values(PERMISSIONS) as Permission[],
  manager: [
    PERMISSIONS.USERS_VIEW, PERMISSIONS.USERS_CREATE,
    PERMISSIONS.PRODUCTS_VIEW, PERMISSIONS.PRODUCTS_MANAGE,
    PERMISSIONS.CATEGORIES_VIEW, PERMISSIONS.CATEGORIES_MANAGE,
    PERMISSIONS.INVENTORY_VIEW, PERMISSIONS.INVENTORY_MANAGE,
    PERMISSIONS.ORDERS_VIEW, PERMISSIONS.ORDERS_CREATE, PERMISSIONS.ORDERS_MANAGE,
    PERMISSIONS.LOCATIONS_MANAGE,
    // Ops triage tools — read-only webhooks + manage notifications + read audit.
    PERMISSIONS.WEBHOOKS_VIEW,
    PERMISSIONS.NOTIFICATIONS_VIEW, PERMISSIONS.NOTIFICATIONS_MANAGE,
    PERMISSIONS.AUDIT_VIEW,
  ],
  staff: [
    PERMISSIONS.PRODUCTS_VIEW,
    PERMISSIONS.CATEGORIES_VIEW,
    PERMISSIONS.INVENTORY_VIEW,
    PERMISSIONS.ORDERS_VIEW, PERMISSIONS.ORDERS_CREATE,
  ],
}

export const hasPermission = (role: string | undefined, perm: Permission): boolean =>
  ROLE_PERMISSIONS[role ?? '']?.includes(perm) ?? false
