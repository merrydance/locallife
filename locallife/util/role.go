package util

const (
	DepositorRole = "depositor"
	BankerRole    = "banker"
	AdminRole     = "admin"
)

// IsSupportedRole returns true if the role is supported
func IsSupportedRole(role string) bool {
	switch role {
	case DepositorRole, BankerRole, AdminRole:
		return true
	}
	return false
}

// HasRole checks if the user's role matches any of the allowed roles
func HasRole(userRole string, allowedRoles ...string) bool {
	for _, role := range allowedRoles {
		if userRole == role {
			return true
		}
	}
	return false
}
