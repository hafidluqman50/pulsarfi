package contracts

import (
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// GenesisHash roots a Task's hash chain to the original request itself.
func GenesisHash(taskID int64, triggerDescription, owner string) string {
	hash := crypto.Keccak256Hash(
		[]byte(strconv.FormatInt(taskID, 10)),
		[]byte(triggerDescription),
		[]byte(strings.ToLower(owner)),
	)
	return hash.Hex()
}

// DecisionHash computes one agent_sub_tasks row's hash from its own content plus the previous row's hash.
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
