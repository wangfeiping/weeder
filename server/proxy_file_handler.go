package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wangfeiping/weeder/log"
	"github.com/wangfeiping/weeder/util"
)

var ErrBadRequest = errors.New(`Bad request!`)
var ErrNotFound = errors.New(`File not found!`)
var ErrNullFilename = errors.New(`Bad request, file's name is null!`)

var fileNameEscaper = strings.NewReplacer("\\", "\\\\", "\"", "\\\"")

type VolumeAssignRequest struct {
	Count       uint64
	Replication string
	Collection  string
	Ttl         string
	DataCenter  string
	Rack        string
	DataNode    string
}

type AssignResult struct {
	Fid       string `json:"fid,omitempty"`
	Url       string `json:"url,omitempty"`
	PublicUrl string `json:"publicUrl,omitempty"`
	Count     uint64 `json:"count,omitempty"`
	Error     string `json:"error,omitempty"`
}

type UploadResult struct {
	Name  string `json:"name,omitempty"`
	Size  uint32 `json:"size,omitempty"`
	Error string `json:"error,omitempty"`
}

type FilerMeta struct {
	Path  string `json:"path"`
	Start string `json:"start"`
	Ttl   string `json:"ttl"`
}

func (ps *ProxyServer) accessCheckAndGetFiler(w http.ResponseWriter, r *http.Request,
	logHeader *log.LogHeader) {
	if ps.isAccessible(logHeader.Caddress) {
		ps.getFileHandler(w, r, true, logHeader)
	} else {
		logHeader.Key = "response"
		logHeader.Status = "denied"
		log.Info(logHeader, `{"detail":"Does not allow access."}`)
	}
}

/**
 * 获取文件
 * 成功是返回图片；
 * 失败时返回json格式信息。
 * {"message":"Can't get the file!", "status":404}
 */
func (ps *ProxyServer) getFileHandler(w http.ResponseWriter, r *http.Request,
	isFiler bool, logHeader *log.LogHeader) {
	logHeader.ClassName = "getfile"
	req := "{\"uri\":\"" + r.RequestURI + "\", \"filer\":" +
		strconv.FormatBool(isFiler) + " }"
	log.Info(logHeader, req)
	r.Close = true

	if ps.Config.DebugDetailLog && r != nil {
		for k, v := range r.Header {
			for _, vv := range v {
				log.Debug(logHeader, " request header ", k, " ; ", vv)
			}
		}
	}

	if !strings.EqualFold(r.Method, "get") &&
		!strings.EqualFold(r.Method, "head") {
		ret := "{\"result\":[], \"message\":\"Only accept GET or HEAD requests!\", \"status\":405}"
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(ret))
		w.Write([]byte("\n"))
		logHeader.Key = "response"
		logHeader.Status = "err"
		log.Error(logHeader, ret)
		return
	}
	//	err := ps.download(w, r, 0, isFiler, logHeader)
	retry := int32(0)
	err := ps.download(w, r, retry, isFiler, logHeader)
	for err != nil && retry < ps.Config.Retry {
		retry++
		log.Debug(logHeader, err.Error(), " retry: ", retry)
		err = ps.download(w, r, retry, isFiler, logHeader)
	}
	r.Body.Close()
	//	var err error = nil
	if err == nil {
		w.WriteHeader(http.StatusOK)
		logHeader.Key = "response"
		logHeader.ClassName = "getfile"
		logHeader.Status = "ok"
		log.Info(logHeader)
		return
	}
	var p *log.ApiResult
	// 使用ab 进行压力测试，反复测试后，还是会发生reset by peer 的err，
	// 但发生比较偶然，还没有确定具体原因。
	// 经判断在压力比较小时出现的reset by peer 应该是客户端的偶发问题，
	// 服务端识别该异常进行特殊处理。
	//	operr, ok := err.(*net.OpError)
	//	if operr.Err.Error() == syscall.ECONNRESET.Error() {
	if strings.Contains(err.Error(), "reset") { // syscall.ECONNRESET.Error()) {
		w.WriteHeader(http.StatusInternalServerError)
		p = &log.ApiResult{
			Result:  make([]*log.FileMeta, 0, 0),
			Message: "Closed by client! ",
			Status:  1000,
			Detail:  err.Error(),
		}
	} else if err == ErrNotFound {
		w.WriteHeader(http.StatusNotFound)
		p = &log.ApiResult{
			Result:  make([]*log.FileMeta, 0, 0),
			Message: "File not found! " + r.RequestURI,
			Status:  http.StatusNotFound,
			Detail:  err.Error(),
		}
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		p = &log.ApiResult{
			Result:  make([]*log.FileMeta, 0, 0),
			Message: "Can't get the file! " + r.RequestURI,
			Status:  http.StatusInternalServerError,
			Detail:  err.Error(),
		}
	}
	logHeader.Key = "response"
	logHeader.Status = "err"
	if bs, e := json.Marshal(&p); e == nil {
		log.Error(logHeader, string(bs))
	} else {
		log.Error(logHeader, err.Error())
	}
}

