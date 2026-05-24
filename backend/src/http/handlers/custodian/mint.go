package custodian

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	custodianMiddleware "github.com/horizonlabs/pulsarfi-backend/src/http/middleware/custodian"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
)

type recordMintRequestBody struct {
	OnChainID       int64   `json:"on_chain_id"      binding:"required"`
	Ticker          string  `json:"ticker"           binding:"required"`
	TokenAmount     string  `json:"token_amount"     binding:"required"`
	IdrxAmount      *string `json:"idrx_amount"`
	AttestationHash string  `json:"attestation_hash" binding:"required"`
	Destination     string  `json:"destination"      binding:"required,oneof=operator_wallet liquidity_pool"`
	TxHash          string  `json:"tx_hash"          binding:"required"`
}

func RecordMintRequestHandler(c *gin.Context) {
	claims, ok := custodianMiddleware.Get(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	var req recordMintRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

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

	_, exists, _ := repos.MintProposal.FindByOnChainID(c.Request.Context(), req.OnChainID)
	if exists {
		response.OK(c, "proposal already recorded", nil)
		return
	}

	requesterID := custodian.ID
	txHash := req.TxHash
	proposal, err := repos.MintProposal.Create(c.Request.Context(), repository.MintProposalCreateInput{
		OnChainID:       req.OnChainID,
		StockID:         stock.ID,
		RequesterID:     &requesterID,
		TokenAmount:     req.TokenAmount,
		IdrxAmount:      req.IdrxAmount,
		AttestationHash: req.AttestationHash,
		Destination:     req.Destination,
		RequestTxHash:   &txHash,
	})
	if err != nil {
		response.InternalError(c, "failed to record proposal")
		return
	}

	// Record requester's auto-approval (approvalCount starts at 1 on-chain)
	repos.MintApproval.Create(c.Request.Context(), proposal.ID, custodian.ID, &txHash)

	if streamService != nil {
		streamService.Emit(external.LevelInfo, "[mint]",
			fmt.Sprintf("requestMint recorded · ticker=%s · proposal=%d · requester=%s",
				req.Ticker, req.OnChainID, custodian.WalletAddress[:10]+"..."))
	}

	response.Created(c, "mint proposal recorded", proposal)
}

type recordApprovalBody struct {
	OnChainID int64  `json:"on_chain_id" binding:"required"`
	TxHash    string `json:"tx_hash"     binding:"required"`
}

func RecordMintApprovalHandler(c *gin.Context) {
	claims, ok := custodianMiddleware.Get(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	var req recordApprovalBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	proposal, found, err := repos.MintProposal.FindByOnChainID(c.Request.Context(), req.OnChainID)
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
	repos.MintApproval.Create(c.Request.Context(), proposal.ID, custodian.ID, &txHash)
	repos.MintProposal.IncrementApprovalCount(c.Request.Context(), req.OnChainID)

	if streamService != nil {
		streamService.Emit(external.LevelOK, "[attest]",
			fmt.Sprintf("approveMint · proposal=%d · approver=%s",
				req.OnChainID, custodian.WalletAddress[:10]+"..."))
	}

	response.OK(c, "approval recorded", nil)
}

type recordExecutionBody struct {
	OnChainID       int64  `json:"on_chain_id"       binding:"required"`
	TxHash          string `json:"tx_hash"           binding:"required"`
	ContractAddress string `json:"contract_address"  binding:"required"`
}

func RecordMintExecutionHandler(c *gin.Context) {
	var req recordExecutionBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	proposal, found, err := repos.MintProposal.FindByOnChainID(c.Request.Context(), req.OnChainID)
	if err != nil || !found {
		response.NotFound(c, "proposal not found")
		return
	}

	repos.MintProposal.MarkExecuted(c.Request.Context(), req.OnChainID, req.TxHash)
	repos.Stock.UpdateContractAddress(c.Request.Context(), proposal.StockID, repository.StockUpdateContractInput{
		ContractAddress: req.ContractAddress,
	})

	if streamService != nil {
		streamService.Emit(external.LevelOK, "[evm]",
			fmt.Sprintf("executeMint confirmed · proposal=%d · contract=%s · tx=%s",
				req.OnChainID, req.ContractAddress[:10]+"...", req.TxHash[:10]+"..."))
	}

	response.OK(c, "execution recorded", nil)
}

func ListMintProposalsHandler(c *gin.Context) {
	proposals, err := repos.MintProposal.FindAll(c.Request.Context())
	if err != nil {
		response.InternalError(c, "failed to fetch proposals")
		return
	}
	response.OK(c, "proposals retrieved", proposals)
}

func GetMintProposalHandler(c *gin.Context) {
	onChainID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid proposal id")
		return
	}

	proposal, found, err := repos.MintProposal.FindByOnChainID(c.Request.Context(), onChainID)
	if err != nil || !found {
		response.NotFound(c, "proposal not found")
		return
	}

	approvals, _ := repos.MintApproval.FindByProposalID(c.Request.Context(), proposal.ID)

	response.OK(c, "proposal retrieved", gin.H{
		"proposal":  proposal,
		"approvals": approvals,
	})
}
