package goldenpay

import (
	"crypto/rand"
	"math/big"
	"strings"
	"time"
)

const tagAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

func randomTag() string {
	b := make([]byte, 10)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(tagAlphabet))))
		b[i] = tagAlphabet[n.Int64()]
	}
	return string(b)
}

func extractPHPSessID(cookies []string) string {
	for _, c := range cookies {
		if idx := strings.Index(c, "PHPSESSID="); idx >= 0 {
			val := c[idx+len("PHPSESSID="):]
			if semi := strings.IndexByte(val, ';'); semi >= 0 {
				val = val[:semi]
			}
			val = strings.TrimSpace(val)
			if val != "" {
				return val
			}
		}
	}
	return ""
}

func retrySleep(attempt int, base time.Duration) time.Duration {
	if attempt <= 1 {
		return base
	}
	factor := 1 << (attempt - 1)
	return base * time.Duration(factor)
}
