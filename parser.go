package goldenpay

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

var reDigits = regexp.MustCompile(`\d+`)
var rePrice = regexp.MustCompile(`[\d.,]+`)
var reUserID = regexp.MustCompile(`/users/(\d+)/`)
var reOfferID = regexp.MustCompile(`data-offer="(\d+)"`)

// ---------------------------------------------------------------------------
// User / Auth
// ---------------------------------------------------------------------------

func parseUserFromHome(htmlContent, phpsessid string) (*UserInfo, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, wrapError(ErrParse, "parse home page", err)
	}

	var bodyAttrs map[string]string
	var findBody func(*html.Node)
	findBody = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "body" {
			for _, a := range n.Attr {
				if a.Key == "data-app-data" {
					bodyAttrs = parseAppDataJSON(a.Val)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findBody(c)
		}
	}
	findBody(doc)

	if bodyAttrs == nil {
		return nil, newError(ErrUnauthorized, "missing app data")
	}

	userID, _ := strconv.ParseInt(bodyAttrs["userId"], 10, 64)
	csrfToken := bodyAttrs["csrf-token"]
	if userID == 0 || csrfToken == "" {
		return nil, newError(ErrUnauthorized, "incomplete app data")
	}

	username := walkClassText(doc, "user-link-name")
	return &UserInfo{ID: userID, Username: strings.TrimSpace(username), CSRFToken: csrfToken, PHPSessID: phpsessid}, nil
}

func parseAppDataJSON(s string) map[string]string {
	m := make(map[string]string)
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "{}")
	for _, pair := range strings.Split(s, ",") {
		kv := strings.SplitN(pair, ":", 2)
		if len(kv) == 2 {
			key := strings.Trim(strings.TrimSpace(kv[0]), `"`)
			val := strings.Trim(strings.TrimSpace(kv[1]), `"`)
			m[key] = val
		}
	}
	return m
}

func parseBalance(htmlContent string) (float64, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return 0, wrapError(ErrParse, "parse balance", err)
	}
	text := walkClassText(doc, "badge-balance")
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, newError(ErrParse, "balance element not found")
	}
	// Remove currency symbols like ₽ $
	for _, ch := range []string{"₽", "$", "€", " ", "\u00a0"} {
		text = strings.ReplaceAll(text, ch, "")
	}
	text = strings.ReplaceAll(text, ",", ".")
	val, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, wrapError(ErrParse, "parse balance float", err)
	}
	return val, nil
}

// ---------------------------------------------------------------------------
// Orders
// ---------------------------------------------------------------------------

func parseOrders(htmlContent string, sellerID int64) ([]OrderInfo, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, wrapError(ErrParse, "parse orders", err)
	}

	var orders []OrderInfo
	walkNodes(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "tc-item") {
			if o := parseOrderItem(n, sellerID); o != nil {
				orders = append(orders, *o)
			}
		}
	})
	if orders == nil {
		return []OrderInfo{}, nil
	}
	return orders, nil
}

func parseOrderItem(n *html.Node, sellerID int64) *OrderInfo {
	href := attrValue(n, "href")
	id := strings.TrimPrefix(href, "/orders/")
	id = strings.TrimSuffix(id, "/")

	status := OrderPaid
	// Check for status indicators
	if hasClass(n, "tc-item--closed") || nodeContains(n, "order-closed") {
		status = OrderClosed
	}

	var buyerUsername string
	var buyerID int64
	var desc string
	var subcat string
	var amount int32

	walkNodes(n, func(c *html.Node) {
		if c.Type == html.ElementNode {
			if hasClass(c, "media-user-link") {
				buyerUsername = extractText(c)
				if m := reUserID.FindStringSubmatch(attrValue(c, "href")); len(m) > 1 {
					buyerID, _ = strconv.ParseInt(m[1], 10, 64)
				}
			}
			if hasClass(c, "tc-order-desc") {
				desc = extractText(c)
			}
			if hasClass(c, "tc-order-category") {
				subcat = extractText(c)
			}
			if hasClass(c, "tc-order-amount") {
				text := extractText(c)
				text = strings.TrimSpace(strings.ReplaceAll(text, "x", ""))
				if a, err := strconv.ParseInt(text, 10, 32); err == nil {
					amount = int32(a)
				}
			}
		}
	})

	chatID := buildChatID(sellerID, buyerID)

	return &OrderInfo{
		ID:              id,
		BuyerUsername:   strings.TrimSpace(buyerUsername),
		BuyerID:         buyerID,
		ChatID:          chatID,
		Description:     strings.TrimSpace(desc),
		SubcategoryName: strings.TrimSpace(subcat),
		Amount:          amount,
		Status:          status,
	}
}

