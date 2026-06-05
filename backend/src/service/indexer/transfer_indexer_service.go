package indexer

import (
	"context"
	"log"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
	publicsvc "github.com/horizonlabs/pulsarfi-backend/src/service/public"
)

const (
	transferCheckpointID = int64(1)
	defaultConfirmations = int64(5)
	defaultBatchSize     = int64(500)
	defaultLookback      = int64(20)
	defaultPollInterval  = 30 * time.Second
)

var transferTopic = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))
var zeroAddress = common.Address{}

type TransferIndexerConfig struct {
	RPCURL        string
	PollInterval  time.Duration
	BatchSize     int64
	Confirmations int64
	StartLookback int64
}

type TransferIndexerService struct {
	Stocks      *repository.StockRepository
	Checkpoints *repository.TransferIndexerCheckpointRepository
	Recorder    *publicsvc.StockTransactionService
	Price       *external.PriceService
	Config      TransferIndexerConfig

	client *ethclient.Client
}

func (s *TransferIndexerService) Run(ctx context.Context) {
	if strings.TrimSpace(s.Config.RPCURL) == "" {
		log.Println("transfer-indexer disabled: missing RPC URL")
		return
	}

	client, err := ethclient.DialContext(ctx, s.Config.RPCURL)
	if err != nil {
		log.Printf("transfer-indexer dial error: %v", err)
		return
	}
	defer client.Close()
	s.client = client

	interval := s.Config.PollInterval
	if interval <= 0 {
		interval = defaultPollInterval
	}

	log.Printf("transfer-indexer started, interval=%s", interval)
	if err := s.IndexOnce(ctx); err != nil {
		log.Printf("transfer-indexer initial run error: %v", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("transfer-indexer stopped")
			return
		case <-ticker.C:
			if err := s.IndexOnce(ctx); err != nil {
				log.Printf("transfer-indexer run error: %v", err)
			}
		}
	}
}

func (s *TransferIndexerService) IndexOnce(ctx context.Context) error {
	stocks, err := s.Stocks.FindMarketReady(ctx)
	if err != nil {
		return err
	}
	if len(stocks) == 0 {
		return nil
	}

	latest, err := s.client.BlockNumber(ctx)
	if err != nil {
		return err
	}

	confirmations := positiveOrDefault(s.Config.Confirmations, defaultConfirmations)
	target := int64(latest) - confirmations
	if target < 0 {
		return nil
	}

	checkpoint, found, err := s.Checkpoints.FindByID(ctx, transferCheckpointID)
	if err != nil {
		return err
	}

	lastIndexed := checkpoint.LastIndexedBlock
	if !found {
		lastIndexed = target - positiveOrDefault(s.Config.StartLookback, defaultLookback) - 1
		if lastIndexed < -1 {
			lastIndexed = -1
		}
	}
	if target <= lastIndexed {
		return nil
	}

	batchSize := positiveOrDefault(s.Config.BatchSize, defaultBatchSize)
	toBlock := minInt64(target, lastIndexed+batchSize)
	if err := s.indexRange(ctx, stocks, lastIndexed+1, toBlock); err != nil {
		return err
	}
	return s.Checkpoints.Upsert(ctx, transferCheckpointID, toBlock)
}

