package websocket

import db "github.com/merrydance/locallife/db/sqlc"

func IsMerchantClientRole(role string) bool {
	switch role {
	case "merchant", "merchant_owner", "merchant_staff":
		return true
	default:
		return false
	}
}

func ResolveClientInfoFromRoles(roles []db.UserRole) (ClientType, int64) {
	for _, role := range roles {
		if role.Role == "rider" && role.RelatedEntityID.Valid {
			return ClientTypeRider, role.RelatedEntityID.Int64
		}

		if IsMerchantClientRole(role.Role) && role.RelatedEntityID.Valid {
			return ClientTypeMerchant, role.RelatedEntityID.Int64
		}
	}

	return "", 0
}
