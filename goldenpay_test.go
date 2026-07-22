package goldenpay

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

func TestConfigValidate(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty golden key")
	}
	cfg.GoldenKey = "test-key"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewConfig(t *testing.T) {
	cfg := NewConfig("my-key")
	if cfg.GoldenKey != "my-key" {
		t.Errorf("expected my-key, got %s", cfg.GoldenKey)
	}
}

// ---------------------------------------------------------------------------
// Crypto
// ---------------------------------------------------------------------------

func TestHMACSHA256(t *testing.T) {
	sig := HMACSHA256([]byte("secret"), []byte("hello"))
	if len(sig) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(sig))
	}
	// Same input -> same output
	sig2 := HMACSHA256([]byte("secret"), []byte("hello"))
	if !hmacEqual(sig, sig2) {
		t.Error("HMAC should be deterministic")
	}
}

func TestVerifyHMAC(t *testing.T) {
	payload := []byte("test-payload")
	secret := []byte("my-secret")
	sig := HMACSHA256(secret, payload)
	if !VerifyHMAC(secret, payload, sig) {
		t.Error("VerifyHMAC should return true for valid signature")
	}
	if VerifyHMAC(secret, payload, []byte("invalid")) {
		t.Error("VerifyHMAC should return false for invalid signature")
	}
	if VerifyHMAC([]byte("other-secret"), payload, sig) {
		t.Error("VerifyHMAC should return false for wrong secret")
	}
}

func hmacEqual(a, b []byte) bool {
	if len(a) != len(b) { return false }
	for i := range a {
		if a[i] != b[i] { return false }
	}
	return true
}

// ---------------------------------------------------------------------------
// SecureString
// ---------------------------------------------------------------------------

func TestSecureString(t *testing.T) {
	s := NewSecureString("hello")
	if s.Value() != "hello" {
		t.Errorf("expected hello, got %s", s.Value())
	}
	if s.String() != "***" {
		t.Errorf("expected ***, got %s", s.String())
	}
}

// ---------------------------------------------------------------------------
// Models: OfferEdit Merge
// ---------------------------------------------------------------------------

func TestOfferEditMerge(t *testing.T) {
	base := OfferEdit{Price: strPtr("100"), Active: boolPtr(true)}
	patch := OfferEdit{Price: strPtr("200")}
	merged := base.Merge(patch)
	if merged.Price == nil || *merged.Price != "200" {
		t.Error("Price should be overridden to 200")
	}
	if merged.Active == nil || *merged.Active != true {
		t.Error("Active should be preserved from base")
	}
}

func TestOfferEditMergeNilPatch(t *testing.T) {
	base := OfferEdit{Price: strPtr("100")}
	merged := base.Merge(OfferEdit{})
	if merged.Price == nil || *merged.Price != "100" {
		t.Error("Price should be preserved when patch is empty")
	}
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }

// ---------------------------------------------------------------------------
// Parsers
// ---------------------------------------------------------------------------

func TestBuildChatID(t *testing.T) {
	id := buildChatID(100, 200)
	if id != "users-100-200" {
		t.Errorf("expected users-100-200, got %s", id)
	}
	id = buildChatID(200, 100)
	if id != "users-100-200" {
		t.Errorf("expected users-100-200 (sorted), got %s", id)
	}
}

func TestParseAppDataJSON(t *testing.T) {
	m := parseAppDataJSON(`{"userId":"12345","csrf-token":"abc","other":"val"}`)
	if m["userId"] != "12345" {
		t.Errorf("expected 12345, got %s", m["userId"])
	}
	if m["csrf-token"] != "abc" {
		t.Errorf("expected abc, got %s", m["csrf-token"])
	}
}