/**
 * 文件上传，支持批量上传
 *
 * 上传成功返回数据结构:
 * { "result":[
 *         {"fid":"19,2cc8a17085","fileName":"abc.png","fileUrl":"192.168.1.182:80/19,2cc8a17085","size":43132},
 *         {"fid":"19,2d905ce4a8","fileName":"123.jpg","fileUrl":"192.168.1.182:80/19,2d905ce4a8","size":24198}
 *    ],
 *    "message":"ok", "status":200
 *  }
 *
 * 上传失败：
 * {"result":[], "message":"Only submit via POST!", "status":405}
 */
func (ps *ProxyServer) submitHandler(w http.ResponseWriter, r *http.Request) {
	logHeader := &log.LogHeader{
		TraceId:    checkGid(r),
		Caddress:   checkRealIp(r),
		UserId:     checkUniSource(r),
		Key:        "request",
		ThreadName: r.URL.Path,
		MethodName: r.Method,
	}
	ps.submit(w, r, false, logHeader)
}

func (ps *ProxyServer) deleteHandler(w http.ResponseWriter, r *http.Request) {
	logHeader := &log.LogHeader{
		TraceId:    checkGid(r),
		Caddress:   checkRealIp(r),
		UserId:     checkUniSource(r),
		Key:        "request",
		ThreadName: r.URL.Path,
		ClassName:  "delete",
		MethodName: r.Method,
	}
	if !strings.EqualFold(r.Method, "post") {
		ret := `{\"result\":[], \"message\":\"Only delete via POST!\", \"status\":405}`
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(ret))
		w.Write([]byte("\n"))
		logHeader.Key = "response"
		logHeader.Status = "err"
		log.Error(logHeader, ret)
		return
	}
	r.ParseForm()
	if r.MultipartForm == nil {
		err := r.ParseMultipartForm(util.DefaultMaxMemoryMultiUpload)
		if err != nil {
			log.Error(logHeader, err.Error())
			return
		}
	}
	paths := r.MultipartForm.Value["path"]
	if len(paths) > 0 {
		path := paths[0]
		isFid := fidChecker.MatchString(path)
		ps.deleteFile(w, path, r.RemoteAddr, !isFid, logHeader)
	}
}

/**
 * 删除公有云对应文件(根据路径名判断是否需要调用公有云删除接口),以及本地文件
 */
func (ps *ProxyServer) deleteFile(w http.ResponseWriter,
	filepath string, remoteAddr string,
	isFiler bool, logHeader *log.LogHeader) {
	logHeader.ClassName = "delete"
	logHeader.Key = "response"
	logHeader.ThreadName = filepath

	result := &log.ApiResult{
		Result:  make([]*log.FileMeta, 0, 0),
		Message: "ok",
		Status:  http.StatusOK,
		Detail:  "",
	}

	if ps.Config.UniSourceCheck && logHeader.UserId == "" {
		logHeader.Key = "response"
		logHeader.Status = "err"
		result.Message = "error"
		result.Status = http.StatusNotAcceptable
		result.Detail = "It's not allowed to delete without Uni-Source header."
		log.ErrorResponse(logHeader, result, w)
		return
	}

	//仅检查r.RemoteAddr （resthub）是否在白名单中
	if ip, ok := ps.isWritable(remoteAddr); !ok {
		logHeader.Status = "err"
		result.Status = http.StatusNotAcceptable
		result.Message = "error"
		result.Detail = fmt.Sprintf(
			"It's not allowed to upload by whitelist (%s).", ip)
		log.ErrorResponse(logHeader, result, w)
		return
	}
	err := ps.qiniuDelete(filepath[1:])
	if err != nil {
		if strings.Contains(err.Error(), "no such file") {
			log.Debug(logHeader, "qiniu delete error - ", err.Error())
		} else {
			logHeader.Status = "err"
			result.Status = http.StatusInternalServerError
			result.Message = "error"
			result.Detail = fmt.Sprint("qiniu delete error - ", err.Error())
			log.ErrorResponse(logHeader, result, w)
			return
		}
	}
	retry := int32(0)
	err = ps.weedDelete(w, filepath, isFiler, logHeader, retry)
	for err != nil && retry < ps.Config.Retry {
		retry++
		log.Debug(logHeader, err.Error(), " retry: ", retry)
		err = ps.weedDelete(w, filepath, isFiler, logHeader, retry)
	}
	if err != nil {
		logHeader.Status = "err"
		result.Status = http.StatusInternalServerError
		result.Message = "error"
		result.Detail = fmt.Sprint("seaweedfs delete error - ", err.Error())
		log.ErrorResponse(logHeader, result, w)
		return
	}
	logHeader.Status = "ok"
	log.InfoResponse(logHeader, result, w)
}

