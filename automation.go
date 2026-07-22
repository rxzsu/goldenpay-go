package goldenpay

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ---------------------------------------------------------------------------
// DeliveryItem
// ---------------------------------------------------------------------------

type DeliveryItem struct {
	Value string `json:"value"`
}

// DeliveryItemFormat controls how items are rendered in the delivery message.
type DeliveryItemFormat int

const (
	ItemFormatPlainLines DeliveryItemFormat = iota
	ItemFormatNumbered
	ItemFormatCodeBlock
)

// ---------------------------------------------------------------------------
// DeliveryMessageBuilder
// ---------------------------------------------------------------------------

type DeliveryMessageBuilder struct {
	Greeting         string
	Intro            string
	ItemFormat       DeliveryItemFormat
	IncludeOrderID   bool
	IncludeProductKey bool
	Footer           string
	Template         string
	useTemplate      bool
}

func NewDeliveryMessageBuilder() *DeliveryMessageBuilder {
	return &DeliveryMessageBuilder{
		Greeting:          "Thanks for your purchase!",
		Intro:             "Your item:",
		ItemFormat:        ItemFormatNumbered,
		IncludeOrderID:    true,
		IncludeProductKey: true,
		Footer:            "If you have any questions, reply in this chat.",
	}
}

func (b *DeliveryMessageBuilder) SetGreeting(v string) *DeliveryMessageBuilder        { b.Greeting = v; return b }
func (b *DeliveryMessageBuilder) SetIntro(v string) *DeliveryMessageBuilder           { b.Intro = v; return b }
func (b *DeliveryMessageBuilder) SetItemFormat(v DeliveryItemFormat) *DeliveryMessageBuilder { b.ItemFormat = v; return b }
func (b *DeliveryMessageBuilder) SetIncludeOrderID(v bool) *DeliveryMessageBuilder    { b.IncludeOrderID = v; return b }
func (b *DeliveryMessageBuilder) SetIncludeProductKey(v bool) *DeliveryMessageBuilder { b.IncludeProductKey = v; return b }
func (b *DeliveryMessageBuilder) SetFooter(v string) *DeliveryMessageBuilder          { b.Footer = v; return b }
func (b *DeliveryMessageBuilder) SetTemplate(v string) *DeliveryMessageBuilder        { b.Template = v; b.useTemplate = true; return b }
func (b *DeliveryMessageBuilder) NoTemplate() *DeliveryMessageBuilder                 { b.useTemplate = false; return b }
func (b *DeliveryMessageBuilder) NoFooter() *DeliveryMessageBuilder                   { b.Footer = ""; return b }

func (b *DeliveryMessageBuilder) FormatItems(items []DeliveryItem) string {
	switch b.ItemFormat {
	case ItemFormatPlainLines:
		var s string
		for i, it := range items {
			if i > 0 { s += "\n" }
			s += it.Value
		}
		return s
	case ItemFormatNumbered:
		var s string
		for i, it := range items {
			if i > 0 { s += "\n" }
			s += fmt.Sprintf("%d. %s", i+1, it.Value)
		}
		return s
	case ItemFormatCodeBlock:
		var s string
		for i, it := range items {
			if i > 0 { s += "\n" }
			s += it.Value
		}
		return "```\n" + s + "\n```"
	}
	return ""
}

func (b *DeliveryMessageBuilder) BuildMessage(order *OrderInfo, result *DeliveryResult) string {
	itemsBlock := b.FormatItems(result.Delivered)

	if b.useTemplate {
		msg := b.Template
		msg = stringsReplaceAll(msg, "{buyer}", order.BuyerUsername)
		msg = stringsReplaceAll(msg, "{order_id}", result.OrderID)
		msg = stringsReplaceAll(msg, "{product_key}", result.ProductKey)
		msg = stringsReplaceAll(msg, "{items}", itemsBlock)
		return msg
	}

	var msg string
	msg += b.Greeting + "\n"
	if b.IncludeOrderID {
		msg += fmt.Sprintf("Order: #%s\n", result.OrderID)
	}
	if b.IncludeProductKey {
		msg += fmt.Sprintf("Product: %s\n", result.ProductKey)
	}
	msg += fmt.Sprintf("Buyer: %s\n", order.BuyerUsername)
	msg += b.Intro + "\n"
	msg += itemsBlock + "\n"
	if b.Footer != "" {
		msg += b.Footer + "\n"
	}
	return msg
}

