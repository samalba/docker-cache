package main

import (
	"encoding/json"
	"fmt"
	"github.com/fzzy/radix/redis"
	"github.com/samalba/dockerclient"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const CACHE_PREFIX = "docker"
const EVENTS_CHANNEL = "docker_events"

type Cache struct {
	redisConn *redis.Client
	id        string
	ttl       time.Duration
}

func NewCache(cacheUrl string, id string, ttl time.Duration) (*Cache, error) {
	u, err := url.Parse(cacheUrl)
	if err != nil {
		return nil, err
	}
	redisConn, err := redis.DialTimeout("tcp", u.Host,
		time.Duration(30)*time.Second)
	if err != nil {
		return nil, err
	}
	if len(u.Path) > 2 {
		db, err := strconv.Atoi(u.Path[1:])
		if err == nil {
			return nil, fmt.Errorf("Wrong Redis db: %s", err)
		}
		r := redisConn.Cmd("select", db)
		if r.Err != nil {
			return nil, r.Err
		}
	}
	if u.User != nil {
		if pwd, ok := u.User.Password(); ok {
			r := redisConn.Cmd("auth", pwd)
			if r.Err != nil {
				return nil, r.Err
			}
		}
	}
	cache := &Cache{redisConn, id, ttl}
	cache.publishEvent("new_host", id)
	return cache, nil
}

func (cache *Cache) publishEvent(args ...string) error {
	event := strings.Join(args, ":")
	r := cache.redisConn.Cmd("publish", EVENTS_CHANNEL, event)
	if r.Err != nil {
		return r.Err
	}
	return nil
}

func (cache *Cache) getHostTimestamp() int64 {
	localts := time.Now().UTC().Unix()
	r := cache.redisConn.Cmd("time")
	if r.Err != nil {
		// Redis is too old to support the "time" command
		return localts
	}
	ts, err := r.List()
	if err == nil {
		return localts
	}
	// Return time from Redis (all hosts will be synchronized)
	timestamp, err := strconv.ParseInt(ts[0], 10, 64)
	if err != nil {
		return localts
	}
	return timestamp
}

func (cache *Cache) refreshHostLastUpdate() error {
	lastUpdate := strconv.FormatInt(cache.getHostTimestamp(), 10)
	return cache.SetHostParam("last_update", lastUpdate)
}

func (cache *Cache) deleteHost(id string) {
	c := cache.redisConn
	c.Append("multi")
	key := CACHE_PREFIX + ":hosts"
	c.Append("srem", key, id)
	key += ":" + id
	c.Append("del", key)
	key += ":containers"
	c.Append("del", key)
	c.Append("exec")
	for i := 0; i < 5; i++ {
		c.GetReply()
	}
}

func (cache *Cache) ClearExpiredHosts() error {
	r := cache.redisConn.Cmd("smembers", CACHE_PREFIX+":hosts")
	if r.Err != nil {
		return r.Err
	}
	hosts, err := r.List()
	if err != nil {
		return err
	}
	for _, host := range hosts {
		if host == cache.id {
			// ignore myself
			continue
		}
		key := fmt.Sprintf("%s:hosts:%s", CACHE_PREFIX, host)
		r := cache.redisConn.Cmd("hget", key, "last_update")
		if r.Err != nil {
			continue
		}
		lastUpdate, err := r.Int64()
		if err != nil {
			continue
		}
		r = cache.redisConn.Cmd("hget", key, "update_interval")
		if r.Err != nil {
			continue
		}
		updateInterval, err := r.Int64()
		if err != nil {
			continue
		}
		timestamp := cache.getHostTimestamp()
		// If the host missed 2 updates, we consider it expired
		if (timestamp - lastUpdate) < (2 * updateInterval) {
			continue
		}
		cache.deleteHost(host)
		cache.publishEvent("expired_host", host)
	}
	return nil
}

// Converts a struct to a map of strings
func structToMap(st interface{}, out *map[string]string, prefix string) {
	val := reflect.ValueOf(st)
	if reflect.TypeOf(st).Kind() == reflect.Ptr {
		val = val.Elem()
	}
	for i := 0; i < val.NumField(); i++ {
		valueField := val.Field(i)
		typeField := val.Type().Field(i)
		f := valueField.Interface()
		val := reflect.ValueOf(f)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		name := prefix + strings.ToLower(typeField.Name)
		if val.Kind() == reflect.Struct {
			structToMap(f, out, name+"_")
			continue
		}
		switch val.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			(*out)[name] = strconv.FormatInt(val.Int(), 10)
		case reflect.Bool:
			(*out)[name] = strconv.FormatBool(val.Bool())
		case reflect.String:
			(*out)[name] = val.String()
		case reflect.Array, reflect.Slice, reflect.Map:
			b, err := json.Marshal(val.Interface())
			if err != nil {
				continue
			}
			(*out)[name] = string(b)
		}
	}
}

