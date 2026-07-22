package goldenpay

// OfferEditBuilder provides a fluent API for building OfferEdit patches.
type OfferEditBuilder struct {
	inner OfferEdit
}

func NewOfferEditBuilder() *OfferEditBuilder {
	return &OfferEditBuilder{}
}

func (b *OfferEditBuilder) Quantity(v string) *OfferEditBuilder {
	b.inner.Quantity = &v; return b
}
func (b *OfferEditBuilder) Quantity2(v string) *OfferEditBuilder {
	b.inner.Quantity2 = &v; return b
}
func (b *OfferEditBuilder) Method(v string) *OfferEditBuilder {
	b.inner.Method = &v; return b
}
func (b *OfferEditBuilder) OfferType(v string) *OfferEditBuilder {
	b.inner.OfferType = &v; return b
}
func (b *OfferEditBuilder) ServerID(v string) *OfferEditBuilder {
	b.inner.ServerID = &v; return b
}
func (b *OfferEditBuilder) Location(v string) *OfferEditBuilder {
	b.inner.Location = &v; return b
}
func (b *OfferEditBuilder) Price(v string) *OfferEditBuilder {
	b.inner.Price = &v; return b
}
func (b *OfferEditBuilder) Active(v bool) *OfferEditBuilder {
	b.inner.Active = &v; return b
}
func (b *OfferEditBuilder) Deleted(v bool) *OfferEditBuilder {
	b.inner.Deleted = &v; return b
}
func (b *OfferEditBuilder) DescRU(v string) *OfferEditBuilder {
	b.inner.DescriptionRU = &v; return b
}
func (b *OfferEditBuilder) DescEN(v string) *OfferEditBuilder {
	b.inner.DescriptionEN = &v; return b
}
func (b *OfferEditBuilder) PaymentMsgRU(v string) *OfferEditBuilder {
	b.inner.PaymentMsgRU = &v; return b
}
func (b *OfferEditBuilder) PaymentMsgEN(v string) *OfferEditBuilder {
	b.inner.PaymentMsgEN = &v; return b
}
func (b *OfferEditBuilder) SummaryRU(v string) *OfferEditBuilder {
	b.inner.SummaryRU = &v; return b
}
func (b *OfferEditBuilder) SummaryEN(v string) *OfferEditBuilder {
	b.inner.SummaryEN = &v; return b
}
func (b *OfferEditBuilder) Game(v string) *OfferEditBuilder {
	b.inner.Game = &v; return b
}
func (b *OfferEditBuilder) Images(v string) *OfferEditBuilder {
	b.inner.Images = &v; return b
}
func (b *OfferEditBuilder) DeactivateAfterSale(v bool) *OfferEditBuilder {
	b.inner.DeactivateAfterSale = &v; return b
}

// Build returns the constructed OfferEdit.
func (b *OfferEditBuilder) Build() OfferEdit {
	return b.inner
}
