package main

import (
	"sync"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/go-redis/redis"
)

// RedisCache represents a redis cache object that conforms to the Cache interface
type RedisCache struct {
	T         *TricksterHandler
	Config    RedisConfig
	client    *redis.Client
	CacheKeys sync.Map
}

// Connect connects to the configured Redis endpoint
func (r *RedisCache) Connect() error {

	// Connect to Redis
	level.Info(r.T.Logger).Log("event", "connecting to redis", "protocol", r.Config.Protocol, "Endpoint", r.Config.Endpoint)
	r.client = redis.NewClient(&redis.Options{
		Network: r.Config.Protocol,
		Addr:    r.Config.Endpoint,
	})
	return r.client.Ping().Err()

}

// Store places the the data into the Redis Cache using the provided Key and TTL
func (r *RedisCache) Store(cacheKey string, data string, ttl int64) error {
	level.Debug(r.T.Logger).Log("event", "redis cache store", "key", cacheKey)
	return r.client.Set(cacheKey, data, time.Second*time.Duration(ttl)).Err()
}

// Retrieve gets data from the Redis Cache using the provided Key
func (r *RedisCache) Retrieve(cacheKey string) (string, error) {
	level.Debug(r.T.Logger).Log("event", "redis cache retrieve", "key", cacheKey)
	return r.client.Get(cacheKey).Result()
}

// Reap continually iterates through the cache to find expired elements and removes them
func (r *RedisCache) Reap() {

	for {

		var keys []string

		// Get a lock to enumerate the keys without r/w collisions
		r.T.ChannelCreateMtx.Lock()
		for key, _ := range r.T.ResponseChannels {
			keys = append(keys, key)
		}
		// Unlock
		r.T.ChannelCreateMtx.Unlock()

		for _, key := range keys {

			// check if the channel has a corresponding redis key
			_, err := r.client.Get(key).Result()
			if err == redis.Nil {

				// Query Results expired and are not in Redis anymore, close the channel
				level.Debug(r.T.Logger).Log("event", "redis cache reap", "key", key)

				// Get a lock (again) to clear the channel without map collsions
				r.T.ChannelCreateMtx.Lock()

				// Close out the channel if it exists
				if _, ok := r.T.ResponseChannels[key]; ok {
					close(r.T.ResponseChannels[key])
					delete(r.T.ResponseChannels, key)
				}

				// Unlock
				r.T.ChannelCreateMtx.Unlock()

			}
		}

		time.Sleep(time.Duration(r.T.Config.Caching.ReapSleepMS) * time.Millisecond)
	}
}

// Close disconnects from the Redis Cache
func (r *RedisCache) Close() error {
	level.Info(r.T.Logger).Log("event", "closing redis connection")
	r.client.Close()
	return nil

}