func parseOrderPage(htmlContent, orderID string) (*OrderPage, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, wrapError(ErrParse, "parse order page", err)
	}

	p := &OrderPage{ID: orderID, RawHTML: htmlContent}

	// Status
	walkNodes(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "span" && hasClass(n, "badge") {
			text := strings.TrimSpace(extractText(n))
			switch {
			case strings.Contains(text, "paid") || strings.Contains(text, "Оплачен"):
				p.Status = OrderPaid
			case strings.Contains(text, "closed") || strings.Contains(text, "Закрыт"):
				p.Status = OrderClosed
			default:
				p.Status = OrderStatus(text)
			}
		}
	})

	// Amount
	walkNodes(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && hasClass(n, "order-amount") {
			text := reDigits.FindString(extractText(n))
			if a, err := strconv.ParseInt(text, 10, 32); err == nil {
				p.Amount = int32(a)
			}
		}
	})

	// Sum & Currency
	walkNodes(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && hasClass(n, "order-sum") {
			text := extractText(n)
			if m := rePrice.FindString(text); m != "" {
				m = strings.ReplaceAll(m, ",", ".")
				if s, err := strconv.ParseFloat(m, 64); err == nil {
					p.Sum = s
				}
			}
			if strings.Contains(text, "$") {
				p.Currency = "USD"
			} else if strings.Contains(text, "₽") {
				p.Currency = "RUB"
			} else {
				p.Currency = "RUB"
			}
		}
	})

	// Buyer
	walkNodes(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" && strings.Contains(attrValue(n, "class"), "user-link") {
			href := attrValue(n, "href")
			if m := reUserID.FindStringSubmatch(href); len(m) > 1 {
				p.BuyerID, _ = strconv.ParseInt(m[1], 10, 64)
			}
			p.BuyerUsername = extractText(n)
		}
	})

	// Chat link
	walkNodes(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" && strings.Contains(attrValue(n, "href"), "/chat/") {
			p.ChatID = strings.TrimPrefix(attrValue(n, "href"), "/chat/")
		}
	})

	// Params (key-value pairs like "Server: EU")
	walkNodes(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && hasClass(n, "param-item") {
			var key, val string
			walkNodes(n, func(c *html.Node) {
				if c.Type == html.ElementNode && hasClass(c, "param-item-label") {
					key = extractText(c)
				}
				if c.Type == html.ElementNode && hasClass(c, "param-item-value") {
					val = extractText(c)
				}
			})
			if key != "" {
				p.Params = append(p.Params, [2]string{strings.TrimSpace(key), strings.TrimSpace(val)})
			}
		}
	})

	// Secrets
	walkNodes(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && hasClass(n, "secret-item") {
			p.Secrets = append(p.Secrets, extractText(n))
		}
	})

	// Short description
	walkNodes(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && hasClass(n, "order-short-description") {
			p.ShortDesc = extractText(n)
		}
	})

	// Review
	walkNodes(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && hasClass(n, "review-item") {
			r := &Review{}
			walkNodes(n, func(c *html.Node) {
				if c.Type == html.ElementNode && hasClass(c, "review-stars") {
					text := reDigits.FindString(extractText(c))
					if s, err := strconv.Atoi(text); err == nil {
						r.Stars = s
					}
				}
				if c.Type == html.ElementNode && hasClass(c, "review-text") {
					r.Text = extractText(c)
				}
			})
			p.Review = r
		}
	})

	if p.Status == "" {
		p.Status = OrderPaid
	}

	return p, nil
}

// ---------------------------------------------------------------------------
// Offers (own)
// ---------------------------------------------------------------------------

