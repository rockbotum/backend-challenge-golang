package api

import (
	"encoding/json"
	"fmt"

	"github.com/shopspring/decimal"
)

type Order struct {
	Price  decimal.Decimal
	Volume decimal.Decimal
}

type OrderBookDto struct {
	LastUpdateID int64   `json:"lastUpdateId"`
	Bids         []Order `json:"bids"`
	Asks         []Order `json:"asks"`
}

func (o *Order) UnmarshalJSON(data []byte) error {
	var raw []string

	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("decode order: %w", err)
	}

	if len(raw) != 2 {
		return fmt.Errorf("invalid order lenght: %d, want 2", len(raw))
	}

	price, err := decimal.NewFromString(raw[0])
	if err != nil {
		return fmt.Errorf("parse price %q: %w", raw[0], err)
	}

	volume, err := decimal.NewFromString(raw[1])
	if err != nil {
		return fmt.Errorf("parse volume %q: %w", raw[1], err)
	}

	o.Price = price
	o.Volume = volume

	return nil
}
