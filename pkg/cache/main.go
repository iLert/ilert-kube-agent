package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/iLert/ilert-kube-agent/pkg/utils"
	ccache "github.com/karlseguin/ccache/v2"
	"github.com/rs/zerolog/log"
)

const defaultTimeout = 500 * time.Millisecond

// Cache interface
var Cache cacheInterface

type cacheInterface struct {
	Events cacheClientInterface
}

type cacheClientInterface struct {
	Client    *redis.Client
	lruClient *ccache.Cache
}

func (rc *cacheInterface) Init() {
	if utils.GetEnv("REDIS_ENABLED", "") == "true" {
		log.Debug().Msg("Initializing Redis cache")
		redisHost := utils.GetEnv("REDIS_HOST", "localhost")
		redisPort := utils.GetEnv("REDIS_PORT", "6379")
		redisPassword := utils.GetEnv("REDIS_PASSWORD", "")
		rc.Events.Client = redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", redisHost, redisPort),
			Password: redisPassword,
			DB:       0,
		})
		log.Debug().Msg("Redis cache initialized")
	}
	rc.Events.lruClient = ccache.New(ccache.Configure().MaxSize(5000).ItemsToPrune(500))
}

// CheckInitialization checks init
func (rc *cacheClientInterface) CheckInitialization() bool {
	if rc.Client == nil && rc.lruClient == nil {
		log.Fatal().Msg(" Cache not initialized")
		return false
	}
	return true
}

// SetItem sets item value to cache by key
func (rc *cacheClientInterface) SetItem(key string, in string, ttl time.Duration) error {
	if !rc.CheckInitialization() {
		return errors.New(" Redis Cache is not initialized ")
	}

	if rc.Client == nil {
		rc.lruClient.Set(key, in, ttl)
		return nil
	}

	ctx, cancelFn := context.WithTimeout(context.TODO(), defaultTimeout)
	defer cancelFn()

	err := rc.Client.Set(ctx, key, in, ttl).Err()
	if err != nil {
		log.Error().Err(err).Str("key", key).Msg("Failed to set cache item")
		return err
	}
	log.Debug().Str("key", key).Msg("Set cache item")
	return nil
}

// GetItem gets item string from cache by key
func (rc *cacheClientInterface) GetItem(key string) (string, error) {
	if !rc.CheckInitialization() {
		return "", errors.New(" Cache is not initialized ")
	}

	if rc.Client == nil {
		item := rc.lruClient.Get(key)
		if item == nil {
			log.Debug().Msg("Cache item is not found ")
			return "", errors.New("Cache item is not found ")
		}
		if item.Expired() {
			log.Debug().Msg("Cache item is expired ")
			return "", errors.New("Cache item is expired ")
		}
		if itemValue, ok := item.Value().(string); ok && itemValue != "" {
			log.Debug().Str("key", key).Msg("Get cache item")
			return itemValue, nil
		}
		return "", errors.New("Cache item failed to convert to string ")
	}

	ctx, cancelFn := context.WithTimeout(context.TODO(), defaultTimeout)
	defer cancelFn()

	item, err := rc.Client.Get(ctx, key).Result()
	if item == "" || err == redis.Nil {
		log.Debug().Str("key", key).Msg("Cache item is not found ")
		return "", nil
	}
	log.Debug().Str("key", key).Msg("Get cache item")
	return item, nil
}

// GetInt64Item gets item value from cache by key
func (rc *cacheClientInterface) GetInt64Item(key string) (int64, error) {
	if !rc.CheckInitialization() {
		return 0, errors.New(" Cache is not initialized ")
	}

	if rc.Client == nil {
		item := rc.lruClient.Get(key)
		if item == nil {
			log.Debug().Msg("Cache item is not found ")
			return 0, errors.New("Cache item is not found ")
		}
		if item.Expired() {
			log.Debug().Msg("Cache item is expired ")
			return 0, errors.New("Cache item is expired ")
		}
		if itemValue, ok := item.Value().(int64); ok {
			log.Debug().Str("key", key).Msg("Get cache item")
			return itemValue, nil
		}
		return 0, errors.New("Cache item failed to convert to string ")
	}

	ctx, cancelFn := context.WithTimeout(context.TODO(), defaultTimeout)
	defer cancelFn()

	item, err := rc.Client.Get(ctx, key).Int64()
	if err == redis.Nil {
		log.Debug().Str("key", key).Msg("Cache item is not found ")
		return 0, nil
	}
	log.Debug().Str("key", key).Msg("Get cache item")
	return item, nil
}

// DeleteItem delete item from cache by key
func (rc *cacheClientInterface) DeleteItem(key string) error {
	if !rc.CheckInitialization() {
		return errors.New(" Redis is not initialized ")
	}

	if rc.Client == nil {
		success := rc.lruClient.Delete(key)
		if !success {
			log.Debug().Str("key", key).Msg("Failed to delete cache item")
			return errors.New("Cache failed to delete item ")
		}
		return nil
	}

	ctx, cancelFn := context.WithTimeout(context.TODO(), defaultTimeout)
	defer cancelFn()

	_, err := rc.Client.Del(ctx, key).Result()
	if err != nil {
		log.Debug().Err(err).Str("key", key).Msg("Failed to del cache item")
		return err
	}

	log.Debug().Str("key", key).Msg("Delete cache item")
	return nil
}

// IncrementItem increment item by n
func (rc *cacheClientInterface) IncrementItemBy(key string, val int64, ttl time.Duration) error {
	if !rc.CheckInitialization() {
		return errors.New(" Cache is not initialized ")
	}

	if rc.Client == nil {
		rc.lruClient.Set(key, val, ttl)
		return nil
	}

	ctx, cancelFn := context.WithTimeout(context.TODO(), defaultTimeout)
	defer cancelFn()

	_, err := rc.Client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.IncrBy(ctx, key, val)
		pipe.Expire(ctx, key, ttl)
		return nil
	})

	return err
}
