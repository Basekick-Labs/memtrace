package memory

import (
	"strings"
	"testing"
)

func TestBuildSearchSQLUsesExactMemoryID(t *testing.T) {
	query := buildSearchSQL("org_test", &SearchQuery{ID: "mem_0123456789abcdef"})
	if !strings.Contains(query, "id = 'mem_0123456789abcdef'") {
		t.Fatalf("query does not constrain exact memory ID: %s", query)
	}
	if strings.Contains(query, "content LIKE") {
		t.Fatalf("ID lookup unexpectedly searches content: %s", query)
	}
}
