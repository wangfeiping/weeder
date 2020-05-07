package util

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
)

var (
	client    *http.Client
	Transport *http.Transport
)

func init() {
	Transport = &http.Transport{
		MaxIdleConnsPerHost: 100,
	}
	client = &http.Client{Transport: Transport}
}

func Post(url string, values url.Values) ([]byte, error) {
	r, err := client.PostForm(url, values)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode >= 400 {
		return nil, fmt.Errorf("%s: %s", url, r.Status)
	}
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func ParseMultipartForm(r *http.Request) (err error) {
	if r.MultipartForm == nil {
		err = r.ParseMultipartForm(DefaultMaxMemoryMultiUpload)
	}
	return err
}

func Upload(url string, mtype string, fileBytes []byte) ([]byte, error) {
	body_buf := bytes.NewBufferString("")
	body_writer := multipart.NewWriter(body_buf)
	h := make(textproto.MIMEHeader)
	//	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, fileNameEscaper.Replace(filename)))
	//	if mtype == "" {
	//		mtype = mime.TypeByExtension(strings.ToLower(filepath.Ext(filename)))
	//	}
	if mtype != "" {
		h.Set("Content-Type", mtype)
	}
	file_writer, err := body_writer.CreatePart(h)
	if err != nil {
		return nil, err
	}
	if _, err = file_writer.Write(fileBytes); err != nil {
		return nil, err
	}
	content_type := body_writer.FormDataContentType()
	if err = body_writer.Close(); err != nil {
		return nil, err
	}
	var req *http.Request
	req, err = http.NewRequest("POST", url, body_buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", content_type)
	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}
