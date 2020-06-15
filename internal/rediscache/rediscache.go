package rediscache

import (
	"fmt"
	"regexp"
	"time"
	"webchat/internal/config"
	"webchat/internal/logger"

	"github.com/gomodule/redigo/redis"
)

var pool *redis.Pool

var ttl int

var log *logger.Logger

func init() {
	log = logger.NewLogger("webchat")
}

// NewRedisCache is to set the configuration for redis
func NewRedisCache(cfg config.RedisConfig) {
	pool = newPool(cfg)
	ttl = cfg.TTL

	log.I("Redis: %v:%v (ttl: %v)", cfg.Host, cfg.Port, cfg.TTL)
}

// Close to disconnect the connection of redis
func Close() {
	pool.Close()
}

func newPool(cfg config.RedisConfig) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     cfg.PoolMaxIdle,
		MaxActive:   cfg.PoolMaxActive,
		IdleTimeout: time.Duration(cfg.PoolIdleTimeout) * time.Second,
		Wait:        true,
		Dial: func() (redis.Conn, error) {
			url := "redis://" + cfg.Host + ":" + cfg.Port
			return redis.DialURL(
				url,
				redis.DialPassword(cfg.Password),
				redis.DialConnectTimeout(time.Duration(cfg.ConnTimeout)*time.Millisecond),
			)
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}
			_, err := c.Do("PING")
			return err
		},
	}
}

// GetCache is to get the data from redis
func GetCache(key string) (string, error) {
	c := pool.Get()
	defer c.Close()

	raw, err := redis.String(c.Do("GET", key))
	if err == redis.ErrNil {
		return raw, nil
	} else if err != nil {
		return raw, err
	}

	return raw, err
}

// SetCache is to record the data in redis
func SetCache(key string, raw []byte, ttl int) (interface{}, error) {
	c := pool.Get()
	defer c.Close()

	//	log.D("key: %s, value: %+v, ttl: %v", key, string(raw), ttl)

	if ttl == 0 {
		return c.Do("SET", key, raw)
	} else {
		return c.Do("SETEX", key, ttl, raw)
	}
}

// PushList is to record the data in redis
func PushList(key string, raw []byte) (interface{}, error) {
	c := pool.Get()
	defer c.Close()

	//	log.D("RPUSH: key: %s, value: %v", key, string(raw))

	return c.Do("RPUSH", key, raw)
}

// GetList is to record the data in redis
func GetList(key string) ([]string, error) {
	c := pool.Get()
	defer c.Close()

	raw, err := redis.Strings(c.Do("LRANGE", key, 0, -1))
	log.D("raw: %v", raw)
	if err == redis.ErrNil {
		return raw, nil
	} else if err != nil {
		return raw, err
	}

	return raw, err
}

// GetPrefixValues is to get values from prefix key
func GetPrefixValues(prefix string) []string {
	var keys []string

	c := pool.Get()
	defer c.Close()

	pattern := prefix + "*"

	raw, err := redis.MultiBulk(c.Do("SCAN", 0, "COUNT", 1000, "MATCH", pattern))
	if err == redis.ErrNil {
		log.E("%v", err)
	} else if err != nil {
		log.E("%v", err)
	}

	for i := 1; i < len(raw); i++ {
		value := fmt.Sprintf("%s", raw[i])

		r := regexp.MustCompile("[^\\s]+")
		ids := r.FindAllString(string(value[1:len(value)-1]), -1)

		for j := range ids {
			keys = append(keys, ids[j])
		}
	}

	return keys
}

// Publish is to send data in redis PUBSUB
func Publish(key string, raw []byte) (interface{}, error) {
	c := pool.Get()
	defer c.Close()

	// log.D("PUBLISH: %s %v", key, string(raw))

	return c.Do("PUBLISH", key, raw)
}

// Subscribe is to get message event in redis
func Subscribe(channel string, d chan []byte, quit chan struct{}) error {
	c := pool.Get()
	defer c.Close()

	var needQuit = false

	go func() {
		//		log.D("Check quit!")
		select {
		case <-quit:
			log.D("Unsubscribe is required")
			needQuit = true
			//psc.Unsubscribe()
			//c.Close()
			return
		}
	}()

	go func(q bool) {
		// Get a connection from a pool
		c := pool.Get()
		psc := redis.PubSubConn{Conn: c}

		// Set up subscriptions
		if err := psc.Subscribe(channel); err != nil {
			log.E("Subscribe error: %v", err.Error)
		}

		// While not a permanent error on the connection.
		for c.Err() == nil {
			switch v := psc.Receive().(type) {
			case redis.Message:
				d <- v.Data
				// log.D("message: %s %s\n", v.Channel, v.Data)

				if q {
					log.D("Unsubscribed")
					return
				}
			case redis.Subscription:
				log.D("subscribed: %s %s %d\n", v.Channel, v.Kind, v.Count)
			case error:
				log.E("%v", error.Error)
				return
			}
		}

		psc.Unsubscribe()
		c.Close()
	}(needQuit)

	return nil
}

// Del deletes key.
func Del(key string) error {
	c := pool.Get()
	defer c.Close()

	_, err := c.Do("DEL", key)
	return err
}
