package mcp

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// contractTools reads the tool tables of api/mcp-tools.md: each row that
// starts with a tool name in backticks, with the scopes of its last column.
func contractTools(t *testing.T) map[string][]string {
	t.Helper()
	raw, err := os.ReadFile("../../../api/mcp-tools.md")
	if err != nil {
		t.Fatal(err)
	}
	_, tools, found := strings.Cut(string(raw), "## Narzędzia")
	if !found {
		t.Fatal("the contract has no section Narzędzia")
	}
	row := regexp.MustCompile("(?m)^\\| `([a-z_]+)` \\|.*\\| ([^|]*) \\|$")
	scope := regexp.MustCompile("`([a-z]+:[a-z]+)`")
	out := map[string][]string{}
	for _, m := range row.FindAllStringSubmatch(tools, -1) {
		var scopes []string
		for _, s := range scope.FindAllStringSubmatch(m[2], -1) {
			scopes = append(scopes, s[1])
		}
		out[m[1]] = scopes
	}
	return out
}

// The tools the server lists are exactly those of the contract, and each
// one's description names the scopes the contract requires.
func TestToolsMatchTheContract(t *testing.T) {
	w := newWorld(t)
	s := w.connect(w.addBot("b1", w.owner, allScopes))
	listed, err := s.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	contract := contractTools(t)
	var names []string
	for _, tool := range listed.Tools {
		names = append(names, tool.Name)
		scopes, ok := contract[tool.Name]
		if !ok {
			t.Errorf("%s is not in api/mcp-tools.md", tool.Name)
			continue
		}
		if len(tool.Description) < 40 {
			t.Errorf("%s: description too short: %q", tool.Name, tool.Description)
		}
		for _, sc := range scopes {
			if !strings.Contains(tool.Description, sc) {
				t.Errorf("%s: the description does not name the scope %s", tool.Name, sc)
			}
		}
		if tool.InputSchema == nil {
			t.Errorf("%s has no input schema", tool.Name)
		}
	}
	for name := range contract {
		if !slices.Contains(names, name) {
			t.Errorf("%s of the contract is not served", name)
		}
	}
	if len(contract) != 25 {
		t.Errorf("the contract has %d tools", len(contract))
	}
}
