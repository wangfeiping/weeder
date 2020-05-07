package server

import (
	"bytes"
	"net/http"
	"strconv"
	"time"

	"github.com/wangfeiping/weeder/log"
)

/**
 * 健康检查接口，
 * 响应内容：timestamp 单位为秒。
 * {"status":"20000000", "message":"ok", "timestamp":1479812829}
 * 根据Resthub API 规范：http://wiki.qianbaoqm.com/pages/viewpage.action?pageId=14190569
 */
func (ps *ProxyServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	logHeader := &log.LogHeader{
		TraceId:    checkGid(r),
		Caddress:   checkRealIp(r),
		UserId:     checkUniSource(r),
		Key:        "request",
		ThreadName: r.URL.Path,
		ClassName:  "health",
		MethodName: r.Method,
	}
	timestamp := time.Now().Unix()
	log.Info(logHeader, `{"uri":"`, r.RequestURI, `"}`)
	var buffer bytes.Buffer
	buffer.WriteString(`{"status":"20000000", "message":"ok", "timestamp":`)
	buffer.WriteString(strconv.FormatInt(timestamp, 10))
	buffer.WriteString("}")
	ret := buffer.String()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(ret))
	w.Write([]byte("\n"))
	logHeader.Status = "ok"
	log.Info(logHeader, ret)
}

/**
 * 服务可用监控接口，服务启动时会自动上传一个图片文件，监控程序可以定时访问该文件。
 * 通过filer上传和获取，以保证可以使用指定路径上传和获得。
 */
func (ps *ProxyServer) echoHandler(w http.ResponseWriter, r *http.Request) {
	logHeader := &log.LogHeader{
		TraceId:    checkGid(r),
		Caddress:   checkRealIp(r),
		UserId:     checkUniSource(r),
		Key:        "request",
		ThreadName: r.URL.Path,
		ClassName:  "echo",
		MethodName: r.Method,
	}
	log.Info(logHeader, `{"uri":"`, r.RequestURI, `"}`)
	logHeader.Status = "ok"
	log.Info(logHeader)
}
