package rbac

import (
	"slices"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
)

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
)

var RolePermissions = map[domain.Role][]Permission{
	domain.Admin: {
		UsersView, UsersCreate, UsersEdit, UsersDelete,
		ProductsView, ProductsManage,
		CategoriesView, CategoriesManage,
		InventoryView, InventoryManage,
		OrdersView, OrdersCreate, OrdersManage,
	},
	domain.Manager: {
		UsersView, UsersCreate,
		ProductsView, ProductsManage,
		CategoriesView, CategoriesManage,
		InventoryView, InventoryManage,
		OrdersView, OrdersCreate, OrdersManage,
	},
	domain.Staff: {
		ProductsView,
		CategoriesView,
		InventoryView,
		OrdersView, OrdersCreate,
	},
}

func HasPermission(role domain.Role, perm Permission) bool {
	return slices.Contains(RolePermissions[role], perm)
}
