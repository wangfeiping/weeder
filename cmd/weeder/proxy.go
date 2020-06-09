package main

import (
	"net/http"
	"runtime"
	"strconv"

	"github.com/wangfeiping/weeder/log"
	"github.com/wangfeiping/weeder/server"
	"github.com/wangfeiping/weeder/util"
)

var (
	mTimeout = 10 //seconds
	mMaxCpu  = 0  //maximum number of CPUs. 0 means all available CPUs
)

func serv(config *util.WeederConfig) bool {
	if mMaxCpu < 1 {
		mMaxCpu = runtime.NumCPU()
	}
	runtime.GOMAXPROCS(mMaxCpu)

	listeningAddress := config.Ip + ":" + strconv.Itoa(config.Port)

	log.DebugS("main", "config: version ", util.VERSION)

	_ = server.NewProxyServer(config)
	e := http.ListenAndServe(listeningAddress, nil)
	if e != nil {
		log.ErrorS("main", "startup error: ", e)
		return false
	}
	return true
}