func stringsReplaceAll(s, old, new string) string {
	var result string
	for {
		i := 0
		for ; i+len(old) <= len(s); i++ {
			if s[i:i+len(old)] == old {
				result += s[:i] + new
				s = s[i+len(old):]
				break
			}
		}
		if i+len(old) > len(s) {
			result += s
			break
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// Inventory & Matching
// ---------------------------------------------------------------------------

type ProductInventory struct {
	Items []DeliveryItem
}

type DeliveryMatch struct {
	ProductKey string
	Items      []DeliveryItem
}

type DeliveryResult struct {
	OrderID    string         `json:"order_id"`
	ProductKey string         `json:"product_key"`
	Delivered  []DeliveryItem `json:"delivered"`
}

type ReservedDelivery struct {
	Result DeliveryResult
}

// ProductMatcher determines whether a product matches an order.
type ProductMatcher interface {
	Matches(productKey string, order *OrderInfo) bool
}

// ExactSubcategoryMatcher matches orders where subcategory equals the product key.
type ExactSubcategoryMatcher struct{}

func (ExactSubcategoryMatcher) Matches(productKey string, order *OrderInfo) bool {
	return productKey == order.SubcategoryName
}

// ---------------------------------------------------------------------------
// Delivery errors
// ---------------------------------------------------------------------------

type DeliveryError struct {
	Kind    string
	Message string
}

func (e *DeliveryError) Error() string { return e.Message }

var (
	ErrProductNotFound      = &DeliveryError{Kind: "ProductNotFound", Message: "product not found"}
	ErrAlreadyDelivered     = &DeliveryError{Kind: "AlreadyDelivered", Message: "order was already delivered"}
	ErrOrderNotPaid         = &DeliveryError{Kind: "OrderNotPaid", Message: "order is not paid"}
	ErrMessageSendFailed    = &DeliveryError{Kind: "MessageSendFailed", Message: "delivery message was rejected"}
)

func notEnoughItemsErr(requested, available int) *DeliveryError {
	return &DeliveryError{
		Kind:    "NotEnoughItems",
		Message: fmt.Sprintf("not enough items available: requested %d, available %d", requested, available),
	}
}

// ---------------------------------------------------------------------------
// DeliveryService
// ---------------------------------------------------------------------------

type DeliveryService struct {
	mu       sync.Mutex
	Products map[string]*ProductInventory
}

func NewDeliveryService() *DeliveryService {
	return &DeliveryService{Products: make(map[string]*ProductInventory)}
}

func (s *DeliveryService) AddProduct(key string, items []DeliveryItem) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Products[key] = &ProductInventory{Items: items}
}

func (s *DeliveryService) MatchOrder(matcher ProductMatcher, order *OrderInfo) (*DeliveryMatch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.matchOrderLocked(matcher, order)
}

func (s *DeliveryService) matchOrderLocked(matcher ProductMatcher, order *OrderInfo) (*DeliveryMatch, error) {
	var productKey string
	var inventory *ProductInventory
	for k, inv := range s.Products {
		if matcher.Matches(k, order) {
			productKey = k
			inventory = inv
			break
		}
	}
	if inventory == nil {
		return nil, ErrProductNotFound
	}

	requested := int(order.Amount)
	if requested < 0 { requested = 0 }
	available := len(inventory.Items)
	if available < requested {
		return nil, notEnoughItemsErr(requested, available)
	}

	items := make([]DeliveryItem, requested)
	copy(items, inventory.Items[:requested])
	return &DeliveryMatch{ProductKey: productKey, Items: items}, nil
}

func (s *DeliveryService) Deliver(matcher ProductMatcher, order *OrderInfo) (*DeliveryResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	matched, err := s.matchOrderLocked(matcher, order)
	if err != nil {
		return nil, err
	}

	inv := s.Products[matched.ProductKey]
	inv.Items = inv.Items[len(matched.Items):]

	return &DeliveryResult{
		OrderID:    order.ID,
		ProductKey: matched.ProductKey,
		Delivered:  matched.Items,
	}, nil
}

func (s *DeliveryService) Reserve(matcher ProductMatcher, order *OrderInfo) (*ReservedDelivery, error) {
	result, err := s.Deliver(matcher, order)
	if err != nil {
		return nil, err
	}
	return &ReservedDelivery{Result: *result}, nil
}

func (s *DeliveryService) ReleaseReserved(reserved *ReservedDelivery) {
	s.mu.Lock()
	defer s.mu.Unlock()

	inv, ok := s.Products[reserved.Result.ProductKey]
	if !ok {
		inv = &ProductInventory{}
		s.Products[reserved.Result.ProductKey] = inv
	}
	restored := append(reserved.Result.Delivered, inv.Items...)
	inv.Items = restored
}

func (s *DeliveryService) RemainingItems(productKey string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if inv, ok := s.Products[productKey]; ok {
		return len(inv.Items)
	}
	return 0
}

// ---------------------------------------------------------------------------
// DeliveryStore
// ---------------------------------------------------------------------------

type DeliveredOrderRecord struct {
	OrderID    string              `json:"order_id"`
	ProductKey string              `json:"product_key"`
	Delivered  []DeliveryItem      `json:"delivered"`
	Status     DeliveryRecordStatus `json:"status"`
}

type DeliveryRecordStatus string

const (
	RecordPending   DeliveryRecordStatus = "pending"
	RecordDelivered DeliveryRecordStatus = "delivered"
)

// DeliveryStore persists delivery records to prevent duplicate deliveries.
type DeliveryStore interface {
	ContainsOrder(orderID string) (bool, error)
	ClaimPending(result *DeliveryResult) error
	CommitDelivered(result *DeliveryResult) error
	ReleasePending(orderID string) error
}

// MemoryDeliveryStore is an in-memory delivery store.
type MemoryDeliveryStore struct {
	mu      sync.Mutex
	records map[string]*DeliveredOrderRecord
}

func NewMemoryDeliveryStore() *MemoryDeliveryStore {
	return &MemoryDeliveryStore{records: make(map[string]*DeliveredOrderRecord)}
}

func (s *MemoryDeliveryStore) ContainsOrder(orderID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.records[orderID]
	return ok, nil
}

func (s *MemoryDeliveryStore) ClaimPending(result *DeliveryResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.records[result.OrderID]; ok {
		return ErrAlreadyDelivered
	}
	s.records[result.OrderID] = &DeliveredOrderRecord{
		OrderID:    result.OrderID,
		ProductKey: result.ProductKey,
		Delivered:  result.Delivered,
		Status:     RecordPending,
	}
	return nil
}

func (s *MemoryDeliveryStore) CommitDelivered(result *DeliveryResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[result.OrderID] = &DeliveredOrderRecord{
		OrderID:    result.OrderID,
		ProductKey: result.ProductKey,
		Delivered:  result.Delivered,
		Status:     RecordDelivered,
	}
	return nil
}

func (s *MemoryDeliveryStore) ReleasePending(orderID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, orderID)
	return nil
}

// JSONDeliveryStore persists delivery records to a JSON file.
type JSONDeliveryStore struct {
	path string
	mu   sync.Mutex
}

func NewJSONDeliveryStore(path string) *JSONDeliveryStore {
	return &JSONDeliveryStore{path: path}
}

func (s *JSONDeliveryStore) loadAll() (map[string]*DeliveredOrderRecord, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*DeliveredOrderRecord), nil
		}
		return nil, err
	}
	var records map[string]*DeliveredOrderRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	if records == nil {
		records = make(map[string]*DeliveredOrderRecord)
	}
	return records, nil
}

