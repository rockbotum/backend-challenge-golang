package metrics

import (
	"errors"

	"github.com/shopspring/decimal"

	"backendchallengegolang/internal/api"
)

var ErrNoLevels = errors.New("no price levels collected")

type Metrics struct {
	AskVolume decimal.Decimal
	BidVolume decimal.Decimal
	AskCount  int64
	BidCount  int64
	Symbols   int64
	Failed    int64
}

func (m *Metrics) Add(book api.OrderBook) {
	askVolume, askLevels := book.AskVolume()
	bidVolume, bidLevels := book.BidVolume()

	if askLevels > 0 {
		m.AskVolume = m.AskVolume.Add(askVolume)
		m.AskCount++
	}

	if bidLevels > 0 {
		m.BidVolume = m.BidVolume.Add(bidVolume)
		m.BidCount++
	}

	m.Symbols++
}

func (m Metrics) AverageAskVolume() (decimal.Decimal, error) {
	if m.AskCount == 0 {
		return decimal.Decimal{}, ErrNoLevels
	}

	return m.AskVolume.Div(decimal.NewFromInt(m.AskCount)), nil
}

func (m Metrics) AverageBidVolume() (decimal.Decimal, error) {
	if m.BidCount == 0 {
		return decimal.Decimal{}, ErrNoLevels
	}

	return m.BidVolume.Div(decimal.NewFromInt(m.BidCount)), nil
}
