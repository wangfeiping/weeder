package util

import (
	"encoding/json"
	"io/ioutil"
	"strconv"
	"time"

	"github.com/wangfeiping/weeder/util/mysql"
)

type Server struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	Type string `json:"type"` // master/volume/filer/空时默认为volume
}

type RedisConfig struct {
	Addr     string `json:"addr"`
	Password string `json:"password"`
	Database int    `json:"database"`
}

type QiniuConfig struct {
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
	Zone      int    `json:"zone"`
	Bucket    string `json:"bucket"`
}

const (
	//stored unit types
	Empty byte = iota
	Minute
	Hour
	Day
	Week
	Month
	Year
)

type WeederConfig struct {
	Ip                  string            `json:"ip"`
	Port                int               `json:"port"`
	Server              []Server          `json:"server"`
	MaxIdleConnsPerHost int               `json:"maxIdleConnsPerHost"`
	Retry               int32             `json:"retry"`
	LogHost             string            `json:"logHost"`
	FileUrlPrefix       string            `json:"fileUrlPrefix"`
	Redis               RedisConfig       `json:"redis"`
	UploadWhite         []string          `json:"uploadWhite"`
	FilerWhite          []string          `json:"filerWhite"`
	UniSourceCheck      bool              `json:"uniSourceCheck"`
	Shadow              []Server          `json:"shadow"`
	RedisCacheTtl       string            `json:"redisCacheTtl"`
	UnkonwnUriChecker   string            `json:"unkonwnUriChecker"`
	Mysql               mysql.MysqlConfig `json:"mysql"`
	Qiniu               QiniuConfig       `json:"qiniu"`
	DebugDetailLog      bool              `json:"debugDetailLog"`
	DevEnvEnforcedTtl   string            `json:"devEnvEnforcedTtl"`
	VolumeCheckDuration int               `json:"volumeCheckDuration"`
	VolumeCheckUrl      string            `json:"volumeCheckUrl"`
	VolumeCheckBaseLine int               `json:"volumeCheckBaseLine"`
	NodeCheckBaseLine   int               `json:"nodeCheckBaseLine"`
}

/**
 * 读取配置，如果配置文件不存在则生成默认配置
 */
func LoadConfig(filename string) (conf *WeederConfig, err error) {
	var bytes []byte
	bytes, err = ioutil.ReadFile(filename)
	if err != nil {
		str := `{ "ip": "0.0.0.0",
				  "port": 9330,
				  "retry": 3,
				  "maxIdleConnsPerHost": 100,
				  "logHost":"130dev",
				  "fileUrlPrefix":"http://172.28.32.130:9000",
				  "redis":{
						"addr": "10.19.34.77:6379,shadow://redis.haodaibao.com:6379",
						"password":"asd",
						"database":1
				  },
				  "uploadWhite":[
						"192.168.1.182/32",
						"172.28.32.31/32",
						"10.19.33.237/32",
						"10.19.33.238/32",
						"10.19.33.232/32",
						"10.19.33.231/32"
				  ],
				  "filerWhite":[
						"0.0.0.0/8",
						"10.0.0.0/8",
						"127.0.0.0/8",
						"169.254.0.0/16",
						"172.16.0.0/12",
						"192.0.0.0/29", "192.0.0.170/31", "192.0.2.0/24", "192.168.0.0/16",
						"198.18.0.0/15", "198.51.100.0/24",
						"203.0.113.0/24",
						"240.0.0.0/4",
						"255.255.255.255/32"
				  ],
				  "server" : [
						{"host":"192.168.1.185", "port":9333, "type":"master"},
						{"host":"192.168.1.185", "port":9360, "type":"volume"},
						{"host":"192.168.1.185", "port":9380, "type":"filer"},
						{"host":"192.168.1.186", "port":9333, "type":"master"},
						{"host":"192.168.1.186", "port":9360, "type":"volume"},
						{"host":"192.168.1.186", "port":9380, "type":"filer"},
						{"host":"192.168.1.187", "port":9333, "type":"master"},
						{"host":"192.168.1.187", "port":9360, "type":"volume"},
						{"host":"192.168.1.187", "port":9380, "type":"filer"}
                ]}`
		bytes = []byte(str)
		json.Unmarshal(bytes, &conf)
	} else {
		err = json.Unmarshal(bytes, &conf)
	}
	return
}

// 3m: 3 minutes
// 4h: 4 hours
// 5d: 5 days
// 6w: 6 weeks
// 7M: 7 months
// 8y: 8 years
func ParseTtlDuration(ttlString string) (time.Duration, error) {
	if ttlString == "" {
		ttlString = "1M"
	}
	ttlBytes := []byte(ttlString)
	unitByte := ttlBytes[len(ttlBytes)-1]
	countBytes := ttlBytes[0 : len(ttlBytes)-1]
	if '0' <= unitByte && unitByte <= '9' {
		countBytes = ttlBytes
		unitByte = 'm'
	}
	count, err := strconv.ParseInt(string(countBytes), 10, 0)
	unit := toTtlUnit(unitByte)
	return time.Duration(count) * unit, err
}

func toTtlUnit(readableUnitByte byte) time.Duration {
	switch readableUnitByte {
	case 'm':
		return time.Minute
	case 'h':
		return time.Hour
	case 'd':
		return time.Hour * 24
	case 'w':
		return time.Hour * 24 * 7
	case 'M':
		return time.Hour * 24 * 31
	case 'y':
		return time.Hour * 24 * 366
	}
	return 0
}
