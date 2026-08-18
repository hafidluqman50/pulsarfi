package external

// ChainVerifyService re-derives the facts recorded by the public write
// endpoints (redeem requests, swaps, transfers) from the actual on-chain
// transaction receipt, so a valid-but-dishonest caller can't submit
// fabricated amounts/side under their own wallet. One RPC call per write
// request (TransactionReceipt by tx_hash) — no polling, no indexing.

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	ErrReceiptNotFound   = errors.New("chainverify: transaction receipt not found")
	ErrTxFailed          = errors.New("chainverify: transaction reverted")
	ErrEventNotFound     = errors.New("chainverify: expected event not found in receipt logs")
	ErrNotDirectTransfer = errors.New("chainverify: transaction target is not the stock token (not a direct transfer)")
)

var redeemRequestedTopic = crypto.Keccak256Hash([]byte("RedeemRequested(uint256,address,string,uint256,uint256)"))
var v4SwappedTopic = crypto.Keccak256Hash([]byte("V4Swapped(string,address,bool,uint256,uint256)"))
var transferTopic = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

var stringType, _ = abi.NewType("string", "", nil)
var uint256Type, _ = abi.NewType("uint256", "", nil)
var boolType, _ = abi.NewType("bool", "", nil)

var redeemRequestedDataArgs = abi.Arguments{{Type: stringType}, {Type: uint256Type}, {Type: uint256Type}}
var v4SwappedDataArgs = abi.Arguments{{Type: boolType}, {Type: uint256Type}, {Type: uint256Type}}

// ChainVerifyService holds a live RPC connection used to re-derive on-chain facts.
type ChainVerifyService struct {
	client          *ethclient.Client
	protocolAddress common.Address
}

// NewChainVerifyServiceFromEnv dials using the same ALCHEMY_RPC_URL/PULSAR_PROTOCOL
// env vars already used by PriceService/StockService.
func NewChainVerifyServiceFromEnv() (*ChainVerifyService, error) {
	return NewChainVerifyService(os.Getenv("ALCHEMY_RPC_URL"), os.Getenv("PULSAR_PROTOCOL"))
}

func NewChainVerifyService(rpcURL, protocolAddress string) (*ChainVerifyService, error) {
	if strings.TrimSpace(rpcURL) == "" || strings.TrimSpace(protocolAddress) == "" {
		return nil, fmt.Errorf("chainverify: missing RPC URL or protocol address")
	}
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("chainverify: dial: %w", err)
	}
	return &ChainVerifyService{client: client, protocolAddress: common.HexToAddress(protocolAddress)}, nil
}

// RedeemRequestedEvent is the decoded form of PulsarProtocol's on-chain event.
type RedeemRequestedEvent struct {
	RequestID   *big.Int
	User        common.Address
	Ticker      string
	TokenAmount *big.Int
	FeeIdrx     *big.Int
}

func (s *ChainVerifyService) VerifyRedeemRequested(ctx context.Context, txHash string) (*RedeemRequestedEvent, error) {
	receipt, err := s.receiptOK(ctx, txHash)
	if err != nil {
		return nil, err
	}

	for _, l := range receipt.Logs {
		if l.Address != s.protocolAddress || len(l.Topics) != 3 || l.Topics[0] != redeemRequestedTopic {
			continue
		}
		values, err := redeemRequestedDataArgs.Unpack(l.Data)
		if err != nil {
			return nil, fmt.Errorf("chainverify: decode RedeemRequested data: %w", err)
		}
		ticker, ok := values[0].(string)
		if !ok {
			return nil, fmt.Errorf("chainverify: unexpected ticker type in RedeemRequested log")
		}
		tokenAmount, ok := values[1].(*big.Int)
		if !ok {
			return nil, fmt.Errorf("chainverify: unexpected tokenAmount type in RedeemRequested log")
		}
		feeIdrx, ok := values[2].(*big.Int)
		if !ok {
			return nil, fmt.Errorf("chainverify: unexpected feeIdrx type in RedeemRequested log")
		}
		return &RedeemRequestedEvent{
			RequestID:   new(big.Int).SetBytes(l.Topics[1].Bytes()),
			User:        common.BytesToAddress(l.Topics[2].Bytes()),
			Ticker:      ticker,
			TokenAmount: tokenAmount,
			FeeIdrx:     feeIdrx,
		}, nil
	}

	return nil, ErrEventNotFound
}

