package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/service"
	"github.com/TarunVishwakarma1/ims/backend/pkg/rbac"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type RoleHandler struct {
	service  service.RoleService
	validate *validator.Validate
}

func NewRoleHandler(service service.RoleService) *RoleHandler {
	return &RoleHandler{
		service:  service,
		validate: validator.New(),
	}
}

func (h *RoleHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.service.ListRoles(r.Context())
	if err != nil {
		zap.L().Error("failed to list roles", zap.Error(err))
		http.Error(w, "Failed to list roles", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, roles)
}

func (h *RoleHandler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	permissions, err := h.service.ListPermissions(r.Context())
	if err != nil {
		zap.L().Error("failed to list permissions", zap.Error(err))
		http.Error(w, "Failed to list permissions", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, permissions)
}

type CreateRoleRequest struct {
	Name        string `json:"name" validate:"required,min=2,max=50"`
	Description string `json:"description" validate:"max=200"`
}

func (h *RoleHandler) CreateRole(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req CreateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	role := &domain.Role{
		Name:        req.Name,
		Description: req.Description,
		IsSystem:    false,
	}

	if err := h.service.CreateRole(r.Context(), role); err != nil {
		zap.L().Error("failed to create role", zap.Error(err))
		http.Error(w, "Failed to create role", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, role)
}

type UpdateRolePermissionsRequest struct {
	PermissionIDs []uuid.UUID `json:"permission_ids" validate:"required"`
}

func (h *RoleHandler) UpdateRolePermissions(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	roleID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid role ID", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req UpdateRolePermissionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := h.service.UpdateRolePermissions(r.Context(), roleID, req.PermissionIDs); err != nil {
		zap.L().Error("failed to update role permissions", zap.Error(err))
		http.Error(w, "Failed to update role permissions", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "permissions updated successfully"})
}

type UpdateRoleRequest struct {
	Name        string `json:"name" validate:"required,min=2,max=50"`
	Description string `json:"description"`
}

func (h *RoleHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid role ID"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	if err := h.validate.Struct(req); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}

	if err := h.service.UpdateRole(r.Context(), id, req.Name, req.Description); err != nil {
		zap.L().Error("update role failed", zap.Error(err))
		writeJSON(w, 500, map[string]string{"error": "internal server error"})
		return
	}

	reloadAfterChange(h.service, r.Context())
	writeJSON(w, 200, map[string]string{"message": "role updated"})
}

func (h *RoleHandler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid role ID"})
		return
	}

	if err := h.service.DeleteRole(r.Context(), id); err != nil {
		zap.L().Error("delete role failed", zap.Error(err))
		writeJSON(w, 500, map[string]string{"error": "internal server error"})
		return
	}

	reloadAfterChange(h.service, r.Context())
	writeJSON(w, 200, map[string]string{"message": "role deleted"})
}

// helper to auto-reload cache after mutation
func reloadAfterChange(svc service.RoleService, ctx context.Context) {
	if perms, err := svc.LoadRolePermissions(ctx); err == nil {
		rbac.Cache.Load(perms)
	}
}

func (h *RoleHandler) ReloadPermissions(w http.ResponseWriter, r *http.Request) {
	rolePerms, err := h.service.LoadRolePermissions(r.Context())
	if err != nil {
		zap.L().Error("failed to load role permissions", zap.Error(err))
		http.Error(w, "Failed to reload cache", http.StatusInternalServerError)
		return
	}

	rbac.Cache.Load(rolePerms)
	writeJSON(w, http.StatusOK, map[string]string{"message": "cache reloaded successfully"})
}
