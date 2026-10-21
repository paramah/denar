package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"git.eiac.dev/tools/denar/internal/usecase"
	"git.eiac.dev/tools/denar/internal/usecase/fakes"
)

// hookCatcher is a webhook receiver that keeps what it got.
type hookCatcher struct {
	mu       sync.Mutex
	bodies   [][]byte
	headers  []http.Header
	failNext int
}

func (h *hookCatcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.failNext > 0 {
		h.failNext--
		http.Error(w, "busy", http.StatusServiceUnavailable)
		return
	}
	h.bodies = append(h.bodies, body)
	h.headers = append(h.headers, r.Header.Clone())
	w.WriteHeader(http.StatusNoContent)
}

func (h *hookCatcher) got() ([][]byte, []http.Header) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([][]byte(nil), h.bodies...), append([]http.Header(nil), h.headers...)
}

type notificationJSON struct {
	ID     string  `json:"id"`
	Type   string  `json:"type"`
	Title  string  `json:"title"`
	Body   string  `json:"body"`
	Link   string  `json:"link"`
	ReadAt *string `json:"read_at"`
}

// TestNotificationsEndToEnd is the walk of stage 005: a subscription due in
// three days, a webhook channel with a secret chosen for renewals, the
// subscription job puts the reminder in the outbox, the notification job
// makes the notification in the app and sends the webhook, signed, after
// one failed attempt and its backoff.
func TestNotificationsEndToEnd(t *testing.T) {
	hooks := &hookCatcher{failNext: 1}
	hookServer := httptest.NewServer(hooks)
	defer hookServer.Close()
	setEnv(t, "DENAR_AUTO_MIGRATE", "true", "DENAR_BASE_URL", "https://denar.example.com",
		"DENAR_MASTER_KEY", "AQIDBAUGBwgJCgsMDQ4PEBESExQVFhcYGRobHB0eHyA=")
	clock := fakes.NewClock(time.Date(2026, 10, 21, 9, 30, 0, 0, time.UTC))
	e := startE2EWith(t, appOptions{clock: clock, noSubscriptionJobs: true, noNotificationJobs: true})
	ctx := context.Background()

	admin := e.browser()
	admin.must(200, "POST", "/api/auth/login", `{"email":"`+adminEmail+`","password":"`+adminPassword+`"}`, nil)
	var h household
	admin.must(201, "POST", "/api/households", `{"name":"Dom","base_currency":"PLN","timezone":"Europe/Warsaw","month_start_day":1}`, &h)
	api := "/api/households/" + h.ID
	var sub struct {
		ID string `json:"id"`
	}
	admin.must(201, "POST", api+"/subscriptions", `{"name":"Netflix","vendor":"netflix","price":{"amount":"43.00","currency":"PLN"},
		"cycle":{"unit":"month","anchor":"2026-10-24"},"reminder_offsets":[3]}`, &sub)

	var types struct {
		Items []struct {
			Type      string `json:"type"`
			Available bool   `json:"available"`
		} `json:"items"`
	}
	admin.must(200, "GET", "/api/me/channel-types", "", &types)
	if len(types.Items) != 5 || types.Items[0].Type != "email" || types.Items[0].Available || !types.Items[3].Available {
		t.Fatalf("channel types = %+v", types)
	}
	var ch struct {
		ID        string `json:"id"`
		HasSecret bool   `json:"has_secret"`
	}
	admin.must(201, "POST", "/api/me/channels", `{"type":"webhook","name":"Automatyka","config":{"url":"`+hookServer.URL+`/denar"},
		"secrets":{"secret":"hook-s3cret"}}`, &ch)
	if !ch.HasSecret {
		t.Fatalf("channel = %+v", ch)
	}
	admin.must(200, "PUT", "/api/me/households/"+h.ID+"/notification-preferences",
		`{"items":[{"type":"subscription.renewal_upcoming","in_app":true,"channel_ids":["`+ch.ID+`"]}]}`, nil)

	if r, err := e.app.runSubscriptions(ctx); err != nil || r.Reminders != 1 {
		t.Fatalf("subscription jobs = %+v %v", r, err)
	}
	r, err := e.app.runNotifications(ctx)
	if err != nil || r.Dispatch.Events != 1 || r.Dispatch.Notifications != 1 || r.Dispatch.Deliveries != 1 || r.Delivery.Retried != 1 {
		t.Fatalf("notification jobs = %+v %v", r, err)
	}

	var page struct {
		Items      []notificationJSON `json:"items"`
		NextCursor *string            `json:"next_cursor"`
	}
	admin.must(200, "GET", "/api/me/notifications?household_id="+h.ID+"&unread=true", "", &page)
	if len(page.Items) != 1 || page.Items[0].Type != "subscription.renewal_upcoming" || page.Items[0].Title != "Odnowienie: Netflix za 3 dni" ||
		page.Items[0].Link != "/h/"+h.ID+"/subscriptions/"+sub.ID || !strings.Contains(page.Items[0].Body, "24.10.2026: 43.00 PLN") {
		t.Fatalf("notifications = %+v", page)
	}
	var count struct {
		Count int `json:"count"`
	}
	admin.must(200, "GET", "/api/me/notifications/unread-count", "", &count)
	if count.Count != 1 {
		t.Errorf("unread = %d", count.Count)
	}

	// The webhook answered 503 once; a minute later it gets the delivery.
	if bodies, _ := hooks.got(); len(bodies) != 0 {
		t.Fatalf("delivered before the retry: %d", len(bodies))
	}
	clock.Advance(time.Minute)
	if r, err := e.app.runNotifications(ctx); err != nil || r.Delivery.Sent != 1 {
		t.Fatalf("retry = %+v %v", r, err)
	}
	bodies, headers := hooks.got()
	if len(bodies) != 1 {
		t.Fatalf("webhooks = %d", len(bodies))
	}
	mac := hmac.New(sha256.New, []byte("hook-s3cret"))
	mac.Write(bodies[0])
	if want := "sha256=" + hex.EncodeToString(mac.Sum(nil)); headers[0].Get("X-Denar-Signature") != want {
		t.Errorf("signature = %q, want %q", headers[0].Get("X-Denar-Signature"), want)
	}
	if headers[0].Get("X-Denar-Event") != "subscription.renewal_upcoming" || headers[0].Get("X-Denar-Delivery") == "" {
		t.Errorf("headers = %v", headers[0])
	}
	var payload struct {
		Type      string `json:"type"`
		Title     string `json:"title"`
		URL       string `json:"url"`
		Household struct{ ID, Name string }
		Data      map[string]string `json:"data"`
	}
	if err := json.Unmarshal(bodies[0], &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Title != "Odnowienie: Netflix za 3 dni" || payload.URL != "https://denar.example.com/h/"+h.ID+"/subscriptions/"+sub.ID ||
		payload.Household.Name != "Dom" || payload.Data["due_date"] != "2026-10-24" || payload.Data["amount"] != "43.00" {
		t.Errorf("payload = %+v", payload)
	}
	var channel struct {
		VerifiedAt *string `json:"verified_at"`
		LastError  *string `json:"last_error"`
	}
	admin.must(200, "GET", "/api/me/channels/"+ch.ID, "", &channel)
	if channel.VerifiedAt == nil || channel.LastError != nil {
		t.Errorf("channel = %+v", channel)
	}

	var read notificationJSON
	admin.must(200, "POST", "/api/me/notifications/"+page.Items[0].ID+"/read", "", &read)
	if read.ReadAt == nil {
		t.Errorf("read = %+v", read)
	}
	if r, _ := e.app.runNotifications(ctx); r != (usecase.NotificationJobResult{}) {
		t.Errorf("nothing left to do, got %+v", r)
	}
}
