package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"runtime/debug"
	"strings"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

var _bp = func() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Path != "" {
		h := sha256.Sum256([]byte(bi.Main.Path))
		return hex.EncodeToString(h[:4])
	}
	return common.GetRandomString(8)
}()

func RequestId() func(c *gin.Context) {
	return func(c *gin.Context) {
		id := common.GetTimeString() + _bp + common.GetRandomString(8)
		c.Set(common.RequestIdKey, id)
		if atelierRequestId := normalizeAtelierRequestId(c.GetHeader(common.AtelierRequestIdKey)); atelierRequestId != "" {
			c.Set(common.AtelierRequestIdKey, atelierRequestId)
		}
		ctx := context.WithValue(c.Request.Context(), common.RequestIdKey, id)
		c.Request = c.Request.WithContext(ctx)
		c.Header(common.RequestIdKey, id)
		c.Next()
	}
}

func normalizeAtelierRequestId(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 64 {
		return ""
	}
	for _, r := range value {
		if r > unicode.MaxASCII {
			return ""
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return ""
	}
	return value
}
