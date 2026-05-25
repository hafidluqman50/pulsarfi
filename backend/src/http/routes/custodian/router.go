package custodian

import (
	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/auth"
	custodianHandler "github.com/horizonlabs/pulsarfi-backend/src/http/handlers/custodian"
	custodianMiddleware "github.com/horizonlabs/pulsarfi-backend/src/http/middleware/custodian"
)

func RegisterRoutes(rg *gin.RouterGroup, jwtConfig auth.Config) {
	// SSE stream — token via query param karena EventSource tidak support custom header
	rg.GET("/stream", custodianHandler.StreamHandler)

	protected := rg.Group("", custodianMiddleware.Auth(jwtConfig))

	// Dashboard stats
	protected.GET("/stats", custodianHandler.GetStatsHandler)

	// Unified pending request queue (mints + redeems)
	protected.GET("/requests", custodianHandler.ListPendingRequestsHandler)

	// Mint proposals
	protected.POST("/mint-proposals", custodianHandler.RecordMintRequestHandler)
	protected.POST("/mint-proposals/approve", custodianHandler.RecordMintApprovalHandler)
	protected.POST("/mint-proposals/reject", custodianHandler.RecordMintRejectionHandler)
	protected.POST("/mint-proposals/execute", custodianHandler.RecordMintExecutionHandler)
	protected.POST("/mint-proposals/execute-reject", custodianHandler.RecordMintRejectExecutionHandler)
	protected.GET("/mint-proposals", custodianHandler.ListMintProposalsHandler)
	protected.GET("/mint-proposals/:id", custodianHandler.GetMintProposalHandler)

	// Redeem proposals
	protected.POST("/redeem-proposals", custodianHandler.RecordRedeemRequestHandler)
	protected.POST("/redeem-proposals/approve", custodianHandler.RecordRedeemApprovalHandler)
	protected.POST("/redeem-proposals/reject", custodianHandler.RecordRedeemRejectionHandler)
	protected.POST("/redeem-proposals/execute", custodianHandler.RecordRedeemExecutionHandler)
	protected.POST("/redeem-proposals/execute-reject", custodianHandler.RecordRedeemRejectExecutionHandler)
	protected.GET("/redeem-proposals", custodianHandler.ListRedeemProposalsHandler)
	protected.GET("/redeem-proposals/:id", custodianHandler.GetRedeemProposalHandler)

	// Proof of reserves
	protected.POST("/attestations", custodianHandler.SubmitAttestationHandler)
	protected.GET("/attestations", custodianHandler.ListAttestationsHandler)

	// KYC management
	protected.GET("/wallet-verifications", custodianHandler.ListWalletVerificationsHandler)
	protected.PATCH("/wallet-verifications/:id", custodianHandler.UpdateWalletVerificationHandler)
}
