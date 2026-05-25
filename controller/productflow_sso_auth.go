package controller

import (
	"crypto/subtle"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

var (
	errProductFlowSSONotLoggedIn = errors.New("not logged in")
	errProductFlowSSOForbidden   = errors.New("user is not allowed to start Atelier SSO")
)

func currentBrowserSessionUserID(c *gin.Context) (int, error) {
	session := sessions.Default(c)
	id, ok := session.Get("id").(int)
	if !ok || id <= 0 {
		return 0, errProductFlowSSONotLoggedIn
	}
	status, ok := session.Get("status").(int)
	if !ok || status != common.UserStatusEnabled {
		return 0, errProductFlowSSOForbidden
	}
	role, ok := session.Get("role").(int)
	if !ok || role < common.RoleCommonUser {
		return 0, errProductFlowSSOForbidden
	}
	return id, nil
}

func isValidProductFlowSecret(c *gin.Context, expected string) bool {
	actual := strings.TrimSpace(c.GetHeader("Authorization"))
	if !strings.HasPrefix(strings.ToLower(actual), "bearer ") {
		return false
	}
	actual = strings.TrimSpace(actual[len("Bearer "):])
	if actual == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}
