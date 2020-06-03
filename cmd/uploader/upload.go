package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var method = "POST"

type FileMeta struct {
	Name      string `json:"fileName,omitempty"`
	Fid       string `json:"fid,omitempty"`
	Url       string `json:"fileUrl,omitempty"`
	Size      int    `json:"size,omitempty"`
	PublicUrl string `json:"publicUrl,omitempty"`
	Count     int    `json:"count,omitempty"`
	Error     string `json:"error,omitempty"`
}

type ApiResult struct {
	Result  []*FileMeta `json:"result"`
	Message string      `json:"message"`
	Status  int         `json:"status"`
	Detail  string      `json:"detail,omitempty"`
}

func upload(home string, dir, prefix, api string) (err error) {
	fmt.Println("client_home: ", mBinPath)
	fmt.Println("dir: ", dir)
	fmt.Println("prefix: ", prefix)
	fmt.Println("api: ", api)

	err = walkDir(dir, prefix, api)

	return
}

//获取指定目录及所有子目录下的所有文件，可以匹配后缀过滤。
func walkDir(dirPth, prefix, api string) (err error) {
	err = filepath.Walk(dirPth, func(filename string, fi os.FileInfo, err error) error {
		//遍历目录
		//if err != nil { //忽略错误
		// return err
		//}

		if fi.IsDir() { // 忽略目录
			return nil
		}
		fmt.Println("file: " + filename)
		var fileUploadResult = &ApiResult{}
		err = doUploadSingleFile(api,
			filename,
			filepath.Join(prefix, filename),
			fileUploadResult)
		if err != nil {
			fmt.Println("error: ", err.Error())
		}
		return err
	})

	return err
}

func doUploadSingleFile(api, filepath, fileparam string,
	fileUploadResult *ApiResult) (err error) {
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
	req, err = http.NewRequest(method, api, buf)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	req.Header.Set("Uni-Source", "uploader/cmd")
	var client http.Client
	resp, err = client.Do(req)
	defer resp.Body.Close()
	var result []byte
	result, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("read response error: ", err.Error())
		return
	}
	fmt.Println("response body: ", string(result))
	err = json.Unmarshal(result, fileUploadResult)
	if err != nil {
		fmt.Println("upload: error: ", err.Error())
		return
	}
	if len(fileUploadResult.Result) != 1 {
		err = errors.New("result data error")
	} else {
		url := getDownloadDomain(api)
		fmt.Println("upload: ", url+fileparam)

	}
	fmt.Println(string(result))
	return
}

func getDownloadDomain(api string) string {
	if strings.Contains(api, "://dev-apis.qianbao.com") {
		return "http://test-img0.qianbao.com"
	}
	return "http://img0.qianbao.com"
}
