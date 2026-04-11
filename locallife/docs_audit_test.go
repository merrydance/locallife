package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/merrydance/locallife/internal/docaudit"
)

func TestUserJourneyDocEndpointsImplemented(t *testing.T) {
	result, err := docaudit.AuditDocEndpoints("docs/phase0/user_journey_mermaid.md", "api")
	if err != nil {
		t.Fatalf("audit failed: %v", err)
	}
	if len(result.MissingDocEndpoints) == 0 {
		return
	}

	var b strings.Builder
	b.WriteString("doc endpoints missing in code (@Router + api/server.go):\n")
	for _, m := range result.MissingDocEndpoints {
		b.WriteString(fmt.Sprintf("- %s (line %d) raw=%s\n", m.Path, m.Line, m.Raw))
	}
	t.Fatal(b.String())
}