func (s *JSONDeliveryStore) saveAll(records map[string]*DeliveredOrderRecord) error {
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.path)
}

func (s *JSONDeliveryStore) ContainsOrder(orderID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.loadAll()
	if err != nil {
		return false, err
	}
	_, ok := records[orderID]
	return ok, nil
}

func (s *JSONDeliveryStore) ClaimPending(result *DeliveryResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.loadAll()
	if err != nil {
		return err
	}
	if _, ok := records[result.OrderID]; ok {
		return ErrAlreadyDelivered
	}
	records[result.OrderID] = &DeliveredOrderRecord{
		OrderID:    result.OrderID,
		ProductKey: result.ProductKey,
		Delivered:  result.Delivered,
		Status:     RecordPending,
	}
	return s.saveAll(records)
}

func (s *JSONDeliveryStore) CommitDelivered(result *DeliveryResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.loadAll()
	if err != nil {
		return err
	}
	records[result.OrderID] = &DeliveredOrderRecord{
		OrderID:    result.OrderID,
		ProductKey: result.ProductKey,
		Delivered:  result.Delivered,
		Status:     RecordDelivered,
	}
	return s.saveAll(records)
}

func (s *JSONDeliveryStore) ReleasePending(orderID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.loadAll()
	if err != nil {
		return err
	}
	if rec, ok := records[orderID]; ok && rec.Status == RecordPending {
		delete(records, orderID)
		return s.saveAll(records)
	}
	return nil
}

