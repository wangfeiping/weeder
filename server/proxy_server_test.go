package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	redisutil "github.com/wangfeiping/weeder/util/redis"
)

var submitApi string = // "https://dev-apis.qianbao.com/basicservice/v1/intranet/weed"
"https://apis.qianbao.com/basicservice/v1/intranet/weed"
var filerApi string = // "http://dev-apis.qianbao.com/basicservice/v1/intranet/filer"
"https://apis.qianbao.com/basicservice/v1/intranet/filer"
var getFileAddr string = "https://img3.qianbao.com"
var submitFileMeta *FileMeta
var filerFileMeta *FileMeta

// redisAddr 为空字符串的话，query 和redisClient相关接口不再测试
var redisAddr string = "" //"172.28.32.130:7379,shadow://172.28.32.130:6379"
var redisPass string = ""
var redisDb int = 1
var submitTtlFileMeta *FileMeta

func Test_SubmitFile(t *testing.T) {
	if submitApi == "" {
		t.Log("Has not been tested!")
		return
	}
	var err error
	var fileUploadResult = &FileUploadResult{}
	submitUrl := submitApi
	err = doUploadSingleFile("POST", submitUrl,
		"../../echo.png", "echo.png", fileUploadResult, t)
	if err != nil {
		t.Error("Test_SubmitFile error", err.Error())
	}
	submitFileMeta = &fileUploadResult.Result[0]
	if !strings.EqualFold(submitFileMeta.Name, "echo.png") {
		t.Error("Test_SubmitFile error")
	}
}

func Test_UploadFilerFile(t *testing.T) {
	if filerApi == "" {
		t.Log("Has not been tested!")
		return
	}
	var err error
	var fileUploadResult = &FileUploadResult{}
	submitUrl := filerApi
	err = doUploadSingleFile("POST", submitUrl,
		"../../echo.png", "/public/echo/echo.png", fileUploadResult, t)
	if err != nil {
		t.Error("Test_UploadFilerFile error", err.Error())
	}
	filerFileMeta = &fileUploadResult.Result[0]
	if !strings.EqualFold(filerFileMeta.Name, "echo.png") {
		t.Error("Test_UploadFilerFile error")
	}
}

func Test_GetNeverExistFile(t *testing.T) {
	var err error
	var req *http.Request
	var resp *http.Response
	submitUrl := getFileAddr + "/public/neverExist.png"
	req, err = http.NewRequest("GET", submitUrl, nil)

	if err != nil {
		t.Error(err.Error())
	}
	var client http.Client
	resp, err = client.Do(req)
	if err != nil {
		t.Error(err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Error("Test_GetNeverExistFile error")
	}
}

func Test_QueryNeverExistFileId(t *testing.T) {
	var err error
	var req *http.Request
	var resp *http.Response
	submitUrl := getFileAddr + "/public/neverExist.png?fid"
	req, err = http.NewRequest("GET", submitUrl, nil)

	if err != nil {
		t.Error("Test_QueryNeverExistFileId error: ", err.Error())
	}
	var client http.Client
	resp, err = client.Do(req)
	if err != nil {
		t.Error("Test_QueryNeverExistFileId error: ", err.Error())
	}
	defer resp.Body.Close()
	var resultJson []byte
	resultJson, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error("Test_QueryNeverExistFileId error: ", err.Error())
	}
	retJson := FileQueryResult{}
	err = json.Unmarshal(resultJson, &retJson)
	if err != nil ||
		retJson.Status != http.StatusNotFound ||
		!strings.EqualFold(retJson.Id, "") ||
		!strings.EqualFold(retJson.Path, "/public/neverExist.png") ||
		!strings.EqualFold(retJson.Error, "fid not found") {
		t.Error("Test_QueryNeverExistFileId error")
	}
}

func Test_QueryNeverExistFilePath(t *testing.T) {
	var err error
	var req *http.Request
	var resp *http.Response
	submitUrl := getFileAddr + "/never,existfile?filepath"
	req, err = http.NewRequest("GET", submitUrl, nil)

	if err != nil {
		t.Error("Test_QueryNeverExistFilePath error: ", err.Error())
	}
	var client http.Client
	resp, err = client.Do(req)
	if err != nil {
		t.Error("Test_QueryNeverExistFilePath error: ", err.Error())
	}
	defer resp.Body.Close()
	var resultJson []byte
	resultJson, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error("Test_QueryNeverExistFilePath error: ", err.Error())
	}
	retJson := FileQueryResult{}
	err = json.Unmarshal(resultJson, &retJson)
	if err != nil ||
		retJson.Status != http.StatusNotFound ||
		!strings.EqualFold(retJson.Id, "never,existfile") ||
		!strings.EqualFold(retJson.Path, "") ||
		!strings.EqualFold(retJson.Error, "filepath not found") {
		t.Error("Test_QueryNeverExistFilePath error")
	}
}

