package helper

import (
    "fmt"
    "strings"
)

const executeInstruction = "The task is armed and approved on-chain. Size within the approved budget and execute the trade on-chain using submit_trade. If Nova's findings or verdict appear above, reflect them in your chat reasoning to explain your decision."

func BuildExecutorRequest(userInstruction, shape, ticker, side, budgetIDRX, sellAmount, novaFindings string) string {
    var request strings.Builder
    fmt.Fprintf(&request, "User instruction: %s\n\nConfirmed parameters:\n- Shape: %s\n- Ticker: %s\n- Side: %s\n", userInstruction, shape, ticker, side)
    if budgetIDRX != "" {
        fmt.Fprintf(&request, "- IDRX budget: %s\n", budgetIDRX)
    }

    if sellAmount != "" {
        fmt.Fprintf(&request, "- Sell amount: %s\n", sellAmount)
    }

    if strings.TrimSpace(novaFindings) != "" {
        fmt.Fprintf(&request, "\nNova's findings:\n%s\n", novaFindings)
    }

    request.WriteString("\n")
    request.WriteString(executeInstruction)
    return request.String()
}
