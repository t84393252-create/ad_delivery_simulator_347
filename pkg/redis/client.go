package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

type Client struct {
	rdb    *redis.Client
	logger *logrus.Logger
	ctx    context.Context
}

func NewClient(addr, password string, db int, logger *logrus.Logger) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     10,
		MinIdleConns: 3,
		MaxRetries:   3,
	})

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Client{
		rdb:    rdb,
		logger: logger,
		ctx:    ctx,
	}, nil
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

func (c *Client) SetCampaignBudget(campaignID string, dailyBudget, totalBudget float64) error {
	pipe := c.rdb.Pipeline()
	
	dailyKey := fmt.Sprintf("campaign:budget:daily:%s", campaignID)
	totalKey := fmt.Sprintf("campaign:budget:total:%s", campaignID)
	
	pipe.Set(c.ctx, dailyKey, dailyBudget, 24*time.Hour)
	pipe.Set(c.ctx, totalKey, totalBudget, 0)
	
	_, err := pipe.Exec(c.ctx)
	return err
}

func (c *Client) DecrementBudget(campaignID string, amount float64) (bool, error) {
	dailyKey := fmt.Sprintf("campaign:budget:daily:%s", campaignID)
	totalKey := fmt.Sprintf("campaign:budget:total:%s", campaignID)
	
	script := `
		local daily_key = KEYS[1]
		local total_key = KEYS[2]
		local amount = tonumber(ARGV[1])
		
		local daily_budget = redis.call('get', daily_key)
		local total_budget = redis.call('get', total_key)
		
		if not daily_budget or not total_budget then
			return 0
		end
		
		daily_budget = tonumber(daily_budget)
		total_budget = tonumber(total_budget)
		
		if daily_budget < amount or total_budget < amount then
			return 0
		end
		
		redis.call('incrbyfloat', daily_key, -amount)
		redis.call('incrbyfloat', total_key, -amount)
		return 1
	`
	
	result, err := c.rdb.Eval(c.ctx, script, []string{dailyKey, totalKey}, amount).Int()
	if err != nil {
		return false, err
	}
	
	return result == 1, nil
}

func (c *Client) IncrementFrequencyCap(userID, campaignID string, eventType string, window time.Duration) (int64, error) {
	key := fmt.Sprintf("freq:%s:%s:%s", eventType, campaignID, userID)
	
	pipe := c.rdb.Pipeline()
	count := pipe.Incr(c.ctx, key)
	pipe.Expire(c.ctx, key, window)
	
	_, err := pipe.Exec(c.ctx)
	if err != nil {
		return 0, err
	}
	
	return count.Val(), nil
}

func (c *Client) GetFrequencyCount(userID, campaignID string, eventType string) (int64, error) {
	key := fmt.Sprintf("freq:%s:%s:%s", eventType, campaignID, userID)
	count, err := c.rdb.Get(c.ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

func (c *Client) AddBidToAuction(auctionID string, bid interface{}, expiry time.Duration) error {
	bidJSON, err := json.Marshal(bid)
	if err != nil {
		return err
	}
	
	key := fmt.Sprintf("auction:%s:bids", auctionID)
	score := time.Now().UnixNano()
	
	pipe := c.rdb.Pipeline()
	pipe.ZAdd(c.ctx, key, &redis.Z{
		Score:  float64(score),
		Member: bidJSON,
	})
	pipe.Expire(c.ctx, key, expiry)
	
	_, err = pipe.Exec(c.ctx)
	return err
}

func (c *Client) GetTopBids(auctionID string, limit int64) ([]string, error) {
	key := fmt.Sprintf("auction:%s:bids", auctionID)
	return c.rdb.ZRevRange(c.ctx, key, 0, limit-1).Result()
}

func (c *Client) IncrementMetric(metricType, campaignID string) error {
	dayKey := fmt.Sprintf("metrics:%s:%s:%s", metricType, campaignID, time.Now().Format("2006-01-02"))
	hourKey := fmt.Sprintf("metrics:%s:%s:%s", metricType, campaignID, time.Now().Format("2006-01-02:15"))
	
	pipe := c.rdb.Pipeline()
	pipe.Incr(c.ctx, dayKey)
	pipe.Expire(c.ctx, dayKey, 7*24*time.Hour)
	pipe.Incr(c.ctx, hourKey)
	pipe.Expire(c.ctx, hourKey, 24*time.Hour)
	
	_, err := pipe.Exec(c.ctx)
	return err
}

func (c *Client) GetMetrics(metricType, campaignID string, date string) (int64, error) {
	key := fmt.Sprintf("metrics:%s:%s:%s", metricType, campaignID, date)
	count, err := c.rdb.Get(c.ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

func (c *Client) PublishEvent(channel string, event interface{}) error {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return err
	}
	
	return c.rdb.Publish(c.ctx, channel, eventJSON).Err()
}

func (c *Client) Subscribe(channel string) *redis.PubSub {
	return c.rdb.Subscribe(c.ctx, channel)
}

func (c *Client) SetPacingRate(campaignID string, rate float64, ttl time.Duration) error {
	key := fmt.Sprintf("pacing:%s", campaignID)
	return c.rdb.Set(c.ctx, key, rate, ttl).Err()
}

func (c *Client) GetPacingRate(campaignID string) (float64, error) {
	key := fmt.Sprintf("pacing:%s", campaignID)
	rate, err := c.rdb.Get(c.ctx, key).Float64()
	if err == redis.Nil {
		return 1.0, nil
	}
	return rate, err
}

func (c *Client) CacheBidRequest(requestID string, request interface{}, ttl time.Duration) error {
	key := fmt.Sprintf("bidrequest:%s", requestID)
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	return c.rdb.Set(c.ctx, key, data, ttl).Err()
}

func (c *Client) GetCachedBidRequest(requestID string) ([]byte, error) {
	key := fmt.Sprintf("bidrequest:%s", requestID)
	return c.rdb.Get(c.ctx, key).Bytes()
}

func (c *Client) RateLimitCheck(identifier string, limit int, window time.Duration) (bool, error) {
	key := fmt.Sprintf("ratelimit:%s", identifier)
	
	script := `
		local key = KEYS[1]
		local limit = tonumber(ARGV[1])
		local window = tonumber(ARGV[2])
		local current = redis.call('incr', key)
		
		if current == 1 then
			redis.call('expire', key, window)
		end
		
		if current > limit then
			return 0
		end
		return 1
	`
	
	result, err := c.rdb.Eval(c.ctx, script, []string{key}, limit, int(window.Seconds())).Int()
	if err != nil {
		return false, err
	}
	
	return result == 1, nil
}