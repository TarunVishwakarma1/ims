package rbac



type Permission string

const (
	UsersView   Permission = "users:view"
	UsersCreate Permission = "users:create"
	UsersEdit   Permission = "users:edit"
	UsersDelete Permission = "users:delete"

	ProductsView   Permission = "products:view"
	ProductsManage Permission = "products:manage"

	CategoriesView   Permission = "categories:view"
	CategoriesManage Permission = "categories:manage"

	InventoryView   Permission = "inventory:view"
	InventoryManage Permission = "inventory:manage"

	OrdersView   Permission = "orders:view"
	OrdersCreate Permission = "orders:create"
	OrdersManage Permission = "orders:manage"

	RolesManage Permission = "roles:manage"

	LocationsManage Permission = "locations:manage"

	// Admin tooling — webhook DLQ, notifications outbox, audit log.
	// Separated from users:delete so manager / custom roles can grant a
	// subset without unlocking user deletion.
	WebhooksView       Permission = "webhooks:view"
	WebhooksManage     Permission = "webhooks:manage"
	NotificationsView  Permission = "notifications:view"
	NotificationsManage Permission = "notifications:manage"
	AuditView          Permission = "audit:view"
)