func TestParseBalance(t *testing.T) {
	html := `<html><body><span class="badge-balance">1 234.56 ₽</span></body></html>`
	bal, err := parseBalance(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bal != 1234.56 {
		t.Errorf("expected 1234.56, got %f", bal)
	}
}

func TestParseUserFromHome(t *testing.T) {
	html := `<html><body data-app-data='{"userId":"42","csrf-token":"tok123"}'><div class="user-link-name">testuser</div></body></html>`
	user, err := parseUserFromHome(html, "sess123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != 42 { t.Errorf("expected 42, got %d", user.ID) }
	if user.Username != "testuser" { t.Errorf("expected testuser, got %s", user.Username) }
	if user.CSRFToken != "tok123" { t.Errorf("expected tok123, got %s", user.CSRFToken) }
	if user.PHPSessID != "sess123" { t.Errorf("expected sess123, got %s", user.PHPSessID) }
}

func TestParseOrders(t *testing.T) {
	html := `<html><body>` +
		`<a class="tc-item" href="/orders/abc123/">` +
		`<div class="tc-order">#A1B2C3D4</div>` +
		`<div class="order-desc">2 pcs Steam account</div>` +
		`<div class="media-user-name"><span data-href="/users/222/">BuyerOne</span></div>` +
		`<div class="text-muted">Steam Keys</div>` +
		`</a></body></html>`
	orders, err := parseOrders(html, 1)
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if len(orders) != 1 { t.Fatalf("expected 1 order, got %d", len(orders)) }
	o := orders[0]
	if o.ID != "#A1B2C3D4" { t.Errorf("expected #A1B2C3D4, got %s", o.ID) }
	if o.BuyerUsername != "BuyerOne" { t.Errorf("expected BuyerOne, got %s", o.BuyerUsername) }
	if o.BuyerID != 222 { t.Errorf("expected 222, got %d", o.BuyerID) }
	if o.Description != "2 pcs Steam account" { t.Errorf("expected '2 pcs Steam account', got '%s'", o.Description) }
	if o.SubcategoryName != "Steam Keys" { t.Errorf("expected 'Steam Keys', got '%s'", o.SubcategoryName) }
}

func TestParseMyOffers(t *testing.T) {
	html := `<html><body>` +
		`<a class="tc-item" data-offer="555" href="/lots/10/">` +
		`<div class="tc-desc-text">my offer</div>` +
		`<div class="tc-price" data-s="50.00">50.00 <span class="unit">₽</span></div>` +
		`</a></body></html>`
	offers, err := parseMyOffers(html, 10)
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if len(offers) != 1 { t.Fatalf("expected 1 offer, got %d", len(offers)) }
	o := offers[0]
	if o.ID != 555 { t.Errorf("expected 555, got %d", o.ID) }
	if o.NodeID != 10 { t.Errorf("expected 10, got %d", o.NodeID) }
	if o.Price != 50.0 { t.Errorf("expected 50.0, got %f", o.Price) }
	if !o.Active { t.Error("expected active") }
}

// ---------------------------------------------------------------------------
// Scheduler
// ---------------------------------------------------------------------------

func TestScheduleRuleIsActive(t *testing.T) {
	rule := NewScheduleRule(10, 18)
	if rule.IsActive(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)) != true {
		t.Error("expected active at 12:00")
	}
	if rule.IsActive(time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)) != false {
		t.Error("expected inactive at 09:00")
	}
	if rule.IsActive(time.Date(2024, 1, 1, 18, 0, 0, 0, time.UTC)) != false {
		t.Error("expected inactive at 18:00 (end is exclusive)")
	}
}

func TestScheduleRuleOvernight(t *testing.T) {
	rule := NewScheduleRule(22, 6)
	if rule.IsActive(time.Date(2024, 1, 1, 23, 0, 0, 0, time.UTC)) != true {
		t.Error("expected active at 23:00")
	}
	if rule.IsActive(time.Date(2024, 1, 1, 5, 0, 0, 0, time.UTC)) != true {
		t.Error("expected active at 05:00")
	}
	if rule.IsActive(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)) != false {
		t.Error("expected inactive at 12:00")
	}
}

func TestOfferSchedulerPoll(t *testing.T) {
	s := NewOfferScheduler([]ScheduleEntry{
		NewScheduleEntry("test", NewOfferGroup(1, true), NewScheduleRule(0, 24), ActionActivate),
	})
	transitions := s.Poll(time.Now())
	if len(transitions) != 1 {
		t.Fatalf("expected 1 transition, got %d", len(transitions))
	}
	if transitions[0].ShouldBeActive != true {
		t.Error("expected should be active")
	}
	// Second poll should have no transitions (no change)
	transitions = s.Poll(time.Now())
	if len(transitions) != 0 {
		t.Errorf("expected 0 transitions on second poll, got %d", len(transitions))
	}
}

func BenchmarkScheduleRule(b *testing.B) {
	rule := NewScheduleRule(0, 24)
	now := time.Now()
	for i := 0; i < b.N; i++ {
		rule.IsActive(now)
	}
}

// ---------------------------------------------------------------------------
// Storage
// ---------------------------------------------------------------------------

func TestMemoryStateStore(t *testing.T) {
	store := NewMemoryStateStore()
	state, err := store.Load()
	if err != nil { t.Fatalf("load: %v", err) }
	if state.SeenMessages == nil { t.Error("expected non-nil SeenMessages") }

	state.SeenOrders = []string{"ord1", "ord2"}
	state.SeenMessages = map[string]int64{"chat1": 5}
	if err := store.Save(state); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := store.Load()
	if err != nil { t.Fatalf("reload: %v", err) }
	if len(loaded.SeenOrders) != 2 { t.Errorf("expected 2 orders, got %d", len(loaded.SeenOrders)) }
	if loaded.SeenMessages["chat1"] != 5 { t.Errorf("expected 5, got %d", loaded.SeenMessages["chat1"]) }
}

