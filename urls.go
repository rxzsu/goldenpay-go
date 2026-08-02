package goldenpay

import "fmt"

// Urls builds API URLs.
type Urls struct {
	base string
}

func NewUrls(base string) *Urls { return &Urls{base: base} }
func (u *Urls) Base() string              { return u.base }
func (u *Urls) Home() string              { return u.base + "/" }
func (u *Urls) Runner() string            { return u.base + "/runner/" }
func (u *Urls) OrdersTrade() string       { return u.base + "/orders/trade" }
func (u *Urls) OrderPage(id string) string { return u.base + "/orders/" + id + "/" }
func (u *Urls) OrdersRefund() string      { return u.base + "/orders/refund" }
func (u *Urls) LotsCalc() string          { return u.base + "/lots/calc" }
func (u *Urls) LotsHome() string          { return u.base + "/lots/" }
func (u *Urls) LotsPage(nodeID int64) string      { return fmt.Sprintf("%s/lots/%d/", u.base, nodeID) }
func (u *Urls) LotsTrade(nodeID int64) string     { return fmt.Sprintf("%s/lots/%d/trade", u.base, nodeID) }
func (u *Urls) OfferEdit(nodeID, offerID int64) string {
	return fmt.Sprintf("%s/lots/offerEdit?node=%d&offer=%d", u.base, nodeID, offerID)
}
func (u *Urls) OfferSave() string     { return u.base + "/lots/offerSave" }
func (u *Urls) LotsRaise() string     { return u.base + "/yopt/lots/raise" }
func (u *Urls) ChatUpload() string    { return u.base + "/chat/upload" }
func (u *Urls) ReviewReply() string   { return u.base + "/orders/reviewReply" }
func (u *Urls) Profile(userID int64) string  { return fmt.Sprintf("%s/users/%d/", u.base, userID) }
func (u *Urls) Withdraw() string      { return u.base + "/withdraw" }
