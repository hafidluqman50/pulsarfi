package public

import (
	"errors"

	"github.com/gin-gonic/gin"
	usermw "github.com/horizonlabs/pulsarfi-backend/src/http/middleware/user"
	stocktransactionrequest "github.com/horizonlabs/pulsarfi-backend/src/http/request/public/stock_transaction"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	publicsvc "github.com/horizonlabs/pulsarfi-backend/src/service/public"
)

func ListStockTransactionsHandler(c *gin.Context) {
	if !ensureService(c, publicStockTransactionSvc) {
		return
	}

	transactions, err := publicStockTransactionSvc.ListByWallet(c.Request.Context(), c.Query("wallet_address"))
	if errors.Is(err, publicsvc.ErrWalletAddressRequired) {
		response.BadRequest(c, "wallet_address is required")
		return
	}
	if err != nil {
		response.InternalError(c, "failed to fetch transactions")
		return
	}

	response.OK(c, "transactions retrieved", transactions)
}

func RecordSwapHandler(c *gin.Context) {
	if !ensureService(c, publicStockTransactionSvc) {
		return
	}

	claims, ok := usermw.Get(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}

	req, err := stocktransactionrequest.NewSwapRequest(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	tx, created, err := publicStockTransactionSvc.Record(c.Request.Context(), publicsvc.RecordStockTransactionRequest{
		Ticker:              req.Ticker,
		TxHash:              req.TxHash,
		WalletAddress:       req.WalletAddress,
		Side:                req.Side,
		IdrxAmount:          req.IdrxAmount,
		StockAmount:         req.StockAmount,
		ProtocolFeeIdrx:     req.ProtocolFeeIdrx,
		BlockNumber:         req.BlockNumber,
		LogIndex:            req.LogIndex,
		AuthenticatedWallet: claims.WalletAddress,
	})
	if errors.Is(err, publicsvc.ErrStockNotFound) {
		response.NotFound(c, "stock not found")
		return
	}
	if errors.Is(err, publicsvc.ErrInvalidTransactionSide) {
		response.BadRequest(c, "invalid transaction side")
		return
	}
	if errors.Is(err, publicsvc.ErrWalletMismatch) {
		response.Forbidden(c, "authenticated wallet does not match wallet_address")
		return
	}
	if errors.Is(err, publicsvc.ErrOnChainMismatch) {
		response.UnprocessableEntity(c, "submitted swap data does not match the on-chain transaction", nil)
		return
	}
	if errors.Is(err, publicsvc.ErrVerifierUnavailable) {
		response.InternalError(c, "on-chain verification is unavailable, try again later")
		return
	}
	if err != nil {
		response.InternalError(c, "failed to record transaction")
		return
	}

	if !created {
		response.OK(c, "transaction already recorded", tx)
		return
	}

	response.Created(c, "transaction recorded", tx)
}

func RecordTransferHandler(c *gin.Context) {
	if !ensureService(c, publicStockTransactionSvc) {
		return
	}

	claims, ok := usermw.Get(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}

	req, err := stocktransactionrequest.NewSendRequest(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, created, err := publicStockTransactionSvc.RecordTransfer(c.Request.Context(), publicsvc.RecordTransferRequest{
		Ticker:              req.Ticker,
		TxHash:              req.TxHash,
		FromAddress:         req.FromAddress,
		ToAddress:           req.ToAddress,
		IdrxAmount:          req.IdrxAmount,
		StockAmount:         req.StockAmount,
		BlockNumber:         req.BlockNumber,
		LogIndex:            req.LogIndex,
		AuthenticatedWallet: claims.WalletAddress,
	})
	if errors.Is(err, publicsvc.ErrStockNotFound) {
		response.NotFound(c, "stock not found")
		return
	}
	if errors.Is(err, publicsvc.ErrWalletMismatch) {
		response.Forbidden(c, "authenticated wallet does not match from_address")
		return
	}
	if errors.Is(err, publicsvc.ErrOnChainMismatch) {
		response.UnprocessableEntity(c, "submitted transfer data does not match the on-chain transaction", nil)
		return
	}
	if errors.Is(err, publicsvc.ErrVerifierUnavailable) {
		response.InternalError(c, "on-chain verification is unavailable, try again later")
		return
	}
	if errors.Is(err, publicsvc.ErrTransferAddressRequired) || errors.Is(err, publicsvc.ErrTransferSameAddress) {
		response.BadRequest(c, err.Error())
		return
	}
	if errors.Is(err, publicsvc.ErrTransferRecordIncomplete) {
		response.InternalError(c, "transfer record is incomplete")
		return
	}
	if err != nil {
		response.InternalError(c, "failed to record transfer")
		return
	}

	if !created {
		response.OK(c, "transfer already recorded", result)
		return
	}

	response.Created(c, "transfer recorded", result)
}
