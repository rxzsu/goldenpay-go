package goldenpay

import (
	"context"
	"log"
	"time"
)

// BotOptions configures the bot.
type BotOptions struct {
	IgnoreOwnMessages       bool
	EmitMessagesForNewOrders bool
	AutoWelcomeMessage      string
	SleepScheduleStart      int
	SleepScheduleEnd        int
	SleepNodeOffers         [][2]int64 // (node_id, offer_id) pairs
}

func DefaultBotOptions() BotOptions {
	return BotOptions{
		IgnoreOwnMessages:        true,
		EmitMessagesForNewOrders: true,
	}
}

// GoldenPayBot polls for new orders and messages.
type GoldenPayBot struct {
	session *GoldenPaySession
	options BotOptions
	store   StateStore
	seen    *EventStream
}

// EventStream tracks seen orders/messages for dedup.
type EventStream struct {
	SeenOrders   map[string]struct{}
	SeenMessages map[string]int64
}

func NewBot(session *GoldenPaySession) *GoldenPayBot {
	return &GoldenPayBot{
		session: session,
		options: DefaultBotOptions(),
		store:   NewMemoryStateStore(),
		seen: &EventStream{
			SeenOrders:   make(map[string]struct{}),
			SeenMessages: make(map[string]int64),
		},
	}
}

func (b *GoldenPayBot) WithStore(store StateStore) *GoldenPayBot {
	b.store = store
	return b
}

func (b *GoldenPayBot) WithOptions(opts BotOptions) *GoldenPayBot {
	b.options = opts
	return b
}

func (b *GoldenPayBot) LoadState() error {
	state, err := b.store.Load()
	if err != nil { return err }
	for _, id := range state.SeenOrders {
		b.seen.SeenOrders[id] = struct{}{}
	}
	for k, v := range state.SeenMessages {
		b.seen.SeenMessages[k] = v
	}
	return nil
}

func (b *GoldenPayBot) SaveState() error {
	state := &BotState{SeenMessages: b.seen.SeenMessages}
	for id := range b.seen.SeenOrders {
		state.SeenOrders = append(state.SeenOrders, id)
	}
	return b.store.Save(state)
}

// ShouldEmitOrder returns true if the order is new (not yet seen).
func (s *EventStream) ShouldEmitOrder(id string) bool {
	if _, ok := s.SeenOrders[id]; ok { return false }
	s.SeenOrders[id] = struct{}{}
	return true
}

// ShouldEmitMessage checks if a message is new and passes the filter.
func (s *EventStream) ShouldEmitMessage(msg ChatMessage, filter *MessageFilter) bool {
	if filter != nil && filter.IgnoreAuthorID != 0 && msg.AuthorID == filter.IgnoreAuthorID {
		return false
	}
	lastID, ok := s.SeenMessages[msg.ChatID]
	if ok && msg.ID <= lastID { return false }
	s.SeenMessages[msg.ChatID] = msg.ID
	return true
}

// Bootstrap fetches all current orders and messages to seed seen state.
func (b *GoldenPayBot) Bootstrap() error {
	orders, err := b.session.FetchOrders()
	if err != nil { return err }
	for _, o := range orders {
		b.seen.SeenOrders[o.ID] = struct{}{}
		if o.Status == OrderPaid && o.ChatID != "" {
			messages, err := b.session.FetchChatMessages(o.ChatID)
			if err != nil { continue }
			for _, m := range messages {
				if m.ID > b.seen.SeenMessages[o.ChatID] {
					b.seen.SeenMessages[o.ChatID] = m.ID
				}
			}
		}
	}
	return b.SaveState()
}

func (b *GoldenPayBot) Run(ctx context.Context, handler func(GoldenPayEvent) error) error {
	interval := b.session.Config().PollInterval
	sleepState := -1

	if err := b.LoadState(); err != nil {
		log.Printf("failed to load state: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			b.SaveState()
			return nil
		default:
		}

		// Sleep schedule
		if b.options.SleepNodeOffers != nil {
			now := time.Now()
			h := now.Hour()
			inWindow := b.options.SleepScheduleStart <= h && h < b.options.SleepScheduleEnd
			if b.options.SleepScheduleStart > b.options.SleepScheduleEnd {
				inWindow = h >= b.options.SleepScheduleStart || h < b.options.SleepScheduleEnd
			}
			target := 0
			if inWindow { target = 1 }
			if sleepState != target {
				sleepState = target
				active := !inWindow
				for _, pair := range b.options.SleepNodeOffers {
					if _, err := b.session.EditOffer(pair[0], pair[1], OfferEdit{Active: &active}); err != nil {
						log.Printf("sleep schedule: offer %d/%d: %v", pair[0], pair[1], err)
					}
				}
			}
		}

		events, err := b.pollOnce()
		if err != nil {
			log.Printf("poll error: %v", err)
		} else {
			for _, ev := range events {
				if err := handler(ev); err != nil {
					log.Printf("event handler error: %v", err)
				}
			}
		}

		time.Sleep(interval)
	}
}

func (b *GoldenPayBot) pollOnce() ([]GoldenPayEvent, error) {
	events := []GoldenPayEvent{}
	emitChats := []string{}
	markChats := []string{}
	filter := &MessageFilter{}
	if b.options.IgnoreOwnMessages {
		filter.IgnoreAuthorID = b.session.User().ID
	}

	// 1. Fetch orders
	orders, err := b.session.FetchOrders()
	if err != nil { return nil, err }

	for _, o := range orders {
		seen := b.seen.ShouldEmitOrder(o.ID)
		if seen {
			events = append(events, GoldenPayEvent{NewOrder: &o})
		}
		if seen && !b.options.EmitMessagesForNewOrders {
			markChats = append(markChats, o.ChatID)
		} else if o.ChatID != "" {
			emitChats = append(emitChats, o.ChatID)
		}
	}

	// 2. Fetch chat messages (concurrently)
	type chatResult struct {
		chatID   string
		messages []ChatMessage
		err      error
	}
	ch := make(chan chatResult, len(emitChats))
	for _, cid := range emitChats {
		go func(cid string) {
			msgs, err := b.session.FetchChatMessages(cid)
			ch <- chatResult{cid, msgs, err}
		}(cid)
	}
	for range emitChats {
		r := <-ch
		if r.err != nil { continue }
		for _, m := range r.messages {
			if b.seen.ShouldEmitMessage(m, filter) {
				events = append(events, GoldenPayEvent{NewMessage: &m})
			}
		}
	}

	// 3. Mark chats without emitting
	for _, cid := range markChats {
		msgs, err := b.session.FetchChatMessages(cid)
		if err != nil { continue }
		for _, m := range msgs {
			if m.ID > b.seen.SeenMessages[cid] {
				b.seen.SeenMessages[cid] = m.ID
			}
		}
	}

	// 4. Persist state
	if err := b.SaveState(); err != nil {
		log.Printf("failed to save state: %v", err)
	}

	return events, nil
}
