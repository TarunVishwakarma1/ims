package rbac

import (
	"sync"
)

type PermissionCache struct {
	mu    sync.RWMutex
	cache map[string]map[string]bool // role name → set of "resource:action"
}

// Global cache instance
var Cache = &PermissionCache{
	cache: make(map[string]map[string]bool),
}

// HasPermission checks if a role has the specified permission
func (c *PermissionCache) HasPermission(role string, perm Permission) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	perms, ok := c.cache[role]
	if !ok {
		return false
	}
	return perms[string(perm)]
}

// Load replaces the entire cache with new role permissions
func (c *PermissionCache) Load(rolePerms map[string][]string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	newCache := make(map[string]map[string]bool)
	for role, perms := range rolePerms {
		newCache[role] = make(map[string]bool)
		for _, p := range perms {
			newCache[role][p] = true
		}
	}
	c.cache = newCache
}
