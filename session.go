package goldenpay

import "log"

// SessionManager wraps GoldenPaySession with auto-reconnect on auth errors.
type SessionManager struct {
	client     *GoldenPay
	session    *GoldenPaySession
	maxRetries int
}

func NewSessionManager(client *GoldenPay) *SessionManager {
	return &SessionManager{client: client, maxRetries: 3}
}

func (m *SessionManager) Start() error {
	s, err := m.client.Connect()
	if err != nil {
		return err
	}
	m.session = s
	return nil
}

func (m *SessionManager) Session() *GoldenPaySession { return m.session }

func (m *SessionManager) reconnect() error {
	log.Println("reconnecting session...")
	s, err := m.client.Connect()
	if err != nil {
		return err
	}
	m.session = s
	return nil
}

func (m *SessionManager) call(fn func() error) error {
	for attempt := 0; attempt <= m.maxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		if IsAuthError(err) && attempt < m.maxRetries {
			if rerr := m.reconnect(); rerr != nil {
				return rerr
			}
			continue
		}
		return err
	}
	return nil
}

// IsAuthError checks if the underlying error is an authentication error.
func IsAuthError(err error) bool {
	if err == nil { return false }
	e, ok := err.(*GoldenPayError)
	return ok && e.Kind == ErrUnauthorized
}

// Convenience methods for common SessionManager -> Session calls.

func (m *SessionManager) FetchOrders() ([]OrderInfo, error) {
	var orders []OrderInfo
	err := m.call(func() error {
		var e error
		orders, e = m.session.FetchOrders()
		return e
	})
	return orders, err
}

func (m *SessionManager) FetchPaidOrders() ([]OrderInfo, error) {
	var orders []OrderInfo
	err := m.call(func() error {
		var e error
		orders, e = m.session.FetchPaidOrders()
		return e
	})
	return orders, err
}

func (m *SessionManager) FetchOrderPage(id string) (*OrderPage, error) {
	var page *OrderPage
	err := m.call(func() error {
		var e error
		page, e = m.session.FetchOrderPage(id)
		return e
	})
	return page, err
}

func (m *SessionManager) SendMessage(chatID, text string) (*RunnerResponse, error) {
	var resp *RunnerResponse
	err := m.call(func() error {
		var e error
		resp, e = m.session.SendMessage(chatID, text)
		return e
	})
	return resp, err
}

func (m *SessionManager) FetchChatMessages(chatID string) ([]ChatMessage, error) {
	var msgs []ChatMessage
	err := m.call(func() error {
		var e error
		msgs, e = m.session.FetchChatMessages(chatID)
		return e
	})
	return msgs, err
}

func (m *SessionManager) EditOffer(nodeID, offerID int64, patch OfferEdit) (*OfferSaveResponse, error) {
	var resp *OfferSaveResponse
	err := m.call(func() error {
		var e error
		resp, e = m.session.EditOffer(nodeID, offerID, patch)
		return e
	})
	return resp, err
}

func (m *SessionManager) FetchMyOffers(nodeID int64) ([]Offer, error) {
	var offers []Offer
	err := m.call(func() error {
		var e error
		offers, e = m.session.FetchMyOffers(nodeID)
		return e
	})
	return offers, err
}

func (m *SessionManager) Ping() (*RunnerResponse, error) {
	var resp *RunnerResponse
	err := m.call(func() error {
		var e error
		resp, e = m.session.Ping()
		return e
	})
	return resp, err
}

func (m *SessionManager) FetchBalance() (float64, error) {
	var bal float64
	err := m.call(func() error {
		var e error
		bal, e = m.session.FetchBalance()
		return e
	})
	return bal, err
}
