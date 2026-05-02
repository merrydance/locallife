package contracts

import (
	"reflect"
	"testing"
)

func TestCapabilityGroupsExposeExecutableContracts(t *testing.T) {
	tests := map[CapabilityID]int{
		CapabilityApplyment:          7,
		CapabilityAccountWillingness: 5,
		CapabilityMerchantManagement: 8,
		CapabilityPayment:            8,
		CapabilityCombinePayment:     8,
		CapabilityRefund:             3,
		CapabilityProfitSharing:      9,
	}

	groups := CapabilityGroups()
	if len(groups) != len(tests) {
		t.Fatalf("capability group count = %d, want %d", len(groups), len(tests))
	}
	seenEndpoints := map[EndpointID]struct{}{}
	for _, group := range groups {
		want, ok := tests[group.ID]
		if !ok {
			t.Fatalf("unexpected capability group %s", group.ID)
		}
		if len(group.Endpoints) != want {
			t.Fatalf("%s endpoint count = %d, want %d", group.ID, len(group.Endpoints), want)
		}
		for _, endpoint := range group.Endpoints {
			contract, ok := EndpointContractByID(endpoint)
			if !ok {
				t.Fatalf("%s endpoint %s missing executable contract", group.ID, endpoint)
			}
			if contract.Capability != group.ID {
				t.Fatalf("%s capability = %s, want %s", endpoint, contract.Capability, group.ID)
			}
			if contract.Method == "" || contract.Path == "" || contract.Operation == "" {
				t.Fatalf("%s must expose method, path and operation", endpoint)
			}
			if len(contract.RequestTypes) == 0 && len(contract.ResponseTypes) == 0 {
				t.Fatalf("%s must expose request or response contract type", endpoint)
			}
			for _, typ := range append(contract.RequestTypes, contract.ResponseTypes...) {
				if typ == nil || typ.Kind() != reflect.Struct {
					t.Fatalf("%s contract type must be struct, got %v", endpoint, typ)
				}
			}
			if len(contract.RequestTypes) > 0 && contract.RequestTypes[0] != reflect.TypeOf(NoRequestBody{}) && contract.RequestValidator == nil {
				t.Fatalf("%s request contract %s must provide validation", endpoint, contract.RequestTypes[0].Name())
			}
			if _, exists := seenEndpoints[endpoint]; exists {
				t.Fatalf("duplicate endpoint id %s", endpoint)
			}
			seenEndpoints[endpoint] = struct{}{}
		}
	}
	if len(seenEndpoints) != len(EndpointContracts()) {
		t.Fatalf("grouped endpoint count = %d, contract map count = %d", len(seenEndpoints), len(EndpointContracts()))
	}
}

func TestValidateEndpointRequestUsesMappedContract(t *testing.T) {
	err := ValidateEndpointRequest(EndpointApplymentSubmit, ApplymentSubmitRequest{})
	if err == nil {
		t.Fatal("expected mapped applyment submit validation to reject empty request")
	}

	err = ValidateEndpointRequest(EndpointApplymentSubmit, PaymentPrepayRequest{})
	if err == nil {
		t.Fatal("expected mapped validation to reject wrong request type")
	}

	if err := ValidateEndpointRequest(EndpointViolationNotificationConfigQuery, NoRequestBody{}); err != nil {
		t.Fatalf("expected no-body endpoint request to pass, got %v", err)
	}
}
