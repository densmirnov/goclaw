package http

import (
	"net/http"

	"github.com/nextlevelbuilder/goclaw/internal/permissions"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

func requireRoleHTTP(token string, minRole permissions.Role, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		role := permissions.RoleViewer

		if token != "" {
			if !tokenMatch(extractBearerToken(r), token) {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			role = permissions.RoleAdmin
		} else {
			role = permissions.RoleOperator
		}

		if !hasMinRole(role, minRole) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}

		if userID := extractUserID(r); userID != "" {
			r = r.WithContext(store.WithUserID(r.Context(), userID))
		}
		next(w, r)
	}
}

func hasMinRole(role, min permissions.Role) bool {
	return roleLevel(role) >= roleLevel(min)
}

func roleLevel(role permissions.Role) int {
	switch role {
	case permissions.RoleAdmin:
		return 3
	case permissions.RoleOperator:
		return 2
	case permissions.RoleViewer:
		return 1
	default:
		return 0
	}
}
