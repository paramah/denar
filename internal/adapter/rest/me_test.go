package rest

import (
	"strings"
	"testing"
)

func TestUpdateMeLocale(t *testing.T) {
	w := newWorld(t)
	h := w.handler()
	owner := w.as(h, "owner")

	rec := owner.do("GET", "/api/me", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"locale":null`) {
		t.Fatalf("before = %d %s", rec.Code, rec.Body)
	}

	rec = owner.do("PATCH", "/api/me", `{"locale":"en"}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"locale":"en"`) || strings.Contains(rec.Body.String(), "password") {
		t.Fatalf("patch = %d %s", rec.Code, rec.Body)
	}
	rec = owner.do("GET", "/api/me", "")
	if !strings.Contains(rec.Body.String(), `"locale":"en"`) {
		t.Errorf("after = %s", rec.Body)
	}

	rec = owner.do("PATCH", "/api/me", `{"locale":null}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"locale":null`) {
		t.Errorf("clear = %d %s", rec.Code, rec.Body)
	}

	for _, body := range []string{`{"locale":"de"}`, `{}`, `{"locale":7}`} {
		rec = owner.do("PATCH", "/api/me", body)
		if rec.Code != 422 || (body != `{"locale":7}` && !strings.Contains(rec.Body.String(), `"locale"`)) {
			t.Errorf("%s = %d %s", body, rec.Code, rec.Body)
		}
		wantProblemShape(t, rec)
	}
	rec = owner.do("PATCH", "/api/me", `{"locale":"de"}`)
	if !strings.Contains(rec.Body.String(), "must be pl or en") {
		t.Errorf("message = %s", rec.Body)
	}

	rec = w.as(h, "bot").do("PATCH", "/api/me", `{"locale":"en"}`)
	if rec.Code != 403 {
		t.Errorf("bot = %d %s", rec.Code, rec.Body)
	}
	wantProblemShape(t, rec)
}
