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

type recordMintRequestBody struct {
	OnChainID       *int64  `json:"on_chain_id"      binding:"required"`
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

	_, exists, _ := repos.MintProposal.FindByOnChainID(c.Request.Context(), onChainID)
	if exists {
		response.OK(c, "proposal already recorded", nil)
		return
	}

	requesterID := custodian.ID
	txHash := req.TxHash
	proposal, err := repos.MintProposal.Create(c.Request.Context(), repository.MintProposalCreateInput{
		OnChainID:       onChainID,
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
	repos.MintApproval.Create(c.Request.Context(), proposal.ID, custodian.ID, "approve", &txHash)

	if streamService != nil {
		streamService.Emit(external.LevelInfo, "[mint]",
			fmt.Sprintf("requestMint recorded · ticker=%s · proposal=%d · requester=%s",
				req.Ticker, onChainID, custodian.WalletAddress[:10]+"..."))
	}

	response.Created(c, "mint proposal recorded", proposal)
}

type recordApprovalBody struct {
	OnChainID *int64 `json:"on_chain_id" binding:"required"`
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
	onChainID := *req.OnChainID

	proposal, found, err := repos.MintProposal.FindByOnChainID(c.Request.Context(), onChainID)
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
	if _, err := repos.MintApproval.Create(c.Request.Context(), proposal.ID, custodian.ID, "approve", &txHash); err != nil {
		response.BadRequest(c, "approval already recorded")
		return
	}

	if streamService != nil {
		streamService.Emit(external.LevelOK, "[attest]",
			fmt.Sprintf("approveMint · proposal=%d · approver=%s",
				onChainID, custodian.WalletAddress[:10]+"..."))
	}

	response.OK(c, "approval recorded", nil)
}

func RecordMintRejectionHandler(c *gin.Context) {
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
	onChainID := *req.OnChainID

	proposal, found, err := repos.MintProposal.FindByOnChainID(c.Request.Context(), onChainID)
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
	if _, err := repos.MintApproval.Create(c.Request.Context(), proposal.ID, custodian.ID, "reject", &txHash); err != nil {
		response.BadRequest(c, "rejection already recorded")
		return
	}

	if streamService != nil {
		streamService.Emit(external.LevelInfo, "[attest]",
			fmt.Sprintf("rejectMint · proposal=%d · rejecter=%s",
				onChainID, custodian.WalletAddress[:10]+"..."))
	}

	response.OK(c, "rejection recorded", nil)
}

type recordExecutionBody struct {
	OnChainID       *int64 `json:"on_chain_id"       binding:"required"`
	TxHash          string `json:"tx_hash"           binding:"required"`
	ContractAddress string `json:"contract_address"  binding:"required"`
}

func RecordMintExecutionHandler(c *gin.Context) {
	if !ensureService(c, custodianSvc) {
		return
	}

	var req recordExecutionBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	onChainID := *req.OnChainID

	if err := custodianSvc.RecordMintExecution(c.Request.Context(), custodiansvc.RecordMintExecutionRequest{
		OnChainID:       onChainID,
		TxHash:          req.TxHash,
		ContractAddress: req.ContractAddress,
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
			fmt.Sprintf("executeMint confirmed · proposal=%d · contract=%s · tx=%s",
				onChainID, req.ContractAddress[:10]+"...", req.TxHash[:10]+"..."))
	}

	response.OK(c, "execution recorded", nil)
}

func RecordMintRejectExecutionHandler(c *gin.Context) {
	var req recordExecutionBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	onChainID := *req.OnChainID

	_, found, err := repos.MintProposal.FindByOnChainID(c.Request.Context(), onChainID)
	if err != nil || !found {
		response.NotFound(c, "proposal not found")
		return
	}

	repos.MintProposal.MarkRejected(c.Request.Context(), onChainID, req.TxHash)

	if streamService != nil {
		streamService.Emit(external.LevelOK, "[evm]",
			fmt.Sprintf("executeRejectMint confirmed · proposal=%d · tx=%s",
				onChainID, req.TxHash[:10]+"..."))
	}

	response.OK(c, "rejection execution recorded", nil)
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