func Test_QueryFileIdByFilepath(t *testing.T) {
	var err error
	var req *http.Request
	var resp *http.Response
	submitUrl := getFileAddr + "/public/echo/echo.png?fid"
	req, err = http.NewRequest("GET", submitUrl, nil)

	if err != nil {
		t.Error("Test_QueryFileIdByFilepath error: ", err.Error())
	}
	var client http.Client
	resp, err = client.Do(req)
	if err != nil {
		t.Error("Test_QueryFileIdByFilepath error: ", err.Error())
	}
	defer resp.Body.Close()
	var resultJson []byte
	resultJson, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error("Test_QueryFileIdByFilepath error: ", err.Error())
	}
	retJson := FileQueryResult{}
	err = json.Unmarshal(resultJson, &retJson)
	if err != nil ||
		retJson.Status != http.StatusOK ||
		!strings.EqualFold(retJson.Id, filerFileMeta.Fid) ||
		!strings.EqualFold(retJson.Path, "/public/echo/echo.png") {
		t.Error("Test_QueryFileIdByFilepath error")
	}
}

func Test_QueryFilepathByFileId(t *testing.T) {
	var err error
	var req *http.Request
	var resp *http.Response
	submitUrl := getFileAddr + "/" + filerFileMeta.Fid + "?filepath"
	//	meta := FileMeta{Fid: "4,c60fad95f6"}
	//	submitUrl := weederAddr + "/" + meta.Fid + "?filepath"
	req, err = http.NewRequest("GET", submitUrl, nil)

	if err != nil {
		t.Error("Test_QueryFilepathByFileId error: ", err.Error())
	}
	var client http.Client
	resp, err = client.Do(req)
	if err != nil {
		t.Error("Test_QueryFilepathByFileId error: ", err.Error())
	}
	defer resp.Body.Close()
	var resultJson []byte
	resultJson, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error("Test_QueryFilepathByFileId error: ", err.Error())
	}
	retJson := FileQueryResult{}
	err = json.Unmarshal(resultJson, &retJson)
	if err != nil ||
		resp.StatusCode != http.StatusOK ||
		retJson.Status != http.StatusOK ||
		!strings.EqualFold(retJson.Id, filerFileMeta.Fid) ||
		!strings.EqualFold(retJson.Path, "/public/echo/echo.png") {
		t.Error("Test_QueryFilepathByFileId error")
	}
}

func Test_GetFileByFid(t *testing.T) {
	var err error
	var req *http.Request
	var resp *http.Response
	submitUrl := getFileAddr + "/" + submitFileMeta.Fid
	req, err = http.NewRequest("GET", submitUrl, nil)

	if err != nil {
		t.Error("Test_GetFileByFid error: ", err.Error())
	}
	var client http.Client
	resp, err = client.Do(req)
	if err != nil {
		t.Error("Test_GetFileByFid error: ", err.Error())
	}
	defer resp.Body.Close()
	var res []byte
	res, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error("Test_GetFileByFid error: ", err.Error())
	}
	t.Log("size", len(res))
	if err != nil ||
		resp.StatusCode != http.StatusOK ||
		submitFileMeta.Size != len(res) {
		t.Error("Test_GetFileByFid error")
	}
}

func Test_GetFileByFilepath(t *testing.T) {
	var err error
	var req *http.Request
	var resp *http.Response
	submitUrl := getFileAddr + "/public/echo/echo.png"
	req, err = http.NewRequest("GET", submitUrl, nil)
	if err != nil {
		t.Error("Test_GetFileByFilepath error: ", err.Error())
	}
	var client http.Client
	resp, err = client.Do(req)
	if err != nil {
		t.Error("Test_GetFileByFilepath error: ", err.Error())
	}
	defer resp.Body.Close()
	//	var resultJson []byte
	var res []byte
	res, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error("Test_GetFileByFilepath error: ", err.Error())
	}
	t.Log("size", len(res))
	if err != nil ||
		resp.StatusCode != http.StatusOK ||
		filerFileMeta.Size != len(res) {
		t.Error("Test_GetFileByFilepath error")
	}
}

