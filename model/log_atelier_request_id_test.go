package model

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func logTestGinContext(t *testing.T, username string, requestId string, upstreamRequestId string, atelierRequestId string) *gin.Context {
	t.Helper()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("username", username)
	ctx.Set(common.RequestIdKey, requestId)
	ctx.Set(common.UpstreamRequestIdKey, upstreamRequestId)
	ctx.Set(common.AtelierRequestIdKey, atelierRequestId)
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	return ctx
}

func insertLogUser(t *testing.T, id int, username string) {
	t.Helper()

	require.NoError(t, DB.Create(&User{
		Id:       id,
		Username: username,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
}

func TestRecordConsumeLogStoresAtelierRequestId(t *testing.T) {
	truncateTables(t)
	insertLogUser(t, 701, "atelier-consume")
	ctx := logTestGinContext(t, "atelier-consume", "req-consume", "upstream-consume", "atr_consume-1")

	RecordConsumeLog(ctx, 701, RecordConsumeLogParams{
		ChannelId:        1,
		PromptTokens:     10,
		CompletionTokens: 20,
		ModelName:        "gpt-image-2",
		TokenName:        "Atelier",
		Quota:            123,
		Content:          "consume",
		TokenId:          55,
		UseTimeSeconds:   2,
		Group:            "atelier",
		Other:            map[string]interface{}{"safe": true},
	})

	var log Log
	require.NoError(t, LOG_DB.First(&log, "user_id = ? AND type = ?", 701, LogTypeConsume).Error)
	require.Equal(t, "atr_consume-1", log.AtelierRequestId)
	require.Equal(t, "req-consume", log.RequestId)
	require.Equal(t, "upstream-consume", log.UpstreamRequestId)
}

func TestRecordErrorLogStoresAtelierRequestId(t *testing.T) {
	truncateTables(t)
	insertLogUser(t, 702, "atelier-error")
	ctx := logTestGinContext(t, "atelier-error", "req-error", "upstream-error", "atr_error-1")

	RecordErrorLog(ctx, 702, 2, "gpt-image-2", "Atelier", "provider failed", 56, 3, false, "atelier", nil)

	var log Log
	require.NoError(t, LOG_DB.First(&log, "user_id = ? AND type = ?", 702, LogTypeError).Error)
	require.Equal(t, "atr_error-1", log.AtelierRequestId)
	require.Equal(t, "req-error", log.RequestId)
	require.Equal(t, "upstream-error", log.UpstreamRequestId)
}

func TestGetLogsFilterByAtelierRequestId(t *testing.T) {
	truncateTables(t)
	require.NoError(t, LOG_DB.Create(&[]Log{
		{
			UserId:           801,
			Username:         "alice",
			Type:             LogTypeConsume,
			CreatedAt:        common.GetTimestamp(),
			ModelName:        "gpt-image-2",
			AtelierRequestId: "atr_shared",
		},
		{
			UserId:           802,
			Username:         "bob",
			Type:             LogTypeConsume,
			CreatedAt:        common.GetTimestamp(),
			ModelName:        "gpt-image-2",
			AtelierRequestId: "atr_other",
		},
	}).Error)

	allLogs, total, err := GetAllLogs(LogTypeConsume, 0, 0, "", "", "", 0, 10, 0, "", "", "", "atr_shared")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, allLogs, 1)
	require.Equal(t, 801, allLogs[0].UserId)

	userLogs, total, err := GetUserLogs(802, LogTypeConsume, 0, 0, "", "", 0, 10, "", "", "", "atr_shared")
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, userLogs)
}

func TestGetLogByTokenIdAndAtelierRequestIdPreservesRealIdAndScopesToken(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	require.NoError(t, LOG_DB.Create(&[]Log{
		{
			Id:                9101,
			UserId:            901,
			Username:          "alice",
			TokenId:           61,
			TokenName:         "Atelier",
			Type:              LogTypeConsume,
			CreatedAt:         now,
			ModelName:         "gpt-image-2",
			Quota:             123,
			PromptTokens:      11,
			CompletionTokens:  22,
			UseTime:           3,
			RequestId:         "req-token-a",
			UpstreamRequestId: "up-token-a",
			AtelierRequestId:  "atr_token_shared",
			Group:             "atelier",
			Other:             `{"admin_info":{"secret":true},"safe":true}`,
		},
		{
			Id:               9102,
			UserId:           902,
			Username:         "bob",
			TokenId:          62,
			Type:             LogTypeConsume,
			CreatedAt:        now,
			ModelName:        "gpt-image-2",
			AtelierRequestId: "atr_token_shared",
		},
	}).Error)

	logs, err := GetLogByTokenIdAndAtelierRequestId(61, "atr_token_shared")
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, 9101, logs[0].Id)
	require.Equal(t, 61, logs[0].TokenId)
	require.Equal(t, "req-token-a", logs[0].RequestId)
	require.NotContains(t, logs[0].Other, "admin_info")
	require.Contains(t, logs[0].Other, "safe")

	otherTokenLogs, err := GetLogByTokenIdAndAtelierRequestId(63, "atr_token_shared")
	require.NoError(t, err)
	require.Empty(t, otherTokenLogs)
}
