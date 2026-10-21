package agentcore

import "errors"

// The agent domain's sentinel errors. Handlers map them to HTTP status codes;
// nothing here performs I/O.
var (
	ErrChatNotFound       = errors.New("agent: chat not found")
	ErrTurnInFlight       = errors.New("agent: a turn is already running for this chat")
	ErrWalletMismatch     = errors.New("agent: authenticated wallet does not own this task")
	ErrTaskNotFound       = errors.New("agent: task not found")
	ErrTaskNotActionable  = errors.New("agent: task is not actionable")
	ErrTaskNotArmed       = errors.New("agent: task has not been armed yet")
	ErrNothingToRetry     = errors.New("agent: no failed message to retry")
	ErrNoOpenTask         = errors.New("agent: no open task, call open_task first")
	ErrNoPendingExecution = errors.New("agent: task has no pending arm pause to resume")
)
