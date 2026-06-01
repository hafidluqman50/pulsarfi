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

type recordRedeemApprovalBody struct {
	OnChainID *int64 `json:"on_chain_id" binding:"required"`
	TxHash    string `json:"tx_hash"     binding:"required"`
}

func RecordRedeemApprovalHandler(c *gin.Context) {
	if !ensureService(c, custodianRedeemSvc) {
		return
	}

	claims, _ := custodianMiddleware.Get(c)

	var req recordRedeemApprovalBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	err := custodianRedeemSvc.RecordVote(c.Request.Context(), custodiansvc.RecordRedeemVoteRequest{
		OnChainID:     *req.OnChainID,
		CustodianAddr: claims.WalletAddress,
		VoteType:      "approve",
		TxHash:        req.TxHash,
	})
	if err == custodiansvc.ErrRedeemProposalNotFound {
		response.NotFound(c, "proposal not found")
		return
	}
	if err == custodiansvc.ErrRedeemAlreadyVoted {
		response.BadRequest(c, "approval already recorded")
		return
	}
	if err != nil {
		response.InternalError(c, "failed to record approval")
		return
	}

	if streamService != nil {
		streamService.Emit(external.LevelOK, "[redeem]",
			fmt.Sprintf("approveRedeem · proposal=%d · approver=%s",
				*req.OnChainID, claims.WalletAddress[:10]+"..."))
	}

	response.OK(c, "approval recorded", nil)
}

func RecordRedeemRejectionHandler(c *gin.Context) {
	if !ensureService(c, custodianRedeemSvc) {
		return
	}

	claims, _ := custodianMiddleware.Get(c)

	var req recordRedeemApprovalBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	err := custodianRedeemSvc.RecordVote(c.Request.Context(), custodiansvc.RecordRedeemVoteRequest{
		OnChainID:     *req.OnChainID,
		CustodianAddr: claims.WalletAddress,
		VoteType:      "reject",
		TxHash:        req.TxHash,
	})
	if err == custodiansvc.ErrRedeemProposalNotFound {
		response.NotFound(c, "proposal not found")
		return
	}
	if err == custodiansvc.ErrRedeemAlreadyVoted {
		response.BadRequest(c, "rejection already recorded")
		return
	}
	if err != nil {
		response.InternalError(c, "failed to record rejection")
		return
	}

	if streamService != nil {
		streamService.Emit(external.LevelInfo, "[redeem]",
			fmt.Sprintf("rejectRedeem · proposal=%d · rejecter=%s",
				*req.OnChainID, claims.WalletAddress[:10]+"..."))
	}

	response.OK(c, "rejection recorded", nil)
}

type recordRedeemExecutionBody struct {
	OnChainID   *int64 `json:"on_chain_id"   binding:"required"`
	TxHash      string `json:"tx_hash"       binding:"required"`
	BlockNumber int64  `json:"block_number"`
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
		OnChainID:   onChainID,
		TxHash:      req.TxHash,
		BlockNumber: req.BlockNumber,
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

	proposal, found, err := repos.RedeemProposal.FindByOnChainID(c.Request.Context(), onChainID)
	if err != nil || !found {
		response.NotFound(c, "proposal not found")
		return
	}

	repos.RedeemProposal.MarkRejected(c.Request.Context(), onChainID, req.TxHash)

	repos.StockTransaction.Create(c.Request.Context(), repository.StockTransactionCreateInput{
		StockID:       proposal.StockID,
		WalletAddress: proposal.UserAddress,
		Side:          "cancel-redeem",
		IdrxAmount:    proposal.FeeIdrx,
		StockAmount:   proposal.TokenAmount,
		TxHash:        req.TxHash,
		BlockNumber:   req.BlockNumber,
	})

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
