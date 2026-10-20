package audit

import "testing"

func TestChanges(t *testing.T) {
	c := Changes{}.Set("name", "Dom").Diff("role", "member", "admin").Diff("tz", "UTC", "UTC")
	if len(c) != 2 || c["name"].To != "Dom" || c["name"].From != nil || c["role"].From != "member" {
		t.Errorf("changes = %+v", c)
	}
}
