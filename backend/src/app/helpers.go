package app

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func ContainsWord(text, word string) bool {
	if text == "" || word == "" {
		return false
	}
	re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(word) + `\b`)
	if err != nil {
		return false
	}
	return re.MatchString(text)
}

func ReplaceTickerInText(text, oldTicker, newTicker string) string {
	if text == "" || oldTicker == "" || newTicker == "" || strings.EqualFold(oldTicker, newTicker) {
		return text
	}
	re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(oldTicker) + `\b`)
	if err != nil {
		return text
	}
	return re.ReplaceAllString(text, newTicker)
}

func NormalizeBudgetNumber(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "jt") || strings.Contains(raw, "juta") {
		numStr := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(raw, "juta", ""), "jt", ""))
		numStr = strings.ReplaceAll(numStr, ",", ".")
		if val, err := strconv.ParseFloat(numStr, 64); err == nil && val > 0 {
			return fmt.Sprintf("%.0f", val*1000000)
		}
	}
	if strings.Contains(raw, "k") {
		numStr := strings.TrimSpace(strings.ReplaceAll(raw, "k", ""))
		numStr = strings.ReplaceAll(numStr, ",", ".")
		if val, err := strconv.ParseFloat(numStr, 64); err == nil && val > 0 {
			return fmt.Sprintf("%.0f", val*1000)
		}
	}
	cleaned := strings.ReplaceAll(strings.ReplaceAll(raw, ".", ""), ",", "")
	if val, err := strconv.ParseInt(cleaned, 10, 64); err == nil && val > 0 {
		return strconv.FormatInt(val, 10)
	}
	return ""
}

func ExtractBudgetFromText(candidates ...string) string {
	for _, text := range candidates {
		if strings.TrimSpace(text) == "" {
			continue
		}
		reKV := regexp.MustCompile(`(?i)(?:budget|idrx_cap|batas.*budget).*?:\s*(\d+\s*(?:jt|juta|k|m)|[0-9.,]+)`)
		if m := reKV.FindStringSubmatch(text); len(m) > 1 {
			if norm := NormalizeBudgetNumber(m[1]); norm != "" {
				return norm
			}
		}
		rePhrase := regexp.MustCompile(`(?i)(?:budget|anggaran|senilai|sebesar|modal)\s*(?:sebesar|sebanyak|:|=)?\s*(?:rp\.?\s*)?(\d+\s*(?:jt|juta|k|m)|[0-9.,]+)`)
		if m := rePhrase.FindStringSubmatch(text); len(m) > 1 {
			if norm := NormalizeBudgetNumber(m[1]); norm != "" {
				return norm
			}
		}
		reIDRX := regexp.MustCompile(`(?i)\b(\d+\s*(?:jt|juta)|[0-9.,]+)\s*idrx\b`)
		if m := reIDRX.FindStringSubmatch(text); len(m) > 1 {
			if norm := NormalizeBudgetNumber(m[1]); norm != "" {
				return norm
			}
		}
		if norm := NormalizeBudgetNumber(text); norm != "" {
			return norm
		}
	}
	return ""
}

func GenesisHash(taskID int64, triggerDescription, owner string) string {
	hash := crypto.Keccak256Hash(
		[]byte(strconv.FormatInt(taskID, 10)),
		[]byte(triggerDescription),
		[]byte(strings.ToLower(owner)),
	)
	return hash.Hex()
}

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
