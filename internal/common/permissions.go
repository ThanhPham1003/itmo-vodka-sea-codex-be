package common

var rolePermissions = map[string][]string{
	"broker":           {"dossier:create", "dossier:read", "dossier:upload"},
	"legal_specialist": {"dossier:create", "dossier:read", "dossier:read_all", "dossier:upload", "dossier:correct", "dossier:review", "dossier:reopen", "dossier:image_view", "pii:view", "audit:read"},
	"notary":           {"dossier:read", "dossier:image_view", "pii:view"},
	"manager":          {"dossier:read", "dossier:read_all", "dossier:assign", "dashboard:read", "audit:read"},
	"company_admin":    {"user:manage", "catalog:manage", "dossier:create", "dossier:read", "dossier:read_all", "dossier:upload", "dossier:assign", "dossier:correct", "dossier:review", "dossier:reopen", "dossier:image_view", "pii:view", "audit:read", "dashboard:read"},
}

func EffectivePermissions(role string, extra []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, values := range [][]string{rolePermissions[role], extra} {
		for _, value := range values {
			if !seen[value] {
				seen[value] = true
				result = append(result, value)
			}
		}
	}
	return result
}

func ValidCompanyRole(role string) bool { _, ok := rolePermissions[role]; return ok }

func KnownPermission(permission string) bool {
	for _, values := range rolePermissions {
		for _, value := range values {
			if value == permission {
				return true
			}
		}
	}
	return false
}
