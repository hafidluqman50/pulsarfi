package public

import (
	"context"
	"time"
)

type PriceLinePoint struct {
	Date  string  `json:"date"`
	Price float64 `json:"price"`
}

func (s *PriceService) PriceLineHistory(ctx context.Context, ticker, rangeName string) ([]PriceLinePoint, error) {
	history, err := s.GetStockHistory(ctx, ticker, rangeName)
	if err != nil {
		return nil, err
	}
	points := make([]PriceLinePoint, 0, len(history))
	for _, p := range history {
		points = append(points, PriceLinePoint{Date: time.UnixMilli(p.Timestamp).UTC().Format("2006-01-02"), Price: p.Value})
	}
	return points, nil
}
