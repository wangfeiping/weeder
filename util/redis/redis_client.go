package redis

import (
	"errors"

	"github.com/wangfeiping/weeder/log"
	"github.com/wangfeiping/weeder/util"
	redis "gopkg.in/redis.v2"
)

type RedisClient struct {
	Client *redis.Client
}

func NewRedisClient(redisServer string,
	password string, database int) (client *RedisClient, err error) {
	newClient := redis.NewTCPClient(&redis.Options{
		Addr: redisServer,
		//              Password: password,
		//              DB:       int64(database),
	})
	client = &RedisClient{
		Client: newClient,
	}
	return
}

func (r *RedisClient) GetFileId(filepath string) (fid string, err error) {
	//      return r.Client.Get(filepath).Result()
	fid, err = r.Client.Get(filepath).Result()
	log.DebugS("redis", "redis client get filepath: ", filepath, " fid: ", fid)
	if err == redis.Nil {
		err = errors.New("fid not found")
	}
	return
}

func (r *RedisClient) GetFileFullPath(fid string) (filepath string, err error) {
	//      return r.Client.Get(fid).Result()
	filepath, err = r.Client.Get(fid).Result()
	log.DebugS("redis", "redis client get fid: ", fid, " filepath: ", filepath)
	if err == redis.Nil {
		err = errors.New("filepath not found")
	}
	return
}

func (r *RedisClient) SetPathMeta(path string, meta string) (err error) {
	return r.Client.HSet("weed-meta", path, meta).Err()
}

func (r *RedisClient) CacheFilePath(filepath string, fid string, ttl string) (err error) {
	ttlDuration, e := util.ParseTtlDuration(ttl)
	if e != nil {
		log.ErrorS("redis", "parse ttl error: ", ttl, ", ", e.Error())
	}
	return r.Client.SetEx(filepath, ttlDuration, fid).Err()
}

func (r *RedisClient) Close() {
	if r.Client != nil {
		r.Client.Close()
	}
}