func parseMyOffers(htmlContent string, nodeID int64) ([]Offer, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, wrapError(ErrParse, "parse my offers", err)
	}

	var offers []Offer
	walkNodes(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "tc-item") {
			if attrValue(n, "data-offer") == "" {
				return
			}
			oid, _ := strconv.ParseInt(attrValue(n, "data-offer"), 10, 64)
			if oid == 0 {
				return
			}
			o := Offer{ID: oid, NodeID: nodeID, Active: !hasClass(n, "tc-item--inactive")}
			walkNodes(n, func(c *html.Node) {
				if c.Type == html.ElementNode {
					if hasClass(c, "tc-desc-text") {
						o.Description = strings.TrimSpace(extractText(c))
					}
					if hasClass(c, "tc-price") {
						text := extractText(c)
						if m := rePrice.FindString(text); m != "" {
							m = strings.ReplaceAll(m, ",", ".")
							o.Price, _ = strconv.ParseFloat(m, 64)
						}
						if strings.Contains(text, "$") {
							o.Currency = "USD"
						} else {
							o.Currency = "RUB"
						}
					}
				}
			})
			offers = append(offers, o)
		}
	})
	if offers == nil {
		return []Offer{}, nil
	}
	return offers, nil
}

func parseMarketOffers(htmlContent string, nodeID int64) ([]MarketOffer, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, wrapError(ErrParse, "parse market offers", err)
	}

	var offers []MarketOffer
	walkNodes(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "tc-item") {
			if attrValue(n, "data-offer") == "" {
				return
			}
			oid, _ := strconv.ParseInt(attrValue(n, "data-offer"), 10, 64)
			if oid == 0 {
				return
			}
			o := MarketOffer{ID: oid, NodeID: nodeID}

			walkNodes(n, func(c *html.Node) {
				if c.Type == html.ElementNode {
					if hasClass(c, "tc-desc-text") {
						o.Description = strings.TrimSpace(extractText(c))
					}
					if hasClass(c, "tc-price") {
						text := extractText(c)
						if m := rePrice.FindString(text); m != "" {
							m = strings.ReplaceAll(m, ",", ".")
							o.Price, _ = strconv.ParseFloat(m, 64)
						}
						if strings.Contains(text, "$") {
							o.Currency = "USD"
						} else {
							o.Currency = "RUB"
						}
					}
					if hasClass(c, "tc-user") {
						o.SellerName = extractText(c)
						o.SellerOnline = nodeContains(c, "online")
						if m := reUserID.FindStringSubmatch(attrValue(c, "href")); len(m) > 1 {
							o.SellerID, _ = strconv.ParseInt(m[1], 10, 64)
						}
					}
					if hasClass(c, "rating-mini") {
						ratingText := strings.ReplaceAll(extractText(c), ",", ".")
						o.SellerRating, _ = strconv.ParseFloat(ratingText, 64)
					}
				}
			})
			offers = append(offers, o)
		}
	})
	if offers == nil {
		return []MarketOffer{}, nil
	}
	return offers, nil
}

func parseOfferDetails(htmlContent string, offerID, nodeID int64) (*OfferDetails, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, wrapError(ErrParse, "parse offer details", err)
	}

	edit := OfferEdit{}
	customFields := extractCustomFields(doc)

	// Standard fields
	if v := extractInputValue(doc, "fields[quantity]"); v != "" { edit.Quantity = &v }
	if v := extractInputValue(doc, "fields[quantity2]"); v != "" { edit.Quantity2 = &v }
	if v := extractSelectValue(doc, "fields[method]"); v != "" { edit.Method = &v }
	if v := extractSelectValue(doc, "fields[type]"); v != "" { edit.OfferType = &v }
	if v := extractInputValue(doc, "server_id"); v != "" { edit.ServerID = &v }
	if v := extractInputValue(doc, "location"); v != "" { edit.Location = &v }
	if v := extractInputValue(doc, "price"); v != "" { edit.Price = &v }
	if v := extractTextareaValue(doc, "fields[desc][ru]"); v != "" { edit.DescriptionRU = &v }
	if v := extractTextareaValue(doc, "fields[desc][en]"); v != "" { edit.DescriptionEN = &v }
	if v := extractTextareaValue(doc, "fields[payment_msg][ru]"); v != "" { edit.PaymentMsgRU = &v }
	if v := extractTextareaValue(doc, "fields[payment_msg][en]"); v != "" { edit.PaymentMsgEN = &v }
	if v := extractInputValue(doc, "fields[summary][ru]"); v != "" { edit.SummaryRU = &v }
	if v := extractInputValue(doc, "fields[summary][en]"); v != "" { edit.SummaryEN = &v }
	if v := extractInputValue(doc, "fields[game]"); v != "" { edit.Game = &v }
	if v := extractInputValue(doc, "fields[images]"); v != "" { edit.Images = &v }

	return &OfferDetails{Current: edit, CustomFields: customFields}, nil
}

