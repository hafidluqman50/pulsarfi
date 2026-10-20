package agent_test

import (
	"strings"
	"testing"

	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/analyzer"
)

// Nova's prompt is generated from the one domain list, never a second copy
// (docs/plans/news-brief-evidence-cards.md). Relocated here when
// chat_status_test.go was retired together with announceStatus
// (docs/plans/quasar-thinking-label-and-reasoning-toggle.md).
func TestTrustedSources_PromptIsGeneratedFromTheList(t *testing.T) {
	section := analyzer.TrustedSourcesInstructions(analyzer.TrustedNewsDomains)
	for _, domain := range analyzer.TrustedNewsDomains {
		if !strings.Contains(section, domain) {
			t.Errorf("the prompt does not name the configured domain %q", domain)
		}
	}
	if !strings.Contains(section, analyzer.ExternalSourceMarker) {
		t.Errorf("the prompt must explain the marker the fallback search puts on external results")
	}

	custom := analyzer.TrustedSourcesInstructions([]string{"example.org"})
	if !strings.Contains(custom, "example.org") || strings.Contains(custom, "kompas.com") {
		t.Errorf("the section must reflect exactly the list it is given, got: %s", custom)
	}
}
