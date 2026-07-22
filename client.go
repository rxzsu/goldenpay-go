package goldenpay

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GoldenPay is the reusable HTTP client.
type GoldenPay struct {
	http   *http.Client
	config *GoldenPayConfig
	urls   *Urls
}

// GoldenPaySession is an authenticated session.
type GoldenPaySession struct {
	client *GoldenPay
	user   *UserInfo
}

func New(config *GoldenPayConfig) (*GoldenPay, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	transport := &http.Transport{}
	if config.Proxy != "" {
		proxyURL, err := url.Parse(config.Proxy)
		if err != nil {
			return nil, wrapError(ErrHTTP, "invalid proxy URL", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	return &GoldenPay{
		http: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		config: config,
		urls:   NewUrls(config.BaseURL),
	}, nil
}

// Connect authenticates via golden_key cookie and parses UserInfo.
func (c *GoldenPay) Connect() (*GoldenPaySession, error) {
	req, err := http.NewRequest("GET", c.urls.Home(), nil)
	if err != nil {
		return nil, wrapError(ErrHTTP, "connect request", err)
	}
	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("Cookie", "golden_key="+c.config.GoldenKey+"; cookie_prefs=1")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, wrapError(ErrHTTP, "connect", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return nil, newError(ErrUnauthorized, "invalid golden key")
	}

	body, _ := io.ReadAll(resp.Body)

	var phpsessid string
	for _, ck := range resp.Cookies() {
		if ck.Name == "PHPSESSID" {
			phpsessid = ck.Value
			break
		}
	}

	user, err := parseUserFromHome(string(body), phpsessid)
	if err != nil {
		return nil, err
	}

	return &GoldenPaySession{client: c, user: user}, nil
}

// ValidateProxy checks if the proxy (if configured) is reachable.
func (c *GoldenPay) ValidateProxy() (bool, error) {
	_, err := c.http.Get(c.urls.Home())
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *GoldenPaySession) User() *UserInfo             { return s.user }
func (s *GoldenPaySession) Config() *GoldenPayConfig     { return s.client.config }
func (s *GoldenPaySession) SetGoldenKey(key string)     { s.client.config.GoldenKey = key }

// CheckConnection pings the home page.
func (s *GoldenPaySession) CheckConnection() bool {
	_, err := s.getHTML(s.client.urls.Home())
	return err == nil
}

// Ping sends a runner heartbeat.
func (s *GoldenPaySession) Ping() (*RunnerResponse, error) {
	body, err := s.postForm(s.client.urls.Runner(), s.runnerPayload(map[string]string{}),
		s.client.urls.Home(), "application/json")
	if err != nil {
		return nil, err
	}
	return parseRunnerResponse(body)
}

// ---------------------------------------------------------------------------
// Orders
// ---------------------------------------------------------------------------

// FetchOrders returns all orders from the trade page.
func (s *GoldenPaySession) FetchOrders() ([]OrderInfo, error) {
	html, err := s.getHTML(s.client.urls.OrdersTrade())
	if err != nil {
		return nil, err
	}
	return parseOrders(html, s.user.ID)
}

// FetchPaidOrders returns only paid orders.
func (s *GoldenPaySession) FetchPaidOrders() ([]OrderInfo, error) {
	all, err := s.FetchOrders()
	if err != nil {
		return nil, err
	}
	var paid []OrderInfo
	for _, o := range all {
		if o.Status == OrderPaid {
			paid = append(paid, o)
		}
	}
	return paid, nil
}

// FetchOrdersWith applies client-side filters.
func (s *GoldenPaySession) FetchOrdersWith(opts *FetchOrderOptions) ([]OrderInfo, error) {
	all, err := s.FetchOrders()
	if err != nil {
		return nil, err
	}
	if opts == nil {
		return all, nil
	}
	var filtered []OrderInfo
	for _, o := range all {
		if opts.Status != nil && o.Status != *opts.Status {
			continue
		}
		if opts.MinAmount != nil && o.Amount < *opts.MinAmount {
			continue
		}
		if opts.MaxAmount != nil && o.Amount > *opts.MaxAmount {
			continue
		}
		if opts.Subcategory != nil && !strings.Contains(strings.ToLower(o.SubcategoryName), strings.ToLower(*opts.Subcategory)) {
			continue
		}
		if opts.BuyerID != nil && o.BuyerID != *opts.BuyerID {
			continue
		}
		if opts.BuyerUsername != nil && !strings.Contains(strings.ToLower(o.BuyerUsername), strings.ToLower(*opts.BuyerUsername)) {
			continue
		}
		if opts.Description != nil && !strings.Contains(strings.ToLower(o.Description), strings.ToLower(*opts.Description)) {
			continue
		}
		filtered = append(filtered, o)
	}
	return filtered, nil
}

// FetchOrderPage returns full order details.
func (s *GoldenPaySession) FetchOrderPage(orderID string) (*OrderPage, error) {
	html, err := s.getHTML(s.client.urls.OrderPage(orderID))
	if err != nil {
		return nil, err
	}
	return parseOrderPage(html, orderID)
}

// CalculateStatistics computes totals from filtered orders.
func (s *GoldenPaySession) CalculateStatistics(opts *FetchOrderOptions) (*StoreStatistics, error) {
	orders, err := s.FetchOrdersWith(opts)
	if err != nil {
		return nil, err
	}
	stats := &StoreStatistics{}
	seen := make(map[int64]string)
	for _, o := range orders {
		stats.OrderCount++
		if _, exists := seen[o.BuyerID]; !exists {
			seen[o.BuyerID] = o.BuyerUsername
			stats.UniqueBuyers++
			stats.BuyerUsernames = append(stats.BuyerUsernames, o.BuyerUsername)
		}
	}
	return stats, nil
}

// ---------------------------------------------------------------------------
// Chat & Messages
// ---------------------------------------------------------------------------

// SendMessage sends a text message to a chat.
func (s *GoldenPaySession) SendMessage(chatID, text string) (*RunnerResponse, error) {
	payload := s.runnerPayload(map[string]string{
		"action": "chat_message",
		"chat_id": chatID,
		"text":    text,
	})
	body, err := s.postForm(s.client.urls.Runner(), payload, s.client.urls.Home(), "application/json")
	if err != nil {
		return nil, err
	}
	return parseRunnerResponse(body)
}

// FetchChatMessages fetches messages from a chat.
func (s *GoldenPaySession) FetchChatMessages(chatID string) ([]ChatMessage, error) {
	payload := s.runnerPayload(map[string]string{
		"request": "false",
		"chat_id": chatID,
	})
	body, err := s.postForm(s.client.urls.Runner(), payload, s.client.urls.Home(), "application/json")
	if err != nil {
		return nil, err
	}
	return parseChatMessages(chatID, body)
}

// ---------------------------------------------------------------------------
// Offers (own)
// ---------------------------------------------------------------------------

// FetchMyOffers returns the seller's own offers for a category node.
func (s *GoldenPaySession) FetchMyOffers(nodeID int64) ([]Offer, error) {
	html, err := s.getHTML(s.client.urls.LotsTrade(nodeID))
	if err != nil {
		return nil, err
	}
	return parseMyOffers(html, nodeID)
}

// FetchOfferDetails returns current field values for editing.
func (s *GoldenPaySession) FetchOfferDetails(nodeID, offerID int64) (*OfferDetails, error) {
	html, err := s.getHTML(s.client.urls.OfferEdit(nodeID, offerID))
	if err != nil {
		return nil, err
	}
	return parseOfferDetails(html, offerID, nodeID)
}

// EditOffer patches an existing offer.
func (s *GoldenPaySession) EditOffer(nodeID, offerID int64, patch OfferEdit) (*OfferSaveResponse, error) {
	current, err := s.FetchOfferDetails(nodeID, offerID)
	if err != nil {
		return nil, err
	}
	merged := current.Current.Merge(patch)
	return s.saveOffer(offerID, nodeID, &merged, s.client.urls.OfferEdit(nodeID, offerID))
}

// CreateOffer creates a new offer.
func (s *GoldenPaySession) CreateOffer(nodeID int64, details OfferEdit) (*OfferSaveResponse, error) {
	return s.saveOffer(0, nodeID, &details, s.client.urls.LotsTrade(nodeID))
}

// DeactivateAllOffers deactivates all active offers in a category.
func (s *GoldenPaySession) DeactivateAllOffers(nodeID int64) error {
	offers, err := s.FetchMyOffers(nodeID)
	if err != nil {
		return err
	}
	falseVal := false
	for _, o := range offers {
		if o.Active {
			if _, err := s.EditOffer(nodeID, o.ID, OfferEdit{Active: &falseVal}); err != nil {
				return err
			}
		}
	}
	return nil
}

// DeleteAllOffers deletes all offers in a category.
func (s *GoldenPaySession) DeleteAllOffers(nodeID int64) error {
	offers, err := s.FetchMyOffers(nodeID)
	if err != nil {
		return err
	}
	trueVal := true
	for _, o := range offers {
		if _, err := s.EditOffer(nodeID, o.ID, OfferEdit{Deleted: &trueVal}); err != nil {
			return err
		}
	}
	return nil
}

// UndercutPrice sets offer price to lowest competitor minus undercut_by.
func (s *GoldenPaySession) UndercutPrice(nodeID, offerID int64, undercutBy, minPrice float64) (*OfferSaveResponse, error) {
	market, err := s.FetchMarketOffers(nodeID)
	if err != nil {
		return nil, err
	}
	var lowest float64
	for i, o := range market {
		if i == 0 || o.Price < lowest {
			lowest = o.Price
		}
	}
	newPrice := lowest - undercutBy
	if newPrice < minPrice {
		newPrice = minPrice
	}
	priceStr := fmt.Sprintf("%.2f", newPrice)
	return s.EditOffer(nodeID, offerID, OfferEdit{Price: &priceStr})
}

// ---------------------------------------------------------------------------
// Market
// ---------------------------------------------------------------------------

// FetchMarketOffers returns public offers (competitors) on the market page.
func (s *GoldenPaySession) FetchMarketOffers(nodeID int64) ([]MarketOffer, error) {
	html, err := s.getHTML(s.client.urls.LotsPage(nodeID))
	if err != nil {
		return nil, err
	}
	return parseMarketOffers(html, nodeID)
}

// CalcPrice calculates buyer/seller prices and commission.
func (s *GoldenPaySession) CalcPrice(nodeID int64, price float64) (*PriceCalculation, error) {
	payload := url.Values{}
	payload.Set("node", fmt.Sprintf("%d", nodeID))
	payload.Set("price", fmt.Sprintf("%.2f", price))
	body, err := s.postForm(s.client.urls.LotsCalc(), payload.Encode(),
		s.client.urls.LotsPage(nodeID), "application/json")
	if err != nil {
		return nil, err
	}
	return parsePriceCalculation(body, price)
}

// ---------------------------------------------------------------------------
// Categories
// ---------------------------------------------------------------------------

// FetchCategorySubcategories returns subcategory pills from a category page.
func (s *GoldenPaySession) FetchCategorySubcategories(nodeID int64) ([]CategorySubcategory, error) {
	html, err := s.getHTML(s.client.urls.LotsPage(nodeID))
	if err != nil {
		return nil, err
	}
	return parseCategorySubcategories(html)
}

// FetchCategoryFilters returns filter controls from a category page.
func (s *GoldenPaySession) FetchCategoryFilters(nodeID int64) ([]CategoryFilter, error) {
	html, err := s.getHTML(s.client.urls.LotsPage(nodeID))
	if err != nil {
		return nil, err
	}
	return parseCategoryFilters(html)
}

// FetchCategoryTree returns the full marketplace category tree.
func (s *GoldenPaySession) FetchCategoryTree() ([]CategoryNode, error) {
	html, err := s.getHTML(s.client.urls.LotsHome())
	if err != nil {
		return nil, err
	}
	return parseCategoryTree(html)
}

// ---------------------------------------------------------------------------
// Account
// ---------------------------------------------------------------------------

// FetchBalance returns the current account balance.
func (s *GoldenPaySession) FetchBalance() (float64, error) {
	html, err := s.getHTML(s.client.urls.Home())
	if err != nil {
		return 0, err
	}
	return parseBalance(html)
}

// RaiseOffers raises all offers in a category.
func (s *GoldenPaySession) RaiseOffers(nodeID int64) (*RaiseOffersResponse, error) {
	payload := url.Values{}
	payload.Set("node", fmt.Sprintf("%d", nodeID))
	body, err := s.postForm(s.client.urls.LotsRaise(), payload.Encode(),
		s.client.urls.LotsPage(nodeID), "application/json")
	if err != nil {
		return nil, err
	}
	return parseRaiseResponse(body)
}

// ReplyToReview replies to an order review.
func (s *GoldenPaySession) ReplyToReview(orderID, text string) (*RunnerResponse, error) {
	payload := url.Values{}
	payload.Set("order_id", orderID)
	payload.Set("text", text)
	body, err := s.postForm(s.client.urls.ReviewReply(), payload.Encode(),
		s.client.urls.OrderPage(orderID), "application/json")
	if err != nil {
		return nil, err
	}
	return parseRunnerResponse(body)
}

// FetchProfileReviews returns reviews from a user profile.
func (s *GoldenPaySession) FetchProfileReviews(userID int64) ([]ProfileReview, error) {
	html, err := s.getHTML(s.client.urls.Profile(userID))
	if err != nil {
		return nil, err
	}
	return parseProfileReviews(html)
}

// Withdraw initiates a payout.
func (s *GoldenPaySession) Withdraw(req *WithdrawRequest) (*RunnerResponse, error) {
	payload := url.Values{}
	payload.Set("currency", req.Currency)
	payload.Set("ext_currency", req.ExtCurrency)
	payload.Set("wallet", req.Wallet)
	payload.Set("amount", fmt.Sprintf("%.2f", req.Amount))
	body, err := s.postForm(s.client.urls.Withdraw(), payload.Encode(),
		s.client.urls.Home(), "application/json")
	if err != nil {
		return nil, err
	}
	return parseRunnerResponse(body)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (s *GoldenPaySession) cookieHeader() string {
	cookie := "golden_key=" + s.client.config.GoldenKey + "; cookie_prefs=1"
	if s.user.PHPSessID != "" {
		cookie += "; PHPSESSID=" + s.user.PHPSessID
	}
	return cookie
}

func (s *GoldenPaySession) getHTML(url string) (string, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", s.client.config.UserAgent)
	req.Header.Set("Cookie", s.cookieHeader())
	req.Header.Set("Accept", "*/*")

	resp, err := s.client.http.Do(req)
	if err != nil {
		return "", wrapError(ErrHTTP, "get", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return "", newError(ErrUnauthorized, "session expired")
	}

	body, _ := io.ReadAll(resp.Body)
	return string(body), nil
}

func (s *GoldenPaySession) postForm(url, payload, referer, accept string) (string, error) {
	req, _ := http.NewRequest("POST", url, strings.NewReader(payload))
	req.Header.Set("User-Agent", s.client.config.UserAgent)
	req.Header.Set("Cookie", s.cookieHeader())
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Accept", accept)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	resp, err := s.client.http.Do(req)
	if err != nil {
		return "", wrapError(ErrHTTP, "post", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return string(body), nil
}

func (s *GoldenPaySession) saveOffer(offerID, nodeID int64, edit *OfferEdit, referer string) (*OfferSaveResponse, error) {
	body, err := s.postForm(s.client.urls.OfferSave(), buildOfferPayload(edit, s.user.CSRFToken, offerID, nodeID),
		referer, "application/json, text/javascript, */*; q=0.01")
	if err != nil {
		return nil, err
	}
	return parseOfferSaveResponse(body)
}

func (s *GoldenPaySession) runnerPayload(extra map[string]string) string {
	v := url.Values{}
	v.Set("action", "chat_message")
	for k, val := range extra {
		v.Set(k, val)
	}
	return v.Encode()
}

// buildOfferPayload builds form-encoded body for offerSave.
func buildOfferPayload(edit *OfferEdit, csrfToken string, offerID, nodeID int64) string {
	v := url.Values{}
	v.Set("csrf_token", csrfToken)
	v.Set("offer_id", fmt.Sprintf("%d", offerID))
	v.Set("node_id", fmt.Sprintf("%d", nodeID))
	setIf := func(key string, val *string) {
		if val != nil { v.Set(key, *val) }
	}
	setIf("location", edit.Location)
	setIf("fields[quantity]", edit.Quantity)
	setIf("fields[quantity2]", edit.Quantity2)
	setIf("fields[method]", edit.Method)
	setIf("fields[type]", edit.OfferType)
	setIf("server_id", edit.ServerID)
	setIf("fields[desc][ru]", edit.DescriptionRU)
	setIf("fields[desc][en]", edit.DescriptionEN)
	setIf("fields[payment_msg][ru]", edit.PaymentMsgRU)
	setIf("fields[payment_msg][en]", edit.PaymentMsgEN)
	setIf("fields[summary][ru]", edit.SummaryRU)
	setIf("fields[summary][en]", edit.SummaryEN)
	setIf("fields[game]", edit.Game)
	setIf("fields[images]", edit.Images)
	setIf("price", edit.Price)
	if edit.DeactivateAfterSale != nil && *edit.DeactivateAfterSale {
		v.Set("deactivate_after_sale[]", "on")
	}
	if edit.Active != nil && *edit.Active {
		v.Set("active", "on")
	}
	if edit.Deleted != nil && *edit.Deleted {
		v.Set("deleted", "1")
	}
	return v.Encode()
}

// parseRunnerResponse parses the runner JSON response.
func parseRunnerResponse(raw string) (*RunnerResponse, error) {
	var resp struct {
		Success       bool              `json:"success"`
		ErrorMessage  string            `json:"error_message"`
		Objects       []json.RawMessage `json:"objects"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, wrapError(ErrParse, "runner response", err)
	}
	r := &RunnerResponse{
		Success:      resp.Success,
		ErrorMessage: resp.ErrorMessage,
	}
	for _, obj := range resp.Objects {
		var typed struct {
			Type string          `json:"type"`
			ID   string          `json:"id"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(obj, &typed); err != nil {
			continue
		}
		r.Objects = append(r.Objects, RunnerObject{
			Type: typed.Type,
			ID:   typed.ID,
			Data: typed.Data,
		})
	}
	return r, nil
}

// parseOfferSaveResponse parses the offer save JSON response.
func parseOfferSaveResponse(raw string) (*OfferSaveResponse, error) {
	var resp struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, wrapError(ErrParse, "offer save response", err)
	}
	return &OfferSaveResponse{Success: resp.Success, Error: resp.Error}, nil
}

// parseRaiseResponse parses the raise offers JSON response.
func parseRaiseResponse(raw string) (*RaiseOffersResponse, error) {
	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, wrapError(ErrParse, "raise response", err)
	}
	return &RaiseOffersResponse{Success: resp.Success, Message: resp.Message}, nil
}

// parsePriceCalculation parses the price calc JSON response.
func parsePriceCalculation(raw string, inputPrice float64) (*PriceCalculation, error) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, wrapError(ErrParse, "price calc", err)
	}
	pc := &PriceCalculation{
		InputPrice:    inputPrice,
		NumericFields: make(map[string]float64),
	}
	collectNumericFields("", data, pc.NumericFields)
	if v, ok := pc.NumericFields["seller_price"]; ok { pc.SellerPrice = &v }
	if v, ok := pc.NumericFields["buyer_price"]; ok { pc.BuyerPrice = &v }
	if v, ok := pc.NumericFields["commission"]; ok { pc.Commission = &v }
	return pc, nil
}

func collectNumericFields(prefix string, data map[string]interface{}, out map[string]float64) {
	for k, v := range data {
		key := k
		if prefix != "" { key = prefix + "_" + k }
		switch val := v.(type) {
		case float64:
			out[key] = val
		case map[string]interface{}:
			collectNumericFields(key, val, out)
		}
	}
}

