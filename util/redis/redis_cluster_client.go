package redis

import (
	"errors"
	"strings"

	"github.com/wangfeiping/weeder/log"
	"github.com/wangfeiping/weeder/util"
	redis "gopkg.in/redis.v5"
)

type RedisClusterClient struct {
	Client *redis.ClusterClient
}

func NewRedisClusterClient(redisServer string,
	password string, database int) (client *RedisClusterClient, err error) {
	hostPorts := strings.Split(redisServer, ",")
	if len(hostPorts) < 2 {
		err = errors.New("not cluster client")
		return
	}
	cluesterClient := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs: hostPorts,
	})
	client = &RedisClusterClient{Client: cluesterClient}
	return
}

func (r *RedisClusterClient) GetFileId(filepath string) (fid string, err error) {
	fid, err = r.Client.Get(filepath).Result()
	log.DebugS("redis", "redis cluster client get filepath: ", filepath, " fid: ", fid)
	if err == redis.Nil {
		err = errors.New("fid not found")
	}
	return
}

func (r *RedisClusterClient) GetFileFullPath(fid string) (filepath string, err error) {
	filepath, err = r.Client.Get(fid).Result()
	log.DebugS("redis", "redis cluster client get fid: ", fid, " filepath: ", filepath)
	if err == redis.Nil {
		err = errors.New("filepath not found")
	}
	return
}

func (r *RedisClusterClient) SetPathMeta(path string, meta string) (err error) {
	return r.Client.HSet("weed-meta", path, meta).Err()
}

func (r *RedisClusterClient) CacheFilePath(filepath string, fid string, ttl string) (err error) {
	ttlDuration, e := util.ParseTtlDuration(ttl)
	if e != nil {
		log.ErrorS("redis", "parse ttl error: ", ttl, ", ", e.Error())
	}
	return r.Client.Set(filepath, fid, ttlDuration).Err()
}

func (s *RedisClusterClient) Close() {
	if s.Client != nil {
		s.Client.Close()
	}
}
