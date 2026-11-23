package app

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"rust-panel-backend/internal/store"
)

func parseUint64Param(c *gin.Context, key string) (uint64, error) {
	raw := strings.TrimSpace(c.Param(key))
	if raw == "" {
		return 0, errors.New("missing")
	}
	return strconv.ParseUint(raw, 10, 64)
}

func normalizeOptionalString(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	return &s
}

func limitOutput(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func addUserID(ids []uint64, userID uint64) []uint64 {
	if userID == 0 {
		return store.CleanUserIDs(ids)
	}
	for _, id := range ids {
		if id == userID {
			return store.CleanUserIDs(ids)
		}
	}
	ids = append(ids, userID)
	return store.CleanUserIDs(ids)
}

func removeUserID(ids []uint64, userID uint64) []uint64 {
	if len(ids) == 0 {
		return nil
	}
	dst := ids[:0]
	for _, id := range ids {
		if id != userID {
			dst = append(dst, id)
		}
	}
	return store.CleanUserIDs(dst)
}