// ---------------------------------------------------------------------------
// DeliveryService integration methods
// ---------------------------------------------------------------------------

// DeliverOrder delivers an order with deduplication via DeliveryStore.
func (s *DeliveryService) DeliverOrder(matcher ProductMatcher, store DeliveryStore, order *OrderInfo) (*DeliveryResult, error) {
	ok, err := store.ContainsOrder(order.ID)
	if err != nil {
		return nil, err
	}
	if ok {
		return nil, ErrAlreadyDelivered
	}

	result, err := s.Deliver(matcher, order)
	if err != nil {
		return nil, err
	}

	if err := store.ClaimPending(result); err != nil {
		return nil, err
	}
	if err := store.CommitDelivered(result); err != nil {
		return nil, err
	}
	return result, nil
}

// ProcessPaidOrder is a high-level method that matches, reserves, sends, and commits.
func (s *DeliveryService) ProcessPaidOrder(
	matcher ProductMatcher,
	store DeliveryStore,
	messenger DeliveryMessenger,
	builder *DeliveryMessageBuilder,
	order *OrderInfo,
) (*ProcessPaidOrderResult, error) {
	if order.Status != OrderPaid {
		return nil, &DeliveryError{
			Kind:    "OrderNotPaid",
			Message: fmt.Sprintf("order is not paid: status=%s", order.Status),
		}
	}

	ok, err := store.ContainsOrder(order.ID)
	if err != nil {
		return nil, err
	}
	if ok {
		return nil, ErrAlreadyDelivered
	}

	reserved, err := s.Reserve(matcher, order)
	if err != nil {
		return nil, err
	}

	if err := store.ClaimPending(&reserved.Result); err != nil {
		return nil, err
	}

	messageText := builder.BuildMessage(order, &reserved.Result)
	runnerResponse, err := messenger.SendDeliveryMessage(order.ChatID, messageText)
	if err != nil {
		s.ReleaseReserved(reserved)
		store.ReleasePending(order.ID)
		return nil, err
	}
	if !runnerResponse.Success {
		s.ReleaseReserved(reserved)
		store.ReleasePending(order.ID)
		errMsg := runnerResponse.ErrorMessage
		if errMsg == "" {
			errMsg = "runner response reported failure"
		}
		return nil, &DeliveryError{
			Kind:    "MessageSendFailed",
			Message: fmt.Sprintf("delivery message was rejected: %s", errMsg),
		}
	}

	if err := store.CommitDelivered(&reserved.Result); err != nil {
		return nil, err
	}
	delivery := reserved.Result

	return &ProcessPaidOrderResult{
		Delivery:       &delivery,
		MessageText:   messageText,
		RunnerResponse: runnerResponse,
	}, nil
}

// DeliveryMessenger sends delivery messages (interface for testability).
type DeliveryMessenger interface {
	SendDeliveryMessage(chatID, text string) (*RunnerResponse, error)
}

// SessionMessenger adapts GoldenPaySession to DeliveryMessenger.
type SessionMessenger struct {
	Session *GoldenPaySession
}

func (m *SessionMessenger) SendDeliveryMessage(chatID, text string) (*RunnerResponse, error) {
	return m.Session.SendMessage(chatID, text)
}

// ProcessPaidOrderResult holds the outcome of ProcessPaidOrder.
type ProcessPaidOrderResult struct {
	Delivery       *DeliveryResult
	MessageText   string
	RunnerResponse *RunnerResponse
}
