package goldenpay

import "encoding/json"

// UserInfo holds authenticated user metadata.
type UserInfo struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	CSRFToken string `json:"csrf_token"`
	PHPSessID string `json:"phpsessid,omitempty"`
}

//
// Orders
//

type OrderStatus string

const (
	OrderPaid     OrderStatus = "paid"
	OrderClosed   OrderStatus = "closed"
	OrderRefunded OrderStatus = "refunded"
)

// OrderInfo is a compact order from the trade page.
type OrderInfo struct {
	ID              string      `json:"id"`
	BuyerUsername   string      `json:"buyer_username"`
	BuyerID         int64       `json:"buyer_id"`
	ChatID          string      `json:"chat_id"`
	Description     string      `json:"description"`
	SubcategoryName string      `json:"subcategory_name"`
	Amount          int32       `json:"amount"`
	Status          OrderStatus `json:"status"`
}

// OrderPage is a detailed order with secrets and review.
type OrderPage struct {
	ID              string      `json:"id"`
	Status          OrderStatus `json:"status"`
	Amount          int32       `json:"amount"`
	Sum             float64     `json:"sum"`
	Currency        string      `json:"currency"`
	BuyerID         int64       `json:"buyer_id"`
	BuyerUsername   string      `json:"buyer_username"`
	ChatID          string      `json:"chat_id"`
	ShortDesc       string      `json:"short_description,omitempty"`
	FullDesc        string      `json:"full_description,omitempty"`
	SubcategoryName string      `json:"subcategory_name,omitempty"`
	Secrets         []string    `json:"secrets"`
	Params          [][2]string `json:"params"`
	Review          *Review     `json:"review,omitempty"`
	RawHTML         string      `json:"raw_html"`
}

type Review struct {
	Stars int    `json:"stars"`
	Text  string `json:"text,omitempty"`
}

// FetchOrderOptions for client-side filtering.
type FetchOrderOptions struct {
	Status        *OrderStatus
	MinAmount     *int32
	MaxAmount     *int32
	Subcategory   *string
	BuyerID       *int64
	BuyerUsername *string
	Description   *string
}

// StoreStatistics computed from fetched orders.
type StoreStatistics struct {
	TotalRevenue    float64
	TotalSum        float64
	OrderCount      int
	UniqueBuyers    int
	BuyerUsernames  []string
}

//
// Chat
//

// ChatMessage represents a single chat message.
type ChatMessage struct {
	ID       int64  `json:"id"`
	ChatID   string `json:"chat_id"`
	AuthorID int64  `json:"author_id"`
	Text     string `json:"text,omitempty"`
}

//
// Offers
//