func Test_RedisClientQueryFilepathByFileId(t *testing.T) {
	if redisAddr == "" {
		t.Log("Has not been tested!")
		return
	}
	redis, err := redisutil.NewRedisClient(redisAddr, redisPass, redisDb)
	if err != nil {
		t.Error("Test_RedisClientQueryFilepathByFileId error", err.Error())
	}
	var filepath string
	filepath, err = redis.GetFileFullPath(filerFileMeta.Fid)
	if err != nil {
		t.Error("Test_RedisClientQueryFilepathByFileId error", err.Error())
	}
	t.Log(filepath)
	if !strings.EqualFold(filepath, "/public/echo/echo.png") {
		t.Error("Test_RedisClientQueryFilepathByFileId error")
	}
}

func Test_SubmitFileWithTtl3m(t *testing.T) {
	if submitApi == "" {
		t.Log("Has not been tested!")
		return
	}
	var err error
	var resp *http.Response
	submitUrl := submitApi + "?ttl=3m"
	fileUploadResult := &FileUploadResult{}
	err = doUploadSingleFile("POST", submitUrl,
		"../../echo.png", "echo.png", fileUploadResult, t)
	if err != nil {
		t.Error("Test_SubmitFileWithTtl3m error", err.Error())
	}
	fileMeta := fileUploadResult.Result[0]
	if !strings.EqualFold(fileMeta.Name, "echo.png") {
		t.Error("Test_SubmitFileWithTtl3m error", err.Error())
	}

	time.Sleep(200e9)

	submitUrl = getFileAddr + "/" + fileMeta.Fid
	resp, err = doRequest("GET", submitUrl, t)
	defer resp.Body.Close()
	if err != nil ||
		resp.StatusCode != http.StatusNotFound {
		t.Error("Test_SubmitFileWithTtl3m error")
	}
}

func Test_UploadFilerFileWithTtl3m(t *testing.T) {
	if submitApi == "" {
		t.Log("Has not been tested!")
		return
	}
	var err error
	var resp *http.Response
	submitUrl := filerApi + "?ttl=3m"
	fileUploadResult := &FileUploadResult{}
	err = doUploadSingleFile("POST", submitUrl,
		"../../echo.png", "/public/ttl/echo.png", fileUploadResult, t)
	if err != nil {
		t.Error("Test_UploadFilerFileWithTtl3m error", err.Error())
	}
	fileMeta := fileUploadResult.Result[0]
	if !strings.EqualFold(fileMeta.Name, "echo.png") {
		t.Error("Test_UploadFilerFileWithTtl3m error", err.Error())
	}

	time.Sleep(200e9)

	submitUrl = getFileAddr + "/public/ttl/echo.png"
	resp, err = doRequest("GET", submitUrl, t)
	defer resp.Body.Close()
	if err != nil ||
		resp.StatusCode != http.StatusNotFound {
		t.Error("Test_UploadFilerFileWithTtl3m error")
	}
}

func doUploadSingleFile(method string, url string,
	filepath string, fileparam string, fileUploadResult *FileUploadResult,
	t *testing.T) (err error) {
	var req *http.Request
	var resp *http.Response
	var file *os.File
	var formWriter io.Writer
	file, err = os.Open(filepath)
	buf := new(bytes.Buffer)
	multipartWriter := multipart.NewWriter(buf)
	// Create file field
	formWriter, err = multipartWriter.CreateFormFile("file", fileparam)
	if err != nil {
		return
	}
	_, err = io.Copy(formWriter, file)
	if err != nil {
		return
	}
	defer file.Close()
	multipartWriter.Close()
	req, err = http.NewRequest(method, url, buf)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	req.Header.Set("Uni-Source", "go_test")
	var client http.Client
	resp, err = client.Do(req)
	defer resp.Body.Close()
	var result []byte
	result, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	err = json.Unmarshal(result, fileUploadResult)
	if err != nil {
		return
	}
	t.Log(resp.StatusCode, fileUploadResult)
	if len(fileUploadResult.Result) != 1 {
		err = errors.New("result data error")
	}
	return
}

func doRequest(method string, url string, t *testing.T) (resp *http.Response, err error) {
	var req *http.Request
	req, err = http.NewRequest(method, url, nil)
	if err != nil {
		return
	}
	req.Header.Set("Uni-Source", "go_test")
	var client http.Client
	resp, err = client.Do(req)
	if t != nil {
		var result []byte
		result, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return
		}
		t.Log(resp.StatusCode, string(result))
	}
	return
}
