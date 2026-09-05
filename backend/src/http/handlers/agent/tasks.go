package agent

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	usermw "github.com/horizonlabs/pulsarfi-backend/src/http/middleware/user"
	taskrequest "github.com/horizonlabs/pulsarfi-backend/src/http/request/agent/task"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	agentsvc "github.com/horizonlabs/pulsarfi-backend/src/service/agent"
)

// ListTasksHandler always scopes to the authenticated wallet — task
// history is private, unlike public swap/transfer history.
func ListTasksHandler(c *gin.Context) {
	if !ensureService(c) {
		return
	}
	claims, ok := usermw.Get(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}

	tasks, err := taskSvc.ListTasks(c.Request.Context(), claims.WalletAddress)
	if err != nil {
		response.InternalError(c, "failed to fetch tasks")
		return
	}

	response.OK(c, "tasks retrieved", tasks)
}

// ArmTaskHandler is the one call that actually moves this Task on-chain:
// createTask always, grantTradePermission only if the Task is actionable.
// Returns the token/amount the frontend still needs to prompt the owner's
// own ERC20 approve() for — that step is never proxied through this
// backend.
func ArmTaskHandler(c *gin.Context) {
	if !ensureService(c) {
		return
	}
	claims, ok := usermw.Get(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid task id")
		return
	}
	armRequest, err := taskrequest.NewArmRequest(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	armResult, err := taskSvc.ArmTask(c.Request.Context(), taskID, claims.WalletAddress, agentsvc.ArmTaskInput{
		TotalBudget: armRequest.TotalBudget,
		DurationSec: armRequest.DurationSec,
	})
	if errors.Is(err, agentsvc.ErrTaskNotFound) {
		response.NotFound(c, "task not found")
		return
	}
	if errors.Is(err, agentsvc.ErrWalletMismatch) {
		response.Forbidden(c, "task does not belong to the authenticated wallet")
		return
	}
	if err != nil {
		response.InternalError(c, "failed to arm task")
		return
	}

	response.OK(c, "task armed", armResult)
}

// DisarmTaskHandler calls cancelTask only. approve(0) on the relevant
// token is a separate, direct wallet transaction the frontend fires on
// its own — this endpoint never touches ERC20 allowance.
func DisarmTaskHandler(c *gin.Context) {
	if !ensureService(c) {
		return
	}
	claims, ok := usermw.Get(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid task id")
		return
	}

	err = taskSvc.DisarmTask(c.Request.Context(), taskID, claims.WalletAddress)
	if errors.Is(err, agentsvc.ErrTaskNotFound) {
		response.NotFound(c, "task not found")
		return
	}
	if errors.Is(err, agentsvc.ErrWalletMismatch) {
		response.Forbidden(c, "task does not belong to the authenticated wallet")
		return
	}
	if errors.Is(err, agentsvc.ErrTaskNotArmed) {
		response.UnprocessableEntity(c, "task has not been armed yet", nil)
		return
	}
	if err != nil {
		response.InternalError(c, "failed to disarm task")
		return
	}

	response.OK(c, "task disarmed", nil)
}

// PauseTaskHandler and ResumeTaskHandler move no funds and touch no
// on-chain state — plain wallet-owner-authenticated backend calls. A
// paused task's tick is skipped outright by the evaluation heartbeat, not
// partially run.
func PauseTaskHandler(c *gin.Context) {
	if !ensureService(c) {
		return
	}
	claims, ok := usermw.Get(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid task id")
		return
	}

	err = taskSvc.PauseTask(c.Request.Context(), taskID, claims.WalletAddress)
	if errors.Is(err, agentsvc.ErrTaskNotFound) {
		response.NotFound(c, "task not found")
		return
	}
	if errors.Is(err, agentsvc.ErrWalletMismatch) {
		response.Forbidden(c, "task does not belong to the authenticated wallet")
		return
	}
	if err != nil {
		response.InternalError(c, "failed to pause task")
		return
	}

	response.OK(c, "task paused", nil)
}

func ResumeTaskHandler(c *gin.Context) {
	if !ensureService(c) {
		return
	}
	claims, ok := usermw.Get(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid task id")
		return
	}

	err = taskSvc.ResumeTask(c.Request.Context(), taskID, claims.WalletAddress)
	if errors.Is(err, agentsvc.ErrTaskNotFound) {
		response.NotFound(c, "task not found")
		return
	}
	if errors.Is(err, agentsvc.ErrWalletMismatch) {
		response.Forbidden(c, "task does not belong to the authenticated wallet")
		return
	}
	if err != nil {
		response.InternalError(c, "failed to resume task")
		return
	}

	response.OK(c, "task resumed", nil)
}

// GetTaskTradesHandler returns a Task's on-chain Trade ledger — zero, one,
// or many fills. Scoped to the task's own owner, unlike GetReasoningHandler.
func GetTaskTradesHandler(c *gin.Context) {
	if !ensureService(c) {
		return
	}
	claims, ok := usermw.Get(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid task id")
		return
	}

	trades, err := taskSvc.GetTrades(c.Request.Context(), taskID, claims.WalletAddress)
	if errors.Is(err, agentsvc.ErrTaskNotFound) {
		response.NotFound(c, "task not found")
		return
	}
	if errors.Is(err, agentsvc.ErrWalletMismatch) {
		response.Forbidden(c, "task does not belong to the authenticated wallet")
		return
	}
	if err != nil {
		response.InternalError(c, "failed to fetch trades")
		return
	}

	response.OK(c, "trades retrieved", trades)
}

// GetReasoningHandler is deliberately public (no auth): its entire purpose
// is third-party verification against the on-chain reasoningHash, not
// just the task owner's own use.
func GetReasoningHandler(c *gin.Context) {
	if !ensureService(c) {
		return
	}

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid task id")
		return
	}

	chain, err := taskSvc.GetReasoningChain(c.Request.Context(), taskID)
	if errors.Is(err, agentsvc.ErrTaskNotFound) {
		response.NotFound(c, "task not found")
		return
	}
	if err != nil {
		response.InternalError(c, "failed to fetch reasoning chain")
		return
	}

	response.OK(c, "reasoning chain retrieved", chain)
}
