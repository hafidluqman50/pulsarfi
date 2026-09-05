package agent

import (
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// GenesisHash roots a Task's hash chain to the original request itself, so
// the chain's first link ties back to what was actually asked rather than
// to an arbitrary empty value. See docs/plans/agent-role-architecture.md §3a.
func GenesisHash(taskID int64, triggerDescription, owner string) string {
	hash := crypto.Keccak256Hash(
		[]byte(strconv.FormatInt(taskID, 10)),
		[]byte(triggerDescription),
		[]byte(strings.ToLower(owner)),
	)
	return hash.Hex()
}

// DecisionHash computes one agent_sub_tasks row's hash from its own content
// plus the previous row's hash — see docs/plans/agent-role-architecture.md
// §7. Deterministic: the same inputs always produce the same hash, so the
// chain is independently re-computable by anyone, not merely trusted
// because it sits in the database in the right order.
func DecisionHash(agentName, stepName, reasoning string, output []byte, prevDecisionHash string) string {
	hash := crypto.Keccak256Hash(
		[]byte(agentName),
		[]byte(stepName),
		[]byte(reasoning),
		output,
		common.HexToHash(prevDecisionHash).Bytes(),
	)
	return hash.Hex()
}
