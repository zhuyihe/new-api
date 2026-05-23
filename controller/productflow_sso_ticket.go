package controller

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/go-redis/redis/v8"
)

const productFlowTicketKeyPrefix = "productflow:sso:ticket:"

type productFlowMemoryTicket struct {
	Claims    productFlowTicketClaims
	ExpiresAt time.Time
}

var productFlowMemoryTickets = struct {
	sync.Mutex
	items map[string]productFlowMemoryTicket
}{items: map[string]productFlowMemoryTicket{}}

func storeProductFlowTicket(ticket string, claims productFlowTicketClaims, ttl time.Duration) error {
	if ticket == "" {
		return errors.New("empty ticket")
	}
	if common.RedisEnabled && common.RDB != nil {
		payload, err := common.Marshal(claims)
		if err != nil {
			return err
		}
		// Ticket payload contains token material; avoid RedisSet debug logging.
		return common.RDB.Set(context.Background(), productFlowTicketKeyPrefix+ticket, string(payload), ttl).Err()
	}
	productFlowMemoryTickets.Lock()
	defer productFlowMemoryTickets.Unlock()
	pruneExpiredProductFlowTicketsLocked(time.Now())
	productFlowMemoryTickets.items[ticket] = productFlowMemoryTicket{
		Claims:    claims,
		ExpiresAt: time.Now().Add(ttl),
	}
	return nil
}

func consumeProductFlowTicket(ticket string) (productFlowTicketClaims, error) {
	if ticket == "" {
		return productFlowTicketClaims{}, errors.New("missing ticket")
	}
	if common.RedisEnabled && common.RDB != nil {
		return consumeProductFlowTicketFromRedis(ticket)
	}
	return consumeProductFlowTicketFromMemory(ticket)
}

func consumeProductFlowTicketFromRedis(ticket string) (productFlowTicketClaims, error) {
	const script = `local value = redis.call("GET", KEYS[1]); if value then redis.call("DEL", KEYS[1]); end; return value`
	result, err := common.RDB.Eval(context.Background(), script, []string{productFlowTicketKeyPrefix + ticket}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return productFlowTicketClaims{}, errors.New("ticket is invalid or expired")
		}
		return productFlowTicketClaims{}, err
	}
	payload, ok := result.(string)
	if !ok || payload == "" {
		return productFlowTicketClaims{}, errors.New("ticket is invalid or expired")
	}
	var claims productFlowTicketClaims
	if err := common.UnmarshalJsonStr(payload, &claims); err != nil {
		return productFlowTicketClaims{}, err
	}
	return claims, nil
}

func consumeProductFlowTicketFromMemory(ticket string) (productFlowTicketClaims, error) {
	productFlowMemoryTickets.Lock()
	defer productFlowMemoryTickets.Unlock()
	now := time.Now()
	record, ok := productFlowMemoryTickets.items[ticket]
	if !ok || record.ExpiresAt.Before(now) {
		delete(productFlowMemoryTickets.items, ticket)
		return productFlowTicketClaims{}, errors.New("ticket is invalid or expired")
	}
	delete(productFlowMemoryTickets.items, ticket)
	return record.Claims, nil
}

func pruneExpiredProductFlowTicketsLocked(now time.Time) {
	for ticket, record := range productFlowMemoryTickets.items {
		if record.ExpiresAt.Before(now) {
			delete(productFlowMemoryTickets.items, ticket)
		}
	}
}
