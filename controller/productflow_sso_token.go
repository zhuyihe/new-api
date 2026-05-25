package controller

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const productFlowTokenLockKeyPrefix = "productflow:sso:token-lock:"

var productFlowTokenLocks sync.Map

func getOrCreateProductFlowToken(userID int, cfg productFlowSSOConfig) (*model.Token, error) {
	unlock, err := acquireProductFlowTokenLock(userID, cfg.TokenName)
	if err != nil {
		return nil, err
	}
	defer unlock()
	return getOrCreateProductFlowTokenUnlocked(userID, cfg)
}

func getOrCreateProductFlowTokenUnlocked(userID int, cfg productFlowSSOConfig) (*model.Token, error) {
	var token model.Token
	err := model.DB.Where("user_id = ? AND name = ?", userID, cfg.TokenName).Order("id desc").Limit(1).Find(&token).Error
	if err != nil {
		return nil, err
	}
	if token.Id == 0 {
		return createProductFlowToken(userID, cfg)
	}
	return updateProductFlowToken(&token, cfg)
}

func acquireProductFlowTokenLock(userID int, tokenName string) (func(), error) {
	key := fmt.Sprintf("%s%d:%s", productFlowTokenLockKeyPrefix, userID, tokenName)
	if common.RedisEnabled && common.RDB != nil {
		return acquireProductFlowRedisLock(key)
	}
	value, _ := productFlowTokenLocks.LoadOrStore(key, &sync.Mutex{})
	mutex := value.(*sync.Mutex)
	mutex.Lock()
	return mutex.Unlock, nil
}

func acquireProductFlowRedisLock(key string) (func(), error) {
	value, err := common.GenerateRandomCharsKey(24)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	deadline := time.Now().Add(2 * time.Second)
	for {
		locked, err := common.RDB.SetNX(ctx, key, value, 10*time.Second).Result()
		if err != nil {
			return nil, err
		}
		if locked {
			return func() { releaseProductFlowRedisLock(ctx, key, value) }, nil
		}
		if time.Now().After(deadline) {
			return nil, errors.New("Atelier token provisioning is busy")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func releaseProductFlowRedisLock(ctx context.Context, key string, value string) {
	const script = `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) end; return 0`
	_, _ = common.RDB.Eval(ctx, script, []string{key}, value).Result()
}

func createProductFlowToken(userID int, cfg productFlowSSOConfig) (*model.Token, error) {
	key, err := common.GenerateKey()
	if err != nil {
		return nil, err
	}
	token := model.Token{
		UserId:             userID,
		Name:               cfg.TokenName,
		Key:                key,
		Status:             common.TokenStatusEnabled,
		CreatedTime:        common.GetTimestamp(),
		AccessedTime:       common.GetTimestamp(),
		ExpiredTime:        -1,
		RemainQuota:        0,
		UnlimitedQuota:     true,
		ModelLimitsEnabled: false,
		ModelLimits:        "",
		Group:              cfg.TokenGroup,
		CrossGroupRetry:    false,
	}
	if err := token.Insert(); err != nil {
		return nil, err
	}
	return &token, nil
}

func updateProductFlowToken(token *model.Token, cfg productFlowSSOConfig) (*model.Token, error) {
	token.Status = common.TokenStatusEnabled
	token.ExpiredTime = -1
	token.UnlimitedQuota = true
	token.ModelLimitsEnabled = false
	token.ModelLimits = ""
	token.Group = cfg.TokenGroup
	token.CrossGroupRetry = false
	if err := token.Update(); err != nil {
		return nil, err
	}
	return token, nil
}

// buildProductFlowCallbackBaseURL resolves the canonical callback target used
// by both the browser redirect and the status preview so the UI cannot drift
// from the actual redirect semantics when the base URL contains extra path
// segments.
func buildProductFlowCallbackBaseURL(baseURL string) (*url.URL, error) {
	callback, err := url.Parse(common.BuildURL(strings.TrimRight(baseURL, "/")+"/", "/auth/new-api/callback"))
	if err != nil {
		return nil, err
	}
	return callback, nil
}

func buildProductFlowCallbackURL(baseURL string, ticket string) (string, error) {
	callback, err := buildProductFlowCallbackBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	q := callback.Query()
	q.Set("ticket", ticket)
	callback.RawQuery = q.Encode()
	return callback.String(), nil
}