// ---------------------------------------------------------------------------
// URLs
// ---------------------------------------------------------------------------

func TestUrls(t *testing.T) {
	u := NewUrls("https://funpay.com")
	if u.Home() != "https://funpay.com/" { t.Errorf("home: %s", u.Home()) }
	if u.OrdersTrade() != "https://funpay.com/orders/trade" { t.Errorf("orders: %s", u.OrdersTrade()) }
	if u.OrderPage("abc") != "https://funpay.com/orders/abc/" { t.Errorf("order: %s", u.OrderPage("abc")) }
	if u.LotsPage(10) != "https://funpay.com/lots/10/" { t.Errorf("lots: %s", u.LotsPage(10)) }
	if u.OfferEdit(10, 555) != "https://funpay.com/lots/offerEdit?node=10&offer=555" {
		t.Errorf("offerEdit: %s", u.OfferEdit(10, 555))
	}
}

// ---------------------------------------------------------------------------
// Runner response parsing
// ---------------------------------------------------------------------------

func TestParseRunnerResponse(t *testing.T) {
	raw := `{"success":true,"objects":[{"type":"chat_node","id":"1","data":{"messages":[]}}]}`
	resp, err := parseRunnerResponse(raw)
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if !resp.Success { t.Error("expected success") }
	if len(resp.Objects) != 1 { t.Errorf("expected 1 object, got %d", len(resp.Objects)) }
	if resp.Objects[0].Type != "chat_node" { t.Errorf("expected chat_node, got %s", resp.Objects[0].Type) }
}

// ---------------------------------------------------------------------------
// Offer build payload
// ---------------------------------------------------------------------------

func TestBuildOfferPayload(t *testing.T) {
	edit := &OfferEdit{
		Price: strPtr("150.00"),
		Active: boolPtr(true),
	}
	payload := buildOfferPayload(edit, "csrf123", 555, 10)
	if !strings.Contains(payload, "csrf_token=csrf123") { t.Error("missing csrf_token") }
	if !strings.Contains(payload, "offer_id=555") { t.Error("missing offer_id") }
	if !strings.Contains(payload, "price=150.00") { t.Error("missing price") }
	if !strings.Contains(payload, "active=on") { t.Error("missing active=on") }
}

// ---------------------------------------------------------------------------
// EventStream
// ---------------------------------------------------------------------------

func TestEventStreamShouldEmitOrder(t *testing.T) {
	es := &EventStream{SeenOrders: make(map[string]struct{}), SeenMessages: make(map[string]int64)}
	if !es.ShouldEmitOrder("new") { t.Error("expected true for new order") }
	if es.ShouldEmitOrder("new") { t.Error("expected false for duplicate order") }
}

func TestEventStreamShouldEmitMessage(t *testing.T) {
	es := &EventStream{SeenOrders: make(map[string]struct{}), SeenMessages: make(map[string]int64)}
	msg := ChatMessage{ID: 5, ChatID: "chat1", AuthorID: 42}
	if !es.ShouldEmitMessage(msg, nil) { t.Error("expected true for new message") }
	if es.ShouldEmitMessage(msg, nil) { t.Error("expected false for duplicate message") }
	// Older message
	msg2 := ChatMessage{ID: 3, ChatID: "chat1", AuthorID: 42}
	if es.ShouldEmitMessage(msg2, nil) { t.Error("expected false for older message") }
}

func TestEventStreamFilterIgnoreAuthor(t *testing.T) {
	es := &EventStream{SeenOrders: make(map[string]struct{}), SeenMessages: make(map[string]int64)}
	msg := ChatMessage{ID: 1, ChatID: "chat1", AuthorID: 99}
	filter := &MessageFilter{IgnoreAuthorID: 99}
	if es.ShouldEmitMessage(msg, filter) {
		t.Error("expected false for ignored author")
	}
}

// ---------------------------------------------------------------------------
// Webhook server config
// ---------------------------------------------------------------------------

func TestDefaultWebhookConfig(t *testing.T) {
	cfg := DefaultWebhookConfig()
	if cfg.BindAddr != "127.0.0.1:9090" { t.Errorf("unexpected bind: %s", cfg.BindAddr) }
	if cfg.Endpoint != "/webhook" { t.Errorf("unexpected endpoint: %s", cfg.Endpoint) }
	if cfg.MaxBodySize != 1<<20 { t.Errorf("unexpected max body: %d", cfg.MaxBodySize) }
}

// ---------------------------------------------------------------------------
// Client error types
// ---------------------------------------------------------------------------

func TestIsAuthError(t *testing.T) {
	if IsAuthError(nil) { t.Error("nil is not auth error") }
	if IsAuthError(newError(ErrHTTP, "http")) { t.Error("ErrHTTP is not auth error") }
	if !IsAuthError(newError(ErrUnauthorized, "auth")) { t.Error("ErrUnauthorized should be auth error") }
}
