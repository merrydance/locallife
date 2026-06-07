package logic

var membershipBalanceSupportedScenes = []string{"dine_in", "takeaway"}

func IsMembershipBalanceSupportedOrderType(orderType string) bool {
	for _, scene := range membershipBalanceSupportedScenes {
		if scene == orderType {
			return true
		}
	}
	return false
}

func sanitizeMembershipUsableScenes(scenes []string) []string {
	if len(scenes) == 0 {
		return []string{}
	}

	result := make([]string, 0, len(scenes))
	seen := make(map[string]struct{}, len(scenes))
	for _, scene := range scenes {
		if !IsMembershipBalanceSupportedOrderType(scene) {
			continue
		}
		if _, ok := seen[scene]; ok {
			continue
		}
		seen[scene] = struct{}{}
		result = append(result, scene)
	}

	return result
}