func (s *TransferIndexerService) indexRange(
	ctx context.Context,
	stocks []model.Stock,
	fromBlock int64,
	toBlock int64,
) error {
	addresses := make([]common.Address, 0, len(stocks))
	stocksByAddress := make(map[string]model.Stock, len(stocks))
	for _, stock := range stocks {
		if stock.ContractAddress == nil || strings.TrimSpace(*stock.ContractAddress) == "" {
			continue
		}
		address := common.HexToAddress(*stock.ContractAddress)
		addresses = append(addresses, address)
		stocksByAddress[strings.ToLower(address.Hex())] = stock
	}
	if len(addresses) == 0 {
		return nil
	}

	logs, err := s.client.FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: big.NewInt(fromBlock),
		ToBlock:   big.NewInt(toBlock),
		Addresses: addresses,
		Topics:    [][]common.Hash{{transferTopic}},
	})
	if err != nil {
		return err
	}

	records := 0
	for _, eventLog := range logs {
		recorded, err := s.recordTransferLog(ctx, stocksByAddress, eventLog)
		if err != nil {
			log.Printf("transfer-indexer record error tx=%s log=%d: %v", eventLog.TxHash.Hex(), eventLog.Index, err)
			continue
		}
		if recorded {
			records++
		}
	}
	if records > 0 {
		log.Printf("transfer-indexer indexed blocks %d-%d, transfer records=%d", fromBlock, toBlock, records)
	}
	return nil
}

func (s *TransferIndexerService) recordTransferLog(
	ctx context.Context,
	stocksByAddress map[string]model.Stock,
	eventLog types.Log,
) (bool, error) {
	if len(eventLog.Topics) < 3 || len(eventLog.Data) == 0 {
		return false, nil
	}

	stockAddress := eventLog.Address
	stock, ok := stocksByAddress[strings.ToLower(stockAddress.Hex())]
	if !ok {
		return false, nil
	}
	if !s.isDirectTokenTransfer(ctx, eventLog.TxHash, stockAddress) {
		return false, nil
	}

	fromAddress := common.BytesToAddress(eventLog.Topics[1].Bytes())
	toAddress := common.BytesToAddress(eventLog.Topics[2].Bytes())
	if fromAddress == zeroAddress || toAddress == zeroAddress || fromAddress == toAddress {
		return false, nil
	}

	stockAmount := new(big.Int).SetBytes(eventLog.Data).String()
	if stockAmount == "0" {
		return false, nil
	}

	_, created, err := s.Recorder.RecordTransfer(ctx, publicsvc.RecordTransferRequest{
		Ticker:      stock.Ticker,
		TxHash:      eventLog.TxHash.Hex(),
		FromAddress: fromAddress.Hex(),
		ToAddress:   toAddress.Hex(),
		IdrxAmount:  s.estimateIDRXAmount(stock, eventLog.Data),
		StockAmount: stockAmount,
		BlockNumber: int64(eventLog.BlockNumber),
		LogIndex:    int(eventLog.Index),
	})
	return created, err
}

func (s *TransferIndexerService) isDirectTokenTransfer(
	ctx context.Context,
	txHash common.Hash,
	stockAddress common.Address,
) bool {
	tx, _, err := s.client.TransactionByHash(ctx, txHash)
	if err != nil {
		log.Printf("transfer-indexer transaction lookup error tx=%s: %v", txHash.Hex(), err)
		return false
	}
	to := tx.To()
	return to != nil && *to == stockAddress
}

func (s *TransferIndexerService) estimateIDRXAmount(stock model.Stock, stockAmountRaw []byte) string {
	price, err := s.Price.GetYahooIDX(stock.IdxTicker)
	if err != nil || price.Price <= 0 {
		return "0"
	}

	stockAmount := new(big.Int).SetBytes(stockAmountRaw)
	qty, _ := new(big.Float).Quo(
		new(big.Float).SetInt(stockAmount),
		new(big.Float).SetInt(big.NewInt(1_000_000_000_000_000_000)),
	).Float64()

	rawIDRX := math.Round(qty * price.Price * 100 * 100)
	if rawIDRX <= 0 {
		return "0"
	}
	return strconvInt64(rawIDRX)
}

func positiveOrDefault(value int64, fallback int64) int64 {
	if value <= 0 {
		return fallback
	}
	return value
}

func minInt64(a int64, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func strconvInt64(value float64) string {
	const maxInt64Float = float64(9223372036854775807)
	if value > maxInt64Float {
		return "0"
	}
	return strconv.FormatInt(int64(value), 10)
}