// ---------------------------------------------------------------------------
// Chat messages (from runner JSON)
// ---------------------------------------------------------------------------

func parseChatMessages(chatID, jsonContent string) ([]ChatMessage, error) {
	var resp struct {
		Objects []struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		} `json:"objects"`
	}
	if err := json.Unmarshal([]byte(jsonContent), &resp); err != nil {
		return nil, wrapError(ErrParse, "parse chat messages JSON", err)
	}

	var messages []ChatMessage
	for _, obj := range resp.Objects {
		if obj.Type != "chat_node" {
			continue
		}
		var chatData struct {
			Messages []ChatMessage `json:"messages"`
		}
		if err := json.Unmarshal(obj.Data, &chatData); err != nil {
			continue
		}
		for _, msg := range chatData.Messages {
			msg.ChatID = chatID
			messages = append(messages, msg)
		}
	}
	if messages == nil {
		return []ChatMessage{}, nil
	}
	return messages, nil
}

// ---------------------------------------------------------------------------
// Categories
// ---------------------------------------------------------------------------

func parseCategorySubcategories(htmlContent string) ([]CategorySubcategory, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, wrapError(ErrParse, "parse subcategories", err)
	}
	var subs []CategorySubcategory
	walkNodes(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "counter-item") {
			sub := CategorySubcategory{}
			sub.Name = extractText(n)
			if idTxt := reDigits.FindString(attrValue(n, "href")); idTxt != "" {
				sub.ID, _ = strconv.ParseInt(idTxt, 10, 64)
			}
			if hasClass(n, "active") {
				sub.IsActive = true
			}
			subs = append(subs, sub)
		}
	})
	if subs == nil {
		return []CategorySubcategory{}, nil
	}
	return subs, nil
}

func parseCategoryFilters(htmlContent string) ([]CategoryFilter, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, wrapError(ErrParse, "parse filters", err)
	}
	var filters []CategoryFilter
	walkNodes(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && hasClass(n, "lot-field") {
			f := CategoryFilter{Options: []CategoryFilterOption{}}
			walkNodes(n, func(c *html.Node) {
				if c.Type == html.ElementNode {
					if c.Data == "label" {
						f.Name = extractText(c)
					}
					if c.Data == "select" {
						f.FilterType = "select"
						f.ID = attrValue(c, "name")
						walkNodes(c, func(opt *html.Node) {
							if opt.Type == html.ElementNode && opt.Data == "option" {
								f.Options = append(f.Options, CategoryFilterOption{
									Value: attrValue(opt, "value"),
									Label: extractText(opt),
								})
							}
						})
					}
				}
			})
			if f.Name != "" {
				filters = append(filters, f)
			}
		}
	})
	if filters == nil {
		return []CategoryFilter{}, nil
	}
	return filters, nil
}

func parseCategoryTree(htmlContent string) ([]CategoryNode, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, wrapError(ErrParse, "parse category tree", err)
	}
	var nodes []CategoryNode
	walkNodes(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "cat-item") {
			if cn := buildCategoryNode(n); cn != nil {
				nodes = append(nodes, *cn)
			}
		}
	})
	if nodes == nil {
		return []CategoryNode{}, nil
	}
	return nodes, nil
}

func buildCategoryNode(n *html.Node) *CategoryNode {
	cn := &CategoryNode{}
	cn.Name = extractText(n)
	if idTxt := reDigits.FindString(attrValue(n, "href")); idTxt != "" {
		cn.ID, _ = strconv.ParseInt(idTxt, 10, 64)
	}
	// Parse children from sub-menu if present
	walkNodes(n, func(c *html.Node) {
		if c.Type == html.ElementNode && c.Data == "a" && hasClass(c, "cat-item") {
			if child := buildCategoryNode(c); child != nil {
				cn.Children = append(cn.Children, *child)
			}
		}
	})
	if cn.Children == nil {
		cn.Children = []CategoryNode{}
	}
	return cn
}

