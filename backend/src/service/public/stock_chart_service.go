package public

import (
	"context"
	"strings"
	"time"
)

type PriceLinePoint struct {
	Date  string  `json:"date"`
	Price float64 `json:"price"`
}

func (s *PriceService) PriceLineHistory(ctx context.Context, ticker, rangeName string) ([]PriceLinePoint, error) {
	history, err := s.Price.GetYahooIDXHistory(strings.ToUpper(ticker), rangeName)
	if err != nil {
		return nil, err
	}
	if len(history) == 0 {
		return nil, ErrStockNotFound
	}
	points := make([]PriceLinePoint, 0, len(history))
	for _, p := range history {
		points = append(points, PriceLinePoint{Date: time.UnixMilli(p.Timestamp).UTC().Format("2006-01-02"), Price: p.Value})
	}
	return points, nil
}
