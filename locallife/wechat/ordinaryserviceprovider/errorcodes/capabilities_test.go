package errorcodes

import "testing"

func TestCapabilityCodeSetsMapEndpointCodes(t *testing.T) {
	groups := CapabilityCodeSetGroups()
	if len(groups) != 7 {
		t.Fatalf("capability code set group count = %d, want 7", len(groups))
	}

	seen := map[EndpointID]struct{}{}
	for _, group := range groups {
		if group.ID == "" || len(group.Endpoints) == 0 {
			t.Fatalf("invalid capability code set group: %+v", group)
		}
		for _, endpoint := range group.Endpoints {
			set, ok := EndpointCodeSetByID(endpoint)
			if !ok {
				t.Fatalf("%s/%s missing endpoint code set", group.ID, endpoint)
			}
			if set.Name == "" || set.Len() == 0 {
				t.Fatalf("%s/%s has empty documented code set", group.ID, endpoint)
			}
			if _, exists := seen[endpoint]; exists {
				t.Fatalf("duplicate endpoint code set id %s", endpoint)
			}
			seen[endpoint] = struct{}{}
		}
	}
	if len(seen) != len(EndpointCodeSets()) {
		t.Fatalf("grouped endpoint code set count = %d, code set map count = %d", len(seen), len(EndpointCodeSets()))
	}
}

func TestEndpointCodeSetByIDUsesCanonicalAliases(t *testing.T) {
	set, ok := EndpointCodeSetByID(EndpointCombinePrepay)
	if !ok {
		t.Fatal("missing combine prepay documented code set")
	}
	if !set.Has("NO_AUTH") || !set.Has("NOAUTH") {
		t.Fatal("combine prepay documented code set must match canonical and official legacy NOAUTH spelling")
	}
}