// ---------------------------------------------------------------------------
// Profile / Reviews
// ---------------------------------------------------------------------------

func parseProfileReviews(htmlContent string) ([]ProfileReview, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, wrapError(ErrParse, "parse profile reviews", err)
	}
	var reviews []ProfileReview
	walkNodes(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && hasClass(n, "review-item") {
			r := ProfileReview{}
			walkNodes(n, func(c *html.Node) {
				if c.Type == html.ElementNode {
					if hasClass(c, "review-author") {
						r.BuyerUsername = extractText(c)
						if m := reUserID.FindStringSubmatch(attrValue(c, "href")); len(m) > 1 {
							r.BuyerID, _ = strconv.ParseInt(m[1], 10, 64)
						}
					}
					if hasClass(c, "review-stars") {
						if s, err := strconv.Atoi(reDigits.FindString(extractText(c))); err == nil {
							r.Stars = s
						}
					}
					if hasClass(c, "review-text") {
						r.Text = extractText(c)
					}
				}
			})
			reviews = append(reviews, r)
		}
	})
	if reviews == nil {
		return []ProfileReview{}, nil
	}
	return reviews, nil
}

// ---------------------------------------------------------------------------
// HTML helpers
// ---------------------------------------------------------------------------

func walkNodes(n *html.Node, fn func(*html.Node)) {
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkNodes(c, fn)
	}
}

func hasClass(n *html.Node, class string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" {
			classes := strings.Fields(a.Val)
			for _, c := range classes {
				if c == class {
					return true
				}
			}
		}
	}
	return false
}

func attrValue(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func extractText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(c *html.Node) {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
		}
		for ch := c.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(n)
	return b.String()
}

func walkClassText(doc *html.Node, class string) string {
	var result string
	walkNodes(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && hasClass(n, class) {
			result = extractText(n)
		}
	})
	return strings.TrimSpace(result)
}

func nodeContains(n *html.Node, class string) bool {
	found := false
	walkNodes(n, func(c *html.Node) {
		if hasClass(c, class) {
			found = true
		}
	})
	return found
}

func extractInputValue(doc *html.Node, name string) string {
	var val string
	walkNodes(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "input" && attrValue(n, "name") == name {
			val = attrValue(n, "value")
		}
	})
	return val
}

func extractTextareaValue(doc *html.Node, name string) string {
	var val string
	walkNodes(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "textarea" && attrValue(n, "name") == name {
			val = extractText(n)
		}
	})
	return val
}

func extractSelectValue(doc *html.Node, name string) string {
	var val string
	walkNodes(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "select" && attrValue(n, "name") == name {
			walkNodes(n, func(c *html.Node) {
				if c.Type == html.ElementNode && c.Data == "option" && attrValue(c, "selected") != "" {
					val = attrValue(c, "value")
				}
			})
		}
	})
	return val
}

func extractCustomFields(doc *html.Node) []OfferField {
	var fields []OfferField
	walkNodes(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && hasClass(n, "form-group") {
			f := OfferField{}
			walkNodes(n, func(c *html.Node) {
				if c.Type == html.ElementNode {
					if c.Data == "label" {
						f.Label = extractText(c)
					}
					if c.Data == "input" && attrValue(c, "type") != "hidden" {
						f.FieldType = "input"
						f.Name = attrValue(c, "name")
						f.Value = attrValue(c, "value")
						if attrValue(c, "required") != "" {
							f.Required = true
						}
					}
					if c.Data == "textarea" {
						f.FieldType = "textarea"
						f.Name = attrValue(c, "name")
						f.Value = extractText(c)
					}
					if c.Data == "select" {
						f.FieldType = "select"
						f.Name = attrValue(c, "name")
						walkNodes(c, func(opt *html.Node) {
							if opt.Type == html.ElementNode && opt.Data == "option" && attrValue(opt, "selected") != "" {
								f.Value = attrValue(opt, "value")
							}
						})
					}
				}
			})
			if f.Name != "" {
				fields = append(fields, f)
			}
		}
	})
	return fields
}

func buildChatID(sellerID, buyerID int64) string {
	if sellerID < buyerID {
		return fmt.Sprintf("users-%d-%d", sellerID, buyerID)
	}
	return fmt.Sprintf("users-%d-%d", buyerID, sellerID)
}