// Offer is a seller's own offer on the trade page.
type Offer struct {
	ID          int64   `json:"id"`
	NodeID      int64   `json:"node_id"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Currency    string  `json:"currency"`
	Active      bool    `json:"active"`
}

// MarketOffer is a public offer on the market page.
type MarketOffer struct {
	ID             int64   `json:"id"`
	NodeID         int64   `json:"node_id"`
	Description    string  `json:"description"`
	Price          float64 `json:"price"`
	Currency       string  `json:"currency"`
	SellerID       int64   `json:"seller_id"`
	SellerName     string  `json:"seller_name"`
	SellerOnline   bool    `json:"seller_online"`
	SellerRating   float64 `json:"seller_rating"`
	SellerReviews  int32   `json:"seller_reviews"`
	IsPromo        bool    `json:"is_promo"`
}

// OfferEdit patches or creates an offer.
type OfferEdit struct {
	Quantity            *string `json:"quantity,omitempty"`
	Quantity2           *string `json:"quantity2,omitempty"`
	Method              *string `json:"method,omitempty"`
	OfferType           *string `json:"offer_type,omitempty"`
	ServerID            *string `json:"server_id,omitempty"`
	Location            *string `json:"location,omitempty"`
	Price               *string `json:"price,omitempty"`
	Active              *bool   `json:"active,omitempty"`
	Deleted             *bool   `json:"deleted,omitempty"`
	DescriptionRU       *string `json:"desc_ru,omitempty"`
	DescriptionEN       *string `json:"desc_en,omitempty"`
	PaymentMsgRU        *string `json:"payment_msg_ru,omitempty"`
	PaymentMsgEN        *string `json:"payment_msg_en,omitempty"`
	SummaryRU           *string `json:"summary_ru,omitempty"`
	SummaryEN           *string `json:"summary_en,omitempty"`
	Game                *string `json:"game,omitempty"`
	Images              *string `json:"images,omitempty"`
	DeactivateAfterSale *bool   `json:"deactivate_after_sale,omitempty"`
}

// Merge returns a new OfferEdit with non-nil fields from other overriding.
func (e OfferEdit) Merge(other OfferEdit) OfferEdit {
	if other.Quantity != nil            { e.Quantity = other.Quantity }
	if other.Quantity2 != nil           { e.Quantity2 = other.Quantity2 }
	if other.Method != nil              { e.Method = other.Method }
	if other.OfferType != nil           { e.OfferType = other.OfferType }
	if other.ServerID != nil            { e.ServerID = other.ServerID }
	if other.Location != nil            { e.Location = other.Location }
	if other.Price != nil               { e.Price = other.Price }
	if other.Active != nil              { e.Active = other.Active }
	if other.Deleted != nil             { e.Deleted = other.Deleted }
	if other.DescriptionRU != nil       { e.DescriptionRU = other.DescriptionRU }
	if other.DescriptionEN != nil       { e.DescriptionEN = other.DescriptionEN }
	if other.PaymentMsgRU != nil        { e.PaymentMsgRU = other.PaymentMsgRU }
	if other.PaymentMsgEN != nil        { e.PaymentMsgEN = other.PaymentMsgEN }
	if other.SummaryRU != nil           { e.SummaryRU = other.SummaryRU }
	if other.SummaryEN != nil           { e.SummaryEN = other.SummaryEN }
	if other.Game != nil                { e.Game = other.Game }
	if other.Images != nil              { e.Images = other.Images }
	if other.DeactivateAfterSale != nil { e.DeactivateAfterSale = other.DeactivateAfterSale }
	return e
}

// OfferField is a dynamic custom field in the offer edit form.
type OfferField struct {
	Name      string `json:"name"`
	Label     string `json:"label"`
	FieldType string `json:"field_type"` // input, textarea, select
	Value     string `json:"value"`
	Required  bool   `json:"required"`
}

// OfferDetails is the current state of an offer from the edit page.
type OfferDetails struct {
	Current      OfferEdit    `json:"current"`
	CustomFields []OfferField `json:"custom_fields"`
}

// OfferSaveResponse after editing/creating an offer.
type OfferSaveResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

//
// Runner
//

// RunnerResponse from the /runner/ endpoint.
type RunnerResponse struct {
	Success       bool            `json:"success"`
	ErrorMessage  string          `json:"error_message,omitempty"`
	Objects       []RunnerObject  `json:"objects"`
}

// RunnerObject is a typed object from the runner response.
type RunnerObject struct {
	Type       string          `json:"type"`
	ID         string          `json:"id"`
	Data       json.RawMessage `json:"data"`
}

//
// Price
//

// PriceCalculation holds price breakdown.
type PriceCalculation struct {
	InputPrice    float64            `json:"input_price"`
	SellerPrice   *float64           `json:"seller_price,omitempty"`
	BuyerPrice    *float64           `json:"buyer_price,omitempty"`
	Commission    *float64           `json:"commission,omitempty"`
	NumericFields map[string]float64 `json:"numeric_fields"`
}

//
// Categories
//

// CategoryNode is a node in the marketplace category tree.
type CategoryNode struct {
	ID              int64          `json:"id"`
	Name            string         `json:"name"`
	SubcategoryType *string        `json:"subcategory_type,omitempty"`
	Children        []CategoryNode `json:"children"`
}

// CategorySubcategory is a subcategory pill on the category page.
type CategorySubcategory struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	OfferCount      int    `json:"offer_count"`
	SubcategoryType string `json:"subcategory_type"`
	IsActive        bool   `json:"is_active"`
}

// CategoryFilter is a filter control on the category page.
type CategoryFilter struct {
	ID       string                  `json:"id"`
	Name     string                  `json:"name"`
	FilterType string                `json:"filter_type"` // select, radio, range, checkbox
	Options  []CategoryFilterOption  `json:"options"`
}

type CategoryFilterOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

//
// Reviews
//

// ProfileReview from a user profile page.
type ProfileReview struct {
	BuyerUsername string `json:"buyer_username"`
	BuyerID       int64  `json:"buyer_id"`
	Stars         int    `json:"stars"`
	Text          string `json:"text"`
	OrderID       string `json:"order_id"`
}

//
// Withdraw
//

// WithdrawRequest initiates a payout.
type WithdrawRequest struct {
	Currency    string  `json:"currency"`
	ExtCurrency string  `json:"ext_currency"`
	Wallet      string  `json:"wallet"`
	Amount      float64 `json:"amount"`
}

//
// Raise
//

type RaiseOffersResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

//
// Bot
//

// BotState for persistence.
type BotState struct {
	SeenOrders   []string        `json:"seen_orders"`
	SeenMessages map[string]int64 `json:"seen_messages"`
}

//
// Event
//

// GoldenPayEvent emitted by the bot.
type GoldenPayEvent struct {
	NewOrder   *OrderInfo    `json:"new_order,omitempty"`
	NewMessage *ChatMessage  `json:"new_message,omitempty"`
}

// MessageFilter for filtering chat messages.
type MessageFilter struct {
	IgnoreAuthorID int64
	MinMessageID   int64
}
