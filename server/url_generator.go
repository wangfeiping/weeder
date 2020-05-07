package server

import (
	"errors"
	"net/http"
	"strings"
)

func (ps *ProxyServer) getFileUrl(uri string, isFiler bool,
	round int32) string {
	weedLen := len(ps.Weeds)
	var weed *Weed
	if weedLen < 2 {
		weed = &ps.Weeds[0]
	} else if isFiler {
		weed = getWeed(ps.Weeds, "filer", round)
	} else {
		weed = getWeed(ps.Weeds, "master", round)
	}
	return weed.Url + uri
}

func (ps *ProxyServer) getShadowFileUrl(uri string, isFiler bool,
	round int32) string {
	weedLen := len(ps.Shadows)
	var weed *Weed
	if weedLen < 2 {
		weed = &ps.Shadows[0]
	} else if isFiler {
		weed = getWeed(ps.Shadows, "filer", round)
	} else {
		weed = getWeed(ps.Shadows, "master", round)
	}
	if weed == nil {
		return ""
	}
	return weed.Url + uri
}

/**
 * string 返回可访问url；
 * bool 返回是否包含路径
 */
func (ps *ProxyServer) submitUrl(r *http.Request, isFiler bool,
	round int32) (string, bool, string) {
	path := r.URL.Path
	weedLen := len(ps.Weeds)
	var hasPath bool
	var weed *Weed
	if weedLen < 2 {
		weed = &ps.Weeds[0]
	} else if isFiler {
		len := len(path)
		if strings.HasSuffix(path, "/") {
			len = len - 1
		}
		path = path[6:len]
		if len-6 > 0 {
			hasPath = true
		} else {
			hasPath = false
		}
		weed = getWeed(ps.Weeds, "filer", round)
	} else {
		path = "/submit"
		hasPath = false
		weed = getWeed(ps.Weeds, "master", round)
	}
	return weed.Url + path, hasPath, path
}

func checkUrl(isFiler bool, submitRootUrl string, filename string,
	hasPath bool, w http.ResponseWriter) (url string, err error) {
	url = submitRootUrl
	if !isFiler {
		return
	}
	if strings.HasSuffix(filename, "/null") {
		err = ErrNullFilename
		return
	}
	lastIndex := strings.LastIndex(filename, "/")
	if hasPath || lastIndex > 0 {
		// 保证上传访问接口地址至少包含一个路径
		rs := []rune(filename)
		lastIndex = lastIndex + 1
		if strings.HasPrefix(filename, "/") {
			url = submitRootUrl + string(rs[0:lastIndex])
		} else {
			url = submitRootUrl + "/" + string(rs[0:lastIndex])
		}
	} else {
		// 需要使用根路径
		err = errors.New("Error uploading file(s), /module/ path is needed!")
	}
	return
}

func getWeed(weeds []Weed, weedType string, round int32) (weed *Weed) {
	for _, w := range weeds {
		if strings.EqualFold(w.Type, weedType) {
			weed = &w
			if round == 0 {
				// fmt.Println("weed ", w.Type, "round=", round, "; i=", i)
				break
			}
			round--
		}
	}
	return weed
}