func (ps *ProxyServer) qiniuDelete(filepath string) error {
	// if ps.Config.Qiniu.AccessKey == "" || ps.Config.Qiniu.SecretKey == "" {
	// 	return nil
	// }
	// kodo.SetMac(ps.Config.Qiniu.AccessKey, ps.Config.Qiniu.SecretKey)
	// c := kodo.New(ps.Config.Qiniu.Zone, nil)

	// bucket := c.Bucket(ps.Config.Qiniu.Bucket)
	// ctx := context.Background()

	// return bucket.Delete(ctx, filepath)
	return nil
}

func (ps *ProxyServer) weedDelete(w http.ResponseWriter, uri string,
	isFiler bool, logHeader *log.LogHeader, retry int32) error {
	targetUrl := ps.getFileUrl(uri, isFiler, retry)
	if isFiler {
		return deleteRequest(logHeader, targetUrl)
	}
	client := &http.Client{CheckRedirect: deleteCheckRedirect}
	response, err := client.Get(targetUrl)
	if response != nil {
		defer response.Body.Close()
	}
	if err != nil {
		if e, ok := err.(*url.Error); ok && e.Err != nil {
			targetUrl = e.URL
			return deleteRequest(logHeader, targetUrl)
		}
	}
	return errors.New("can't find the file")
}

func deleteCheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) > 0 {
		return errors.New(req.URL.String())
	}
	return nil
}

func deleteRequest(logHeader *log.LogHeader, targetUrl string) (err error) {
	var req *http.Request
	var resp *http.Response
	req, err = http.NewRequest("DELETE", targetUrl, nil)
	if err != nil {
		return
	}
	req.Close = false //true
	resp, err = weedHttpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var respBytes []byte
	respBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	result := &log.ApiResult{
		Result:  make([]*log.FileMeta, 0, 0),
		Message: "error",
		Status:  resp.StatusCode,
		Detail:  fmt.Sprint(resp.Status, " - ", string(respBytes)),
	}
	log.DebugResponse(logHeader, result)
	return
}

/**
 * curl http://192.168.1.182:9333/dir/assign?ttl=3m
 * 	   {"count":1,"fid":"10,05d86c70e4ac3b","url":"127.0.0.1:9360","publicUrl":"localhost:9360"}
 * curl -F "file=@/apps/seaweedfs/echo.png" http://192.168.1.182:9360/10,05d86c70e4ac3b?ttl=3m
 *
 * curl --data "path=/public/test/file&fildId=10,05d86c70e4ac3b" http://192.168.1.182:8888/admin/register
 *
 * curl -F "f=@/apps/seaweedfs/car.jpg" "http://localhost:8888/public/test/?ttl=3m"
 *
 * curl -X DELETE http://localhost:8888/public/test/car.jpg
 *
 * 经测试，filer path 在文件ttl超时被清除后并不会被清除
 */
