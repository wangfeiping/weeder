package server

import (
	"bytes"
	"encoding/json"

	//	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wangfeiping/weeder/log"
	"github.com/wangfeiping/weeder/util"
)

type UploadMeta struct {
	Fid    string `json:"fid,omitempty"`
	Url    string `json:"url,omitempty"`
	Action string `json:"action,omitempty"`
}

func (ps *ProxyServer) split(w http.ResponseWriter, r *http.Request,
	logHeader *log.LogHeader) {
	r.ParseForm()
	if _, exist := r.URL.Query()["upload"]; exist {
		ps.splitUpload(w, r, logHeader)
	} else {
		ps.splitAssign(w, r, logHeader)
	}
}

func (ps *ProxyServer) splitUpload(w http.ResponseWriter, r *http.Request,
	logHeader *log.LogHeader) {
	logHeader.ClassName = "split_upload"
	if ps.Config.DebugDetailLog {
		log.Debug(logHeader, "split upload...")
	}
	result := &log.ApiResult{
		Result:  make([]*log.FileMeta, 1, 1),
		Message: "ok",
		Status:  http.StatusOK,
		Detail:  "",
	}
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
	//	log.Debug(logHeader, fmt.Sprintf("the pointer is : %p \n", r.MultipartForm.File))
	meta, file := checkMeta(r.MultipartForm.File)
	//	log.Debug(logHeader, fmt.Sprintf("the pointer is (meta): %p \n", meta))
	_, err = metaUploadRequest(meta, file, nil, logHeader)

	result.Result = make([]*log.FileMeta, 0, 0)
	if err == nil {
		logHeader.Status = "ok"
		log.InfoResponse(logHeader, result, w)
		return
	}
	result.Message = "error"
	result.Status = http.StatusInternalServerError
	logHeader.Status = "err"
	log.Error(logHeader, err.Error())
	log.ErrorResponse(logHeader, result, w)
}

func metaUploadRequest(metaFile *multipart.FileHeader, file *multipart.FileHeader,
	fileUploaded *log.FileMeta, logHeader *log.LogHeader) (resp []byte, err error) {
	var bs []byte
	bs, err = readBytes(metaFile)
	if err != nil {
		return
	}
	log.Debug(logHeader, string(bs))
	meta := UploadMeta{}
	err = json.Unmarshal(bs, &meta)
	if err != nil {
		return
	}
	bs, err = readBytes(file)
	if err != nil {
		return
	}
	log.Debug(logHeader, string(bs))
	//	meta.Url = "http://172.28.40.95:9360/"
	//	meta.Fid = "dddd"
	meta.Fid = fileUploaded.Fid
	meta.Url = fileUploaded.Url
	if strings.EqualFold("registerChunkMeta", meta.Action) {
		var u *url.URL
		u, err = url.Parse(meta.Url) // + meta.Fid)
		//		fmt.Println("POST chunks meta", u.String())
		q := u.Query()
		q.Set("cm", "true")
		q.Set("ts", strconv.Itoa(int(time.Now().Unix())))
		u.RawQuery = q.Encode()
		registerMetaUrl := u.String()
		log.Debug(logHeader, "register chunks meta url: ", registerMetaUrl)
		resp, err = util.Upload(registerMetaUrl, "application/json", bs)
	}
	return
}

func readBytes(fileHeader *multipart.FileHeader) ([]byte, error) {
	//	fmt.Printf("the pointer is (readBytes): %p \n", fileHeader)
	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	readBytes := new(bytes.Buffer)
	buf := make([]byte, 1024)
	for {
		nr, er := file.Read(buf)
		if nr > 0 {
			nw, ew := readBytes.Write(buf[0:nr])
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
		return nil, err
	}
	return readBytes.Bytes(), nil
}

func checkMeta(files map[string][]*multipart.FileHeader) (
	meta *multipart.FileHeader, file *multipart.FileHeader) {
	//	fmt.Printf("the pointer is (checkMetaPart): %p \n", files)
	for key, fhs := range files {
		if len(fhs) > 0 {
			if strings.EqualFold("meta", key) {
				meta = fhs[0]
			} else if file == nil {
				file = fhs[0]
			}
		}
	}
	return
}

func (ps *ProxyServer) splitAssign(w http.ResponseWriter, r *http.Request,
	logHeader *log.LogHeader) {
	logHeader.ClassName = "split_assign"
	if ps.Config.DebugDetailLog {
		log.Debug(logHeader, "split assign...")
	}
	result := &log.ApiResult{
		Result:  make([]*log.FileMeta, 1, 1),
		Message: "ok",
		Status:  http.StatusOK,
		Detail:  "",
	}
	values := make(url.Values)
	if _, exist := r.URL.Query()["ttl"]; exist {
		values.Add("ttl", r.Form.Get("ttl"))
	}
	assignUrl := ps.getFileUrl("/dir/assign", false, 0)
	logHeader.Key = "response"
	fileJson, err := assignRequest(assignUrl, &values)
	if err == nil {
		result.Result[0] = fileJson
		logHeader.Status = "ok"
		log.InfoResponse(logHeader, result, w)
		return
	}
	result.Message = "error"
	result.Status = http.StatusInternalServerError
	logHeader.Status = "err"
	log.Error(logHeader, err.Error())
	log.ErrorResponse(logHeader, result, w)
}

func assignRequest(url string, vals *url.Values) (*log.FileMeta, error) {
	bytes, err := util.Post(url, *vals)
	if err != nil {
		return nil, err
	}
	fileJson := log.FileMeta{}
	err = json.Unmarshal([]byte(bytes), &fileJson)
	return &fileJson, err
}