// V4SwappedEvent is the decoded form of PulsarProtocol's on-chain event.
type V4SwappedEvent struct {
	User      common.Address
	BuyStock  bool
	AmountIn  *big.Int
	AmountOut *big.Int
}

func (s *ChainVerifyService) VerifyV4Swapped(ctx context.Context, txHash, ticker string) (*V4SwappedEvent, error) {
	receipt, err := s.receiptOK(ctx, txHash)
	if err != nil {
		return nil, err
	}

	tickerTopic := crypto.Keccak256Hash([]byte(ticker))

	for _, l := range receipt.Logs {
		if l.Address != s.protocolAddress || len(l.Topics) != 3 {
			continue
		}
		if l.Topics[0] != v4SwappedTopic || l.Topics[1] != tickerTopic {
			continue
		}
		values, err := v4SwappedDataArgs.Unpack(l.Data)
		if err != nil {
			return nil, fmt.Errorf("chainverify: decode V4Swapped data: %w", err)
		}
		buyStock, ok := values[0].(bool)
		if !ok {
			return nil, fmt.Errorf("chainverify: unexpected buyStock type in V4Swapped log")
		}
		amountIn, ok := values[1].(*big.Int)
		if !ok {
			return nil, fmt.Errorf("chainverify: unexpected amountIn type in V4Swapped log")
		}
		amountOut, ok := values[2].(*big.Int)
		if !ok {
			return nil, fmt.Errorf("chainverify: unexpected amountOut type in V4Swapped log")
		}
		return &V4SwappedEvent{
			User:      common.BytesToAddress(l.Topics[2].Bytes()),
			BuyStock:  buyStock,
			AmountIn:  amountIn,
			AmountOut: amountOut,
		}, nil
	}

	return nil, ErrEventNotFound
}

// TransferEvent is the decoded form of a plain ERC20 Transfer log.
type TransferEvent struct {
	From   common.Address
	To     common.Address
	Amount *big.Int
}

// VerifyTransfer only accepts DIRECT transfers — the transaction's own `to`
// must be the token contract itself (a plain transfer()/transferFrom() call).
// Without this, a Transfer log that's just a side effect of some other
// contract call (e.g. swapV4 pulling the seller's stock into the protocol,
// PulsarProtocol.sol's swapV4) would pass the log-decode check too, letting
// a seller double-record their own sell as an extra fabricated transfer.
func (s *ChainVerifyService) VerifyTransfer(ctx context.Context, txHash string, tokenAddress common.Address, logIndex int) (*TransferEvent, error) {
	receipt, err := s.receiptOK(ctx, txHash)
	if err != nil {
		return nil, err
	}

	tx, _, err := s.client.TransactionByHash(ctx, common.HexToHash(strings.TrimSpace(txHash)))
	if err != nil {
		return nil, fmt.Errorf("chainverify: transaction lookup: %w", err)
	}
	to := tx.To()
	if to == nil || *to != tokenAddress {
		return nil, ErrNotDirectTransfer
	}

	for _, l := range receipt.Logs {
		if l.Address != tokenAddress || uint(l.Index) != uint(logIndex) {
			continue
		}
		if len(l.Topics) != 3 || l.Topics[0] != transferTopic {
			continue
		}
		return &TransferEvent{
			From:   common.BytesToAddress(l.Topics[1].Bytes()),
			To:     common.BytesToAddress(l.Topics[2].Bytes()),
			Amount: new(big.Int).SetBytes(l.Data),
		}, nil
	}

	return nil, ErrEventNotFound
}

func (s *ChainVerifyService) receiptOK(ctx context.Context, txHash string) (*types.Receipt, error) {
	hash := common.HexToHash(strings.TrimSpace(txHash))
	receipt, err := s.client.TransactionReceipt(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("%w: %s (%v)", ErrReceiptNotFound, txHash, err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil, fmt.Errorf("%w: %s", ErrTxFailed, txHash)
	}
	return receipt, nil
}