func (ps *ProxyServer) submit(w http.ResponseWriter, r *http.Request,
	isFiler bool, logHeader *log.LogHeader) {
	logHeader.ClassName = "submit"

	result := &log.ApiResult{
		Result:  make([]*log.FileMeta, 0, 0),
		Message: "ok",
		Status:  http.StatusOK,
		Detail:  "",
	}

	req := "{\"uri\":\"" + r.RequestURI + "\", \"filer\":" +
		strconv.FormatBool(isFiler) + " }"
	log.Info(logHeader, req)

	if ps.Config.DebugDetailLog && r != nil {
		for k, v := range r.Header {
			for _, vv := range v {
				log.Debug(logHeader, " request header ", k, " ; ", vv)
			}
		}
	}

	//仅检查r.RemoteAddr （resthub）是否在白名单中
	if addr, ok := ps.isWritable(r.RemoteAddr); !ok {
		w.WriteHeader(http.StatusNotAcceptable)
		logHeader.Key = "response"
		logHeader.Status = "err"
		result.Message = "error"
		result.Status = http.StatusNotAcceptable
		result.Detail = fmt.Sprintf(
			"It's not allowed to upload by whitelist (%s).", addr)
		log.ErrorResponse(logHeader, result, w)
		return
	}
	if ps.Config.UniSourceCheck && logHeader.UserId == "" {
		logHeader.Key = "response"
		logHeader.Status = "err"
		result.Message = "error"
		result.Status = http.StatusNotAcceptable
		result.Detail = "It's not allowed to upload without Uni-Source header."
		log.ErrorResponse(logHeader, result, w)
		return
	}
	if !strings.EqualFold(r.Method, "post") {
		ret := `{\"result\":[], \"message\":\"Only submit via POST!\", \"status\":405}`
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(ret))
		w.Write([]byte("\n"))
		logHeader.Key = "response"
		logHeader.Status = "err"
		log.Error(logHeader, ret)
		return
	}
	// 为了简化客户端调用，直接在上传文件接口中支持文件夹ttl 设置。
	// TODO 允许对filer 路径设置ttl，如果错误设置可能会造成删除大量重要文件，
	// 需要制定路径相关规范尽量避免错误设置。
	// 规范：
	//	+ 不能对根路径“/”、保留路径“/public/”以及一级业务路径“/{appname}/”设置ttl;
	//	+ 设置ttl 的路径可以是业务路径下的任意级路径：
	//		/appname/yyyy-MM-dd/
	//		/appname/somename/temp/
	//
	// 调用参数设置：
	//	+ 批量参数设置：请求地址后跟设置参数（http://hostPort/filer?ttl=3m）或表单参数，
	//		对请求中的所有文件和地址参数设置相应ttl 参数；
	//	+ 文件参数设置：MultipartBody 中上传文件时文件路径后跟?ttl=3m 参数：
	//		单独对每一个文件设置相应的ttl 参数；
	//	+ 独立参数设置：MultipartBody 的form表单路径参数（path）中后跟?ttl=3m 参数：
	//		单独对每一个路径设置相应的ttl 参数；
	//	+ 同一调用中优先级：独立参数设置>文件参数设置>批量参数设置；同一路径多次调用设置，最后的调用有效！
	//
	// ttl 参数格式：'m', 'h', 'd', 'w', 'M', 'y' 默认为分钟
	//	3m: 3 minutes
	//	4h: 4 hours
	//	5d: 5 days
	//	6w: 6 weeks
	//	7M: 7 months
	//	8y: 8 years
	//
	r.ParseForm()
	err := util.ParseMultipartForm(r)
	if err != nil {
		logHeader.Key = "response"
		logHeader.Status = "err"
		result.Message = "error"
		result.Status = http.StatusBadRequest
		result.Detail = err.Error()

		log.ErrorResponse(logHeader, result, w)
		return
	}
	// 检查并上传文件
	//	fmt.Printf("the pointer is (&result.Result): %p \n", &result.Result)
	//	fmt.Printf("the pointer is (result.Result): %p \n", result.Result)
	retCode, err := ps.doSubmit(r.MultipartForm.File, w, r,
		isFiler, logHeader, &result.Result)
	// 处理路径设置
	if err == nil {
		var neverUpload bool
		if len(r.MultipartForm.File) == 0 {
			neverUpload = true
		}
		// 检查并保存path ttl 参数设置
		//	for p, v := range r.MultipartForm.Value {
		//		for n, val := range v {
		//			log.Debug(gid, "MultipartForm.Value: ", p, "=", n, "=", val)
		//		}
		//	}
		paths := r.MultipartForm.Value["path"]
		var path, query, ttl string
		var vals url.Values
		for _, val := range paths {
			ttl = ""
			path = val
			if i := strings.IndexAny(val, "?"); i >= 0 {
				path, query = val[:i], val[i+1:]
				vals, err = url.ParseQuery(query)
				if err != nil {
					if neverUpload {
						ret := `{"result":[], "message":"Wrong parameters ` +
							val + `", "status":400}`
						w.WriteHeader(http.StatusBadRequest)
						w.Write([]byte(ret))
						w.Write([]byte("\n"))
						logHeader.Key = "response"
						logHeader.Status = "err"
						log.Error(logHeader, ret)
						return
					}
					// 如果有文件已经上传，忽略参数解析异常
					log.Debug(logHeader, "parse query error: ", err.Error())
				}
				ttl = vals.Get("ttl")
			}
			if ttl == "" {
				ttl = r.Form.Get("ttl")
			}
			if ttl != "" {
				if isPathCanBeSetTtl(path) {
					dbclient.SetPathMeta(path, ttl)
					log.Debug(logHeader, "filer_path_ttl: path=", path, " ttl=", ttl)
				} else {
					log.Debug(logHeader, "can't set path ttl: ", path, " ", ttl)
				}
			}
		}
		logHeader.Key = "response"
		logHeader.Status = "ok"
		result.Message = "ok"
		result.Status = retCode
		log.InfoResponse(logHeader, result, w)
	} else {
		logHeader.Key = "response"
		logHeader.Status = "err"
		result.Message = "error"
		result.Status = retCode
		result.Detail = err.Error()
		log.ErrorResponse(logHeader, result, w)
	}
}

