package custodian

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	custodianMiddleware "github.com/horizonlabs/pulsarfi-backend/src/http/middleware/custodian"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	custodiansvc "github.com/horizonlabs/pulsarfi-backend/src/service/custodian"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
)

type recordRedeemRequestBody struct {
	OnChainID   *int64 `json:"on_chain_id"      binding:"required"`
	Ticker      string `json:"ticker"           binding:"required"`
	TokenAmount string `json:"token_amount"     binding:"required"`
	FeeIdrx     string `json:"fee_idrx"`
	UserAddress string `json:"user_address"     binding:"required"`
	TxHash      string `json:"tx_hash"          binding:"required"`
}

func RecordRedeemRequestHandler(c *gin.Context) {
	claims, ok := custodianMiddleware.Get(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	var req recordRedeemRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	onChainID := *req.OnChainID

	stock, found, err := repos.Stock.FindByTicker(c.Request.Context(), req.Ticker)
	if err != nil {
		response.InternalError(c, "failed to lookup stock")
		return
	}
	if !found {
		response.NotFound(c, "stock not found")
		return
	}

	custodian, found, err := repos.Custodian.FindByWalletAddress(c.Request.Context(), claims.WalletAddress)
	if err != nil || !found {
		response.InternalError(c, "failed to lookup custodian")
		return
	}

	_, exists, _ := repos.RedeemProposal.FindByOnChainID(c.Request.Context(), onChainID)
	if exists {
		response.OK(c, "proposal already recorded", nil)
		return
	}

	txHash := req.TxHash
	feeIdrx := req.FeeIdrx
	if feeIdrx == "" {
		feeIdrx = "0"
	}
	proposal, err := repos.RedeemProposal.Create(c.Request.Context(), repository.RedeemProposalCreateInput{
		OnChainID:     onChainID,
		StockID:       stock.ID,
		TokenAmount:   req.TokenAmount,
		FeeIdrx:       feeIdrx,
		UserAddress:   req.UserAddress,
		RequestTxHash: &txHash,
	})
	if err != nil {
		response.InternalError(c, "failed to record proposal")
		return
	}

	repos.RedeemApproval.Create(c.Request.Context(), proposal.ID, custodian.ID, "approve", &txHash)

	if streamService != nil {
		streamService.Emit(external.LevelInfo, "[redeem]",
			fmt.Sprintf("requestRedeem recorded · ticker=%s · proposal=%d · requester=%s",
				req.Ticker, onChainID, custodian.WalletAddress[:10]+"..."))
	}

	response.Created(c, "redeem proposal recorded", proposal)
}

type recordRedeemApprovalBody struct {
	OnChainID *int64 `json:"on_chain_id" binding:"required"`
	TxHash    string `json:"tx_hash"     binding:"required"`
}

func RecordRedeemApprovalHandler(c *gin.Context) {
	claims, ok := custodianMiddleware.Get(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	var req recordRedeemApprovalBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	onChainID := *req.OnChainID

	proposal, found, err := repos.RedeemProposal.FindByOnChainID(c.Request.Context(), onChainID)
	if err != nil || !found {
		response.NotFound(c, "proposal not found")
		return
	}

	custodian, found, err := repos.Custodian.FindByWalletAddress(c.Request.Context(), claims.WalletAddress)
	if err != nil || !found {
		response.InternalError(c, "failed to lookup custodian")
		return
	}

	txHash := req.TxHash
	if err := repos.RedeemApproval.Create(c.Request.Context(), proposal.ID, custodian.ID, "approve", &txHash); err != nil {
		response.BadRequest(c, "approval already recorded")
		return
	}

	if streamService != nil {
		streamService.Emit(external.LevelOK, "[redeem]",
			fmt.Sprintf("approveRedeem · proposal=%d · approver=%s",
				onChainID, custodian.WalletAddress[:10]+"..."))
	}

	response.OK(c, "approval recorded", nil)
}

func RecordRedeemRejectionHandler(c *gin.Context) {
	claims, ok := custodianMiddleware.Get(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	var req recordRedeemApprovalBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	onChainID := *req.OnChainID

	proposal, found, err := repos.RedeemProposal.FindByOnChainID(c.Request.Context(), onChainID)
	if err != nil || !found {
		response.NotFound(c, "proposal not found")
		return
	}

	custodian, found, err := repos.Custodian.FindByWalletAddress(c.Request.Context(), claims.WalletAddress)
	if err != nil || !found {
		response.InternalError(c, "failed to lookup custodian")
		return
	}

	txHash := req.TxHash
	if err := repos.RedeemApproval.Create(c.Request.Context(), proposal.ID, custodian.ID, "reject", &txHash); err != nil {
		response.BadRequest(c, "rejection already recorded")
		return
	}

	if streamService != nil {
		streamService.Emit(external.LevelInfo, "[redeem]",
			fmt.Sprintf("rejectRedeem · proposal=%d · rejecter=%s",
				onChainID, custodian.WalletAddress[:10]+"..."))
	}

	response.OK(c, "rejection recorded", nil)
}

type recordRedeemExecutionBody struct {
	OnChainID *int64 `json:"on_chain_id" binding:"required"`
	TxHash    string `json:"tx_hash"     binding:"required"`
}

func RecordRedeemExecutionHandler(c *gin.Context) {
	if !ensureService(c, custodianSvc) {
		return
	}

	var req recordRedeemExecutionBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	onChainID := *req.OnChainID

	if err := custodianSvc.RecordRedeemExecution(c.Request.Context(), custodiansvc.RecordRedeemExecutionRequest{
		OnChainID: onChainID,
		TxHash:    req.TxHash,
	}); err != nil {
		if err == custodiansvc.ErrProposalNotFound {
			response.NotFound(c, "proposal not found")
			return
		}
		response.InternalError(c, "failed to record execution")
		return
	}

	if streamService != nil {
		streamService.Emit(external.LevelOK, "[evm]",
			fmt.Sprintf("executeRedeem confirmed · proposal=%d · tx=%s",
				onChainID, req.TxHash[:10]+"..."))
	}

	response.OK(c, "execution recorded", nil)
}

func RecordRedeemRejectExecutionHandler(c *gin.Context) {
	var req recordRedeemExecutionBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	onChainID := *req.OnChainID

	_, found, err := repos.RedeemProposal.FindByOnChainID(c.Request.Context(), onChainID)
	if err != nil || !found {
		response.NotFound(c, "proposal not found")
		return
	}

	repos.RedeemProposal.MarkRejected(c.Request.Context(), onChainID, req.TxHash)

	if streamService != nil {
		streamService.Emit(external.LevelOK, "[evm]",
			fmt.Sprintf("executeReject confirmed · proposal=%d · tx=%s",
				onChainID, req.TxHash[:10]+"..."))
	}

	response.OK(c, "rejection execution recorded", nil)
}

func ListRedeemProposalsHandler(c *gin.Context) {
	proposals, err := repos.RedeemProposal.FindAll(c.Request.Context())
	if err != nil {
		response.InternalError(c, "failed to fetch proposals")
		return
	}
	response.OK(c, "proposals retrieved", proposals)
}

func GetRedeemProposalHandler(c *gin.Context) {
	onChainID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid proposal id")
		return
	}

	proposal, found, err := repos.RedeemProposal.FindByOnChainID(c.Request.Context(), onChainID)
	if err != nil || !found {
		response.NotFound(c, "proposal not found")
		return
	}

	approvals, _ := repos.RedeemApproval.FindByProposalID(c.Request.Context(), proposal.ID)

	response.OK(c, "proposal retrieved", gin.H{
		"proposal":  proposal,
		"approvals": approvals,
	})
}