func (cache *Cache) SetContainerInfo(container *dockerclient.ContainerInfo) error {
	m := make(map[string]string)
	structToMap(container, &m, "")
	key := fmt.Sprintf("%s:containers:%s", CACHE_PREFIX, container.Id)
	c := cache.redisConn
	c.Append("multi")
	c.Append("del", key)
	for k, v := range m {
		c.Append("hset", key, k, v)
	}
	c.Append("expire", key, int(cache.ttl.Seconds()))
	c.Append("exec")
	for i := 0; i < len(m)+4; i++ {
		r := c.GetReply()
		if r.Err != nil {
			return r.Err
		}
	}
	// Set the container's json
	key += ":json"
	b, err := json.Marshal(container)
	if err != nil {
		return err
	}
	c.Append("multi")
	c.Append("del", key)
	c.Append("set", key, string(b))
	c.Append("expire", key, int(cache.ttl.Seconds()))
	c.Append("exec")
	for i := 0; i < 5; i++ {
		r := c.GetReply()
		if r.Err != nil {
			return r.Err
		}
	}
	return nil
}

func (cache *Cache) AddContainer(container *dockerclient.ContainerInfo) error {
	err := cache.SetContainerInfo(container)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("%s:hosts:%s", CACHE_PREFIX, cache.id)
	r := cache.redisConn.Cmd("hincrby", key, "containers_running", 1)
	if r.Err != nil {
		return r.Err
	}
	err = cache.refreshHostLastUpdate()
	if err != nil {
		return err
	}
	cache.publishEvent("new_container", cache.id, container.Id)
	return nil
}

func (cache *Cache) DeleteContainer(container *dockerclient.ContainerInfo) error {
	key := fmt.Sprintf("%s:containers:%s", CACHE_PREFIX, container.Id)
	c := cache.redisConn
	c.Append("multi")
	c.Append("del", key)
	key = fmt.Sprintf("%s:hosts:%s:containers", CACHE_PREFIX, cache.id)
	c.Append("srem", key, container.Id)
	key = fmt.Sprintf("%s:hosts:%s", CACHE_PREFIX, cache.id)
	c.Append("hincrby", key, "containers_running", -1)
	c.Append("exec")
	for i := 0; i < 5; i++ {
		r := c.GetReply()
		if r.Err != nil {
			return r.Err
		}
	}
	err := cache.refreshHostLastUpdate()
	if err != nil {
		return err
	}
	cache.publishEvent("delete_container", cache.id, container.Id)
	return nil
}

func (cache *Cache) SetContainersList(containers []dockerclient.Container) error {
	// Store containers list and set containers info
	key := fmt.Sprintf("%s:hosts:%s:containers", CACHE_PREFIX, cache.id)
	c := cache.redisConn
	c.Append("multi")
	c.Append("del", key)
	for _, container := range containers {
		c.Append("sadd", key, container.Id)
	}
	c.Append("expire", key, int(cache.ttl.Seconds()))
	c.Append("exec")
	for i := 0; i < len(containers)+4; i++ {
		r := c.GetReply()
		if r.Err != nil {
			return r.Err
		}
	}
	// Store some host stats
	err := cache.SetHostParam("containers_running", strconv.Itoa(len(containers)))
	if err != nil {
		return err
	}
	err = cache.SetHostParam("update_interval", strconv.Itoa(int(cache.ttl.Seconds())))
	if err != nil {
		return err
	}
	err = cache.refreshHostLastUpdate()
	if err != nil {
		return err
	}
	key = fmt.Sprintf("%s:hosts:%s", CACHE_PREFIX, cache.id)
	r := cache.redisConn.Cmd("expire", key, int(cache.ttl.Seconds()))
	if err != nil {
		return err
	}
	// Register the host in the host list
	key = CACHE_PREFIX + ":hosts"
	r = cache.redisConn.Cmd("sadd", key, cache.id)
	if r.Err != nil {
		return r.Err
	}
	r = cache.redisConn.Cmd("expire", key, int(cache.ttl.Seconds()))
	if r.Err != nil {
		return r.Err
	}
	cache.publishEvent("refresh_containers", cache.id)
	return nil
}

func (cache *Cache) SetHostParam(key string, value string) error {
	hKey := fmt.Sprintf("%s:hosts:%s", CACHE_PREFIX, cache.id)
	r := cache.redisConn.Cmd("hset", hKey, key, value)
	if r.Err != nil {
		return r.Err
	}
	return nil
}