/**
 * 向master 服务器转发文件
 */
func (ps *ProxyServer) doSubmit(files map[string][]*multipart.FileHeader,
	w http.ResponseWriter, r *http.Request, isFiler bool,
	logHeader *log.LogHeader, metas *[]*log.FileMeta) (int, error) {
	// 普通上传操作
	submitRootUrl, hasPath, fullpath := ps.submitUrl(r, isFiler, 0)
	log.Debug(logHeader,
		"files ", len(files), " ", submitRootUrl)
	var err error
	meta, file := checkMeta(files)
	if meta != nil {
		//		var fileUploaded *log.FileMeta
		//		// 元数据按照指定路径上传并获取fid
		//		fileUploaded, err = ps.doSubmitFile(file,
		//			submitRootUrl, hasPath, fullpath, w, r, isFiler, logHeader)
		//		if err == ErrNullFilename {
		//			return http.StatusBadRequest, err
		//		} else if err != nil {
		//			return http.StatusInternalServerError, err
		//		}
		//		// 元数据上传注册
		//		var bs []byte
		//		bs, err = metaUploadRequest(meta, file, fileUploaded, logHeader)
		//		if err != nil {
		//			return http.StatusInternalServerError, err
		//		}
		//		metaResp := string(bs)
		//		if ps.Config.DebugDetailLog {
		//			log.Debug(logHeader, "meta upload result: ", metaResp)
		//		}
		//		if strings.Contains(metaResp, `"error"`) {
		//			return http.StatusBadRequest, ErrBadRequest
		//		}
		//		*metas = append(*metas, fileUploaded)
		//		fileUploaded.Url = ps.Config.FileUrlPrefix + fileUploaded.Fid
		//		//		fmt.Printf("the pointer is (metas) 2: %p \n", metas)
		//		return http.StatusOK, nil
		fileMeta := &log.FileMeta{}
		var status int
		status, err = ps.registerChunkedFileMeta(meta, file, fullpath,
			fileMeta, w, r, logHeader)
		//		fileUploaded.Fid = fid
		if err != nil {
			return status, err
		}
		*metas = append(*metas, fileMeta)
		return status, nil
	}
	var fileUploaded *log.FileMeta
	for _, fhs := range files {
		if len(fhs) > 0 {
			fileUploaded, err = ps.doSubmitFile(fhs[0],
				submitRootUrl, hasPath, fullpath, w, r, isFiler, logHeader)
			if err == ErrNullFilename {
				return http.StatusBadRequest, err
			} else if err != nil {
				return http.StatusInternalServerError, err
			}
			*metas = append(*metas, fileUploaded)
			fileUploaded.Url = ps.Config.FileUrlPrefix + fileUploaded.Fid
		} else {
			return http.StatusBadRequest, ErrBadRequest
		}
	}

	return http.StatusOK, nil
}

func (ps *ProxyServer) doSubmitFile(file *multipart.FileHeader,
	submitRootUrl string, hasPath bool, fullpath string,
	w http.ResponseWriter, r *http.Request,
	isFiler bool, logHeader *log.LogHeader) (*log.FileMeta, error) {
	retry := int32(0)
	filename := file.Filename
	if ps.Config.DebugDetailLog {
		log.Debug(logHeader, " filename=", filename)
	}
	submitUrl, err := checkUrl(isFiler, submitRootUrl,
		filename, hasPath, w)
	if err != nil {
		return nil, err
	}
	var ttl string
	var fileUrl *url.URL
	if !strings.HasPrefix(filename, "/") {
		filename = "/" + filename
	}
	fileUrl, err = url.ParseRequestURI(filename)
	if err != nil {
		return nil, err
	}
	var vals url.Values
	vals = fileUrl.Query()
	if len(vals) > 0 {
		ttl = vals.Get("ttl")
	}
	if ttl == "" {
		ttl = r.Form.Get("ttl")
	}
	if !strings.EqualFold(ps.Config.DevEnvEnforcedTtl, "") {
		submitUrl = submitUrl + "?ttl=" + ps.Config.DevEnvEnforcedTtl
	} else if !strings.EqualFold(ttl, "") {
		submitUrl = submitUrl + "?ttl=" + ttl
	}
	msg, err := ps.doUpload(r, file, fileUrl.Path,
		w, submitUrl, logHeader)
	for err != nil {
		if retry > ps.Config.Retry {
			log.Error(logHeader, "submit:", "file", err)
			return nil, err
		} else {
			retry++
			log.Error(logHeader, "submit: ", "retrying ", retry, " file ", err)
			submitRootUrl, hasPath, fullpath = ps.submitUrl(r, isFiler, retry)
			submitUrl, err = checkUrl(isFiler, submitRootUrl,
				filename, hasPath, w)
			if err != nil {
				return nil, err
			}
			msg, err = ps.doUpload(r, file, fileUrl.Path,
				w, submitUrl, logHeader)
		}
	}
	// 解析格式化返回数据
	fileJson := log.FileMeta{}
	err = json.Unmarshal([]byte(msg), &fileJson)
	if err != nil {
		return nil, err
	}
	if fileJson.Error != "" {
		return nil, errors.New("Error uploading file(s)!")
	}
	if isFiler {
		filepath := fullpath + fileUrl.Path
		if ps.Config.RedisCacheTtl != "" {
			redisclient.CacheFilePath(
				filepath, fileJson.Fid, ps.Config.RedisCacheTtl)
			log.Debug(logHeader, "cache file path -> ", filepath, ", ",
				fileJson.Fid, ", ", ps.Config.RedisCacheTtl)
		}
	}
	return &fileJson, nil
}

