package server

import (
	"encoding/json"
	"net/http"

	"github.com/wangfeiping/weeder/log"
)

type FileQueryResult struct {
	Status int    `json:"status"`
	Id     string `json:"fid"`
	Path   string `json:"path"`
	Error  string `json:"error,omitempty"`
}

/**
 * 使用文件标识fid 查询文件路径
 */
func (ps *ProxyServer) queryFilePathHandler(w http.ResponseWriter, r *http.Request,
	logHeader *log.LogHeader) {
	logHeader.ClassName = "queryFilepath"
	log.Info(logHeader, `{"uri":"`, r.RequestURI, `"}`)
	ret := FileQueryResult{Id: r.URL.Path[1:]}
	//	fid := "4,a8f34c3a4c"
	//	ret := FileQueryResult{Id: fid}
	var err error

	ret.Path, err = dbclient.GetFileFullPath(ret.Id)
	if err != nil {
		ret.Error = err.Error()
		ret.Status = http.StatusNotFound
	} else {
		ret.Status = http.StatusOK
	}
	logHeader.Key = "response"
	if bs, e := json.Marshal(&ret); e == nil {
		w.WriteHeader(http.StatusOK)
		w.Write(bs)
		logHeader.Status = "ok"
		log.Info(logHeader, string(bs))
	} else {
		logHeader.Status = "err"
		log.Error(logHeader, " id=", ret.Id, " path=", ret.Path, " err=", e.Error())
	}
}

/**
 * 使用文件路径查询文件标识fid
 */
func (ps *ProxyServer) queryFileIdHandler(w http.ResponseWriter, r *http.Request,
	logHeader *log.LogHeader) {
	logHeader.ClassName = "getFileid"
	log.Info(logHeader, `{"uri":"`, r.RequestURI, `"}`)
	ret := FileQueryResult{Path: r.URL.Path}
	var err error

	ret.Id, err = dbclient.GetFileId(ret.Path)
	if err != nil {
		ret.Error = err.Error()
		ret.Status = http.StatusNotFound
	} else {
		ret.Status = http.StatusOK
	}
	logHeader.Key = "response"
	if bs, e := json.Marshal(&ret); e == nil {
		w.WriteHeader(ret.Status)
		w.Write(bs)
		logHeader.Status = "ok"
		log.Error(logHeader, string(bs))
	} else {
		w.WriteHeader(ret.Status)
		w.Write([]byte(""))
		logHeader.Status = "err"
		log.Error(logHeader, " id=", ret.Id, " path=", ret.Path, " err=", e.Error())
	}
}
