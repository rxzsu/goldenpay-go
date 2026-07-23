# goldenpay-go

Go port of the [goldenpay](https://github.com/rxzsu/goldenpay) Rust SDK for FunPay automation.

## Features

- Session-based HTTP client with golden key authentication
- Proxy support
- Order polling + auto-delivery
- Offer management (read, edit, create, delete, undercut)
- Chat messaging via FunPay runner API
- Price calculator
- Category tree, filters, subcategories
- Market offer listing (competitor prices)
- Webhook server with HMAC verification
- Offer schedule (activate/deactivate by time)
- State persistence (memory / JSON)
- Delivery automation (inventory, message builder, delivery store)
- Session manager with auto-reconnect on auth errors

## Installation

```sh
go get github.com/rxzsu/goldenpay-go
```

## Quick start

```go
package main

import (
    "fmt"
    "log"
    "os"

    "github.com/rxzsu/goldenpay-go"
)

func main() {
    key := os.Getenv("GOLDEN_KEY")
    if key == "" {
        log.Fatal("GOLDEN_KEY is required")
    }

    client, err := goldenpay.New(goldenpay.NewConfig(key))
    if err != nil {
        log.Fatal(err)
    }

    session, err := client.Connect()
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Logged in as %s (ID %d)\n", session.User().Username, session.User().ID)

    orders, _ := session.FetchOrders()
    for _, o := range orders {
        fmt.Printf("Order %s: %s (%s)\n", o.ID, o.Description, o.Status)
    }

    balance, _ := session.FetchBalance()
    fmt.Printf("Balance: %.2f\n", balance)
}
```

## Documentation

Full package documentation at [pkg.go.dev/github.com/rxzsu/goldenpay-go](https://pkg.go.dev/github.com/rxzsu/goldenpay-go).

See [cmd/example/main.go](cmd/example/main.go) for a complete example with bot polling.

## API

| Method | Description |
|---|---|
| `FetchOrders()` | All orders from trade page |
| `FetchPaidOrders()` | Paid orders only |
| `FetchOrderPage(id)` | Full order details (secrets, params, review) |
| `SendMessage(chatID, text)` | Send chat message |
| `FetchChatMessages(chatID)` | Get chat messages |
| `FetchMyOffers(nodeID)` | Your offers in a category |
| `FetchOfferDetails(nodeID, offerID)` | Current offer field values |
| `EditOffer(nodeID, offerID, patch)` | Patch an offer |
| `CreateOffer(nodeID, details)` | Create a new offer |
| `UndercutPrice(nodeID, offerID, undercutBy, minPrice)` | Auto-price below competition |
| `FetchMarketOffers(nodeID)` | Competitor offers |
| `CalcPrice(nodeID, price)` | Price breakdown |
| `FetchCategoryTree()` | Full category tree |
| `FetchBalance()` | Account balance |
| `RaiseOffers(nodeID)` | Raise all offers |
| `Ping()` | Runner heartbeat |

## License

MIT
