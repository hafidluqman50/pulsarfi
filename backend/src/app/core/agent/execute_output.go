// Package agentcore holds the agent domain's types and ports. It performs no
// I/O: no repository, no gorm, no go-ethereum, no HTTP. Implementations that
// need the database or the chain live under service/agent or service/external.
package agentcore

import "math/big"

// ExecuteTradeOutput is what the on-chain service returns after a swap. The
// executor reads it to report the trade; it never talks to the chain itself,
// which is why this DTO lives in the domain, not in the adapter.
type ExecuteTradeOutput struct {
	TxHash         string
	TradeID        uint64
	BlockNumber    int64
	LogIndex       int
	Amount         *big.Int
	ReceivedAmount *big.Int
	ProtocolFee    *big.Int
}
