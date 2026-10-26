package repository

import (
	"context"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"gorm.io/gorm"
)

type AgentTradeRepository struct {
	DB *gorm.DB
}

func (r *AgentTradeRepository) FindByTaskID(ctx context.Context, taskID int64) ([]model.AgentTrade, error) {
	var trades []model.AgentTrade
	err := r.DB.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("executed_at ASC").
		Find(&trades).Error
	return trades, err
}

func (r *AgentTradeRepository) Create(ctx context.Context, trade model.AgentTrade) (model.AgentTrade, error) {
	return trade, r.DB.WithContext(ctx).Create(&trade).Error
}

// InstantExecutionRecord carries everything the instant path writes in one
// round trip: the trade, the portfolio activity row and the Task's terminal
// state. One data-modifying CTE instead of five sequential statements.
type InstantExecutionRecord struct {
	TaskID          int64
	SubTaskID       int64
	OnChainTradeID  int64
	TxHash          string
	Ticker          string
	Side            string
	Amount          string
	Summary         string
	StockID         int64
	WalletAddress   string
	IdrxAmount      string
	StockAmount     string
	ProtocolFeeIdrx string
	BlockNumber     int64
	LogIndex        int
}

func (r *AgentTradeRepository) RecordInstantExecution(ctx context.Context, in InstantExecutionRecord) (model.AgentTrade, error) {
	var trade model.AgentTrade
	err := r.DB.WithContext(ctx).Raw(`
		WITH trade AS (
			INSERT INTO agent_trades
				(task_id, sub_task_id, on_chain_trade_id, tx_hash, ticker, side, amount, summary)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			RETURNING id, task_id, sub_task_id, on_chain_trade_id, tx_hash, ticker, side, amount, summary, executed_at
		), stock_tx AS (
			INSERT INTO stock_transactions
				(stock_id, wallet_address, side, idrx_amount, stock_amount, tx_hash, block_number, log_index, protocol_fee_idrx)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT DO NOTHING
			RETURNING id
		), updated AS (
			UPDATE agent_tasks
			SET status = 'executed', armed_at = NOW(), executed_at = NOW()
			WHERE id = ?
			RETURNING id
		)
		SELECT * FROM trade
	`,
		in.TaskID, in.SubTaskID, in.OnChainTradeID, in.TxHash, in.Ticker, in.Side, in.Amount, in.Summary,
		in.StockID, in.WalletAddress, in.Side, in.IdrxAmount, in.StockAmount, in.TxHash, in.BlockNumber, in.LogIndex, in.ProtocolFeeIdrx,
		in.TaskID,
	).Scan(&trade).Error
	return trade, err
}
