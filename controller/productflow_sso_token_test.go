package controller

import (
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestProductFlowTokenProvisioningIsSerialized(t *testing.T) {
	db := setupProductFlowSSOTestDB(t)
	withProductFlowSSOEnv(t)
	resetProductFlowMemoryTickets(t)
	user := seedProductFlowUser(t, db)
	cfg := getProductFlowSSOConfig()

	const workers = 8
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	tokenIDs := make(chan int, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := getOrCreateProductFlowToken(user.Id, cfg)
			if err != nil {
				errs <- err
				return
			}
			tokenIDs <- token.Id
		}()
	}
	wg.Wait()
	close(errs)
	close(tokenIDs)

	for err := range errs {
		require.NoError(t, err)
	}
	firstID := 0
	for id := range tokenIDs {
		if firstID == 0 {
			firstID = id
		}
		require.Equal(t, firstID, id)
	}

	var count int64
	require.NoError(t, db.Model(&model.Token{}).Where("user_id = ? AND name = ?", user.Id, cfg.TokenName).Count(&count).Error)
	require.EqualValues(t, 1, count)
}