func (ps *ProxyServer) doUpload(r *http.Request, fh *multipart.FileHeader,
	filename string, w http.ResponseWriter, submitUrl string,
	logHeader *log.LogHeader) (string, error) {
	f, err := fh.Open()
	if err != nil {
		return `{"error":"Error parsing request!"}`, err
	}
	defer func() {
		f.Close()
	}()
	if ps.Config.DebugDetailLog {
		log.Debug(logHeader, " filename=", filename, " uploading... ", submitUrl)
	}
	return ps.upload(filename, f, submitUrl, r, logHeader)
}

func (ps *ProxyServer) upload(filename string, f multipart.File,
	submitUrl string, r *http.Request, logHeader *log.LogHeader) (string, error) {
	buf := new(bytes.Buffer)
	multipartWriter := multipart.NewWriter(buf)
	// Create file field
	formWriter, err := multipartWriter.CreateFormFile(filename, filename)
	if err != nil {
		return "", err
	}
	//	_, err = io.Copy(formWriter, f)
	bufFile := make([]byte, 32*1024)
	for {
		nr, er := f.Read(bufFile)
		if nr > 0 {
			nw, ew := formWriter.Write(bufFile[0:nr])
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			err = er
			break
		}
	}
	if err != nil {
		return "", err
	}
	multipartWriter.Close()
	req, err := http.NewRequest("POST", submitUrl, buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	var client http.Client
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if ps.Config.DebugDetailLog {
		log.Debug(logHeader, "upload response status - "+resp.Status)
		for k, v := range resp.Header {
			for _, vv := range v {
				log.Debug(logHeader, "upload response header - ", k, " : ", vv)
			}
		}
	}
	/**
	 * ioutil.ReadAll(resp.Body)
	 * 会将文件整个读取到内存中后才返回,如果文件较大或并发访问量较大,需要注意
	 */
	resultJson, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if ps.Config.DebugDetailLog {
		log.Debug(logHeader, "upload response - ", string(resultJson))
	}
	return string(resultJson), nil
}

func (ps *ProxyServer) download(w http.ResponseWriter, r *http.Request,
	retry int32, isFiler bool, logHeader *log.LogHeader) error {
	url := ps.getFileUrl(r.RequestURI, isFiler, retry)
	referrer := r.Header.Get("referrer")
	log.Debug(logHeader, "isFiler: ", isFiler,
		", url: ", url, ", referrer:", referrer)
	//	defer func() {
	//		if rc := recover(); rc != nil {
	//			log.Error("download", gid, rc)
	//		}
	//	}()
	//  只有在 return err 的情况下，上面defer 才能正常输出错误，
	//  否则会输出 runtime error: invalid memory address or nil pointer dereference
	//	resp, err := http.Get(url)
	//  https: //studygolang.com/articles/9190
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Close = false //true
	//	resp, err := http.DefaultClient.Do(req)
	resp, err := weedHttpClient.Do(req)
	if err != nil {
		log.Debug(logHeader, "getfile: ", err)
		if retry < ps.Config.Retry {
			return err
		}
	} else {
		if resp.StatusCode != http.StatusNotFound {
			return ps.writeResponseContent(resp, w, r)
		}
		resp.Body.Close()
		if !ps.ShadowAccess {
			//			w.WriteHeader(http.StatusNotFound)
			return ErrNotFound
		}
	}
	shadowRetry := int32(0)
	resp, err = ps.downloadShadow(w, r, shadowRetry, isFiler, logHeader)
	for err != nil && shadowRetry < ps.Config.Retry {
		shadowRetry++
		log.Debug(logHeader, err.Error(), " shadow retry: ", shadowRetry)
		resp, err = ps.downloadShadow(w, r, shadowRetry, isFiler, logHeader)
	}
	if err != nil {
		return err
	}
	return ps.writeResponseContent(resp, w, r)
}

func (ps *ProxyServer) downloadShadow(w http.ResponseWriter, r *http.Request,
	retry int32, isFiler bool, logHeader *log.LogHeader) (*http.Response, error) {
	url := ps.getShadowFileUrl(r.RequestURI, isFiler, retry)
	if url == "" {
		return nil, ErrNotFound
	}
	log.Debug(logHeader, "download shadow... ", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Close = false //true
	//	resp, err := http.DefaultClient.Do(req)
	return weedHttpClient.Do(req)
}

/**
 * 检查path 是否是一级目录，如果不是一级目录则返回true(一级目录不能设置TTL)
 */
func isPathCanBeSetTtl(path string) bool {
	counting := true
	count := 0
	l := len(path)
	for i := 0; i < l; i++ {
		b := path[i]
		if b == '/' {
			if counting {
				count++
				counting = false
			}
		} else {
			counting = true
		}
	}
	if path[l-1] == '/' {
		count--
	}
	if count > 1 {
		return true
	}
	return false
}

func (ps *ProxyServer) registerChunkedFileMeta(meta *multipart.FileHeader,
	file *multipart.FileHeader, fullpath string, fileMeta *log.FileMeta,
	w http.ResponseWriter, r *http.Request,
	logHeader *log.LogHeader) (status int, err error) {

	status = http.StatusInternalServerError

	// 申请fid
	ar := &VolumeAssignRequest{
		Count: uint64(1),
		//		Replication: fi.Replication,
		Replication: "001",
		//		Collection:  fi.Collection,
		//		Ttl:         fi.Ttl,
	}

	weed := getWeed(ps.Weeds, "master", 0)
	var ret *AssignResult
	ret, err = Assign(weed.Url, ar, logHeader)
	if err != nil {
		log.Error(logHeader, err.Error())
		return
	}
	fileMeta.Name = filepath.Base(file.Filename)
	fileMeta.Fid = ret.Fid
	fileMeta.Url = ps.Config.FileUrlPrefix + ret.Fid
	fileMeta.PublicUrl = ret.PublicUrl
	fileMeta.Count = ret.Count
	fileMeta.Error = ret.Error
	fileChunksMetaUrl := "http://" + ret.Url + "/" + ret.Fid

	// 注册chunks manifest
	err = ps.upload_chunked_file_manifest(
		fileChunksMetaUrl, file, fileMeta.Name, logHeader)
	if err != nil {
		log.Error(logHeader, err.Error())
		return
	}

	// 映射path 与fid
	weed = getWeed(ps.Weeds, "filer", 0)
	values := make(url.Values)
	values.Add("fileId", fileMeta.Fid)
	values.Add("path", file.Filename)
	_, err = util.Post(weed.Url+"/admin/register", values)
	if err != nil {
		return status, err
	}
	if ps.Config.DebugDetailLog {
		log.Debug(logHeader, "/admin/register ", fileMeta.Fid, " -> ", file.Filename)
	}

	status = http.StatusOK
	return
}

func Assign(server string, r *VolumeAssignRequest,
	logHeader *log.LogHeader) (*AssignResult, error) {
	values := make(url.Values)
	values.Add("count", strconv.FormatUint(r.Count, 10))
	if r.Replication != "" {
		values.Add("replication", r.Replication)
	}
	if r.Collection != "" {
		values.Add("collection", r.Collection)
	}
	if r.Ttl != "" {
		values.Add("ttl", r.Ttl)
	}
	if r.DataCenter != "" {
		values.Add("dataCenter", r.DataCenter)
	}
	if r.Rack != "" {
		values.Add("rack", r.Rack)
	}
	if r.DataNode != "" {
		values.Add("dataNode", r.DataNode)
	}

	jsonBlob, err := util.Post(server+"/dir/assign", values)
	log.Debug(logHeader, "assign result :", string(jsonBlob))
	if err != nil {
		return nil, err
	}
	var ret AssignResult
	err = json.Unmarshal(jsonBlob, &ret)
	if err != nil {
		return nil, fmt.Errorf("/dir/assign result JSON unmarshal error:%v, json:%s", err, string(jsonBlob))
	}
	if ret.Count <= 0 {
		return nil, errors.New(ret.Error)
	}
	return &ret, nil
}

func (ps *ProxyServer) Upload(uploadUrl string, filename string, reader io.Reader,
	isGzipped bool, mtype string, pairMap map[string]string,
	logHeader *log.LogHeader) (*UploadResult, error) {
	return ps.upload_content(uploadUrl, func(w io.Writer) (err error) {
		_, err = io.Copy(w, reader)
		return
	}, filename, isGzipped, mtype, pairMap, logHeader)
}

func (ps *ProxyServer) upload_content(uploadUrl string, fillBufferFunction func(w io.Writer) error,
	filename string, isGzipped bool, mtype string,
	pairMap map[string]string, logHeader *log.LogHeader) (*UploadResult, error) {
	body_buf := bytes.NewBufferString("")
	body_writer := multipart.NewWriter(body_buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, fileNameEscaper.Replace(filename)))
	if mtype == "" {
		mtype = mime.TypeByExtension(strings.ToLower(filepath.Ext(filename)))
	}
	if mtype != "" {
		h.Set("Content-Type", mtype)
	}
	if ps.Config.DebugDetailLog {
		log.Debug(logHeader, "mtype: ", mtype)
		log.Debug(logHeader, "uploadUrl: ", uploadUrl)
	}
	if isGzipped {
		h.Set("Content-Encoding", "gzip")
	}
	//	if jwt != "" {
	//		h.Set("Authorization", "BEARER "+string(jwt))
	//	}

	file_writer, cp_err := body_writer.CreatePart(h)
	if cp_err != nil {
		log.Error(logHeader, "error creating form file", cp_err.Error())
		return nil, cp_err
	}
	if err := fillBufferFunction(file_writer); err != nil {
		log.Error(logHeader, "error copying data", err)
		return nil, err
	}
	content_type := body_writer.FormDataContentType()
	if err := body_writer.Close(); err != nil {
		log.Error(logHeader, "error closing body", err)
		return nil, err
	}

	req, postErr := http.NewRequest("POST", uploadUrl, body_buf)
	if postErr != nil {
		log.Error(logHeader, "failing to upload to", uploadUrl, postErr.Error())
		return nil, postErr
	}
	if ps.Config.DebugDetailLog {
		log.Debug(logHeader, "content_type: ", content_type)
	}
	req.Header.Set("Content-Type", content_type)
	for k, v := range pairMap {
		req.Header.Set(k, v)
	}
	resp, post_err := weedHttpClient.Do(req)
	if post_err != nil {
		log.Error(logHeader, "failing to upload to", uploadUrl, post_err.Error())
		return nil, post_err
	}
	defer resp.Body.Close()
	resp_body, ra_err := ioutil.ReadAll(resp.Body)
	if ra_err != nil {
		return nil, ra_err
	}
	var ret UploadResult
	unmarshal_err := json.Unmarshal(resp_body, &ret)
	if unmarshal_err != nil {
		log.Error(logHeader, "failing to read upload response", uploadUrl, string(resp_body))
		return nil, unmarshal_err
	}
	if ret.Error != "" {
		return nil, errors.New(ret.Error)
	}
	return &ret, nil
}

func (ps *ProxyServer) upload_chunked_file_manifest(fileUrl string,
	file *multipart.FileHeader, filename string, logHeader *log.LogHeader) error {
	f, err := file.Open()
	if err != nil {
		return err
	}
	defer func() {
		f.Close()
	}()

	buf := new(bytes.Buffer)
	//	multipartWriter := multipart.NewWriter(buf)
	// Create file field
	//	formWriter, err := multipartWriter.CreateFormFile(filename, filename)
	//	if err != nil {
	//		return err
	//	}
	//	readBuf := make([]byte, 32*1024)
	//	var nw int
	//	for {
	//		nr, er := f.Read(readBuf)
	//		if nr > 0 {
	//			nw, err = formWriter.Write(readBuf[0:nr])
	//			if err != nil {
	//				break
	//			}
	//			if nr != nw {
	//				err = io.ErrShortWrite
	//				break
	//			}
	//		}
	//		if er == io.EOF {
	//			break
	//		}
	//		if er != nil {
	//			err = er
	//			break
	//		}
	//	}
	//	if err != nil {
	//		return err
	//	}
	//	multipartWriter.Close()
	var written int64
	written, err = io.Copy(buf, f)

	bufReader := bytes.NewReader(buf.Bytes())
	if ps.Config.DebugDetailLog {
		log.Debug(logHeader, "Uploading chunks manifest ", filename,
			"[", written, "] to ", fileUrl, "...")
	}
	u, _ := url.Parse(fileUrl)
	q := u.Query()
	q.Set("cm", "true")
	u.RawQuery = q.Encode()
	_, err = ps.Upload(u.String(), filename, bufReader, false, "application/json", nil, logHeader)
	return err
}
