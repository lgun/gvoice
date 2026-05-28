package ids

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

func New(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err == nil {
		return normalize(prefix) + "-" + hex.EncodeToString(b[:])
	}
	return normalize(prefix) + "-" + fmt.Sprintf("%d", time.Now().UTC().UnixNano())
}

func normalize(prefix string) string {
	prefix = strings.TrimSpace(strings.ToLower(prefix))
	if prefix == "" {
		return "id"
	}
	return prefix
}
