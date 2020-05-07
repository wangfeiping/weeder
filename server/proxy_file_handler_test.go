package server

import (
	"strings"
	"testing"
)

var submitApiMeta *FileMeta
var filerApiMeta *FileMeta

func Test_isPathCanBeSetTtl(t *testing.T) {
	if !isPathCanBeSetTtl("/ppp//asd") {
		t.Error("Test_isPathCanBeSetTtl")
	}
	if !isPathCanBeSetTtl("/ppp//asd/") {
		t.Error("Test_isPathCanBeSetTtl")
	}
	if isPathCanBeSetTtl("//ppp//") {
		t.Error("Test_isPathCanBeSetTtl")
	}
	if isPathCanBeSetTtl("//ppp") {
		t.Error("Test_isPathCanBeSetTtl")
	}
}

func Test_SubmitRestHubApi(t *testing.T) {
	if submitApi == "" {
		t.Log("Has not been tested!")
		return
	}
	var err error
	var fileUploadResult = &FileUploadResult{}
	err = doUploadSingleFile("POST", submitApi,
		"../../echo.png", "echo.png", fileUploadResult, t)
	if err != nil {
		t.Error("Test_SubmitRestHubApi error", err.Error())
	}
	submitApiMeta = &fileUploadResult.Result[0]
	if !strings.EqualFold(submitApiMeta.Name, "echo.png") {
		t.Error("Test_SubmitRestHubApi error")
	}
}

func Test_FilerRestHubApi(t *testing.T) {
	if filerApi == "" {
		t.Log("Has not been tested!")
		return
	}
	var err error
	var fileUploadResult = &FileUploadResult{}
	err = doUploadSingleFile("POST", filerApi,
		"../../echo.png", "/public/weedertest/filerapi-echo.png", fileUploadResult, t)
	//	err = doUploadSingleFile("POST", filerApi,
	//		"D:/workspace/pay_upload/ALIPAY.png", "/public/payment/bank_logo/ALIPAY.png", fileUploadResult, t)
	//	err = doUploadSingleFile("POST", filerApi,
	//		"D:/workspace/pay_upload/BANKCARD.png", "/public/payment/bank_logo/BANKCARD.png", fileUploadResult, t)
	//	err = doUploadSingleFile("POST", filerApi,
	//		"D:/workspace/pay_upload/BHH.png", "/public/payment/bank_logo/BHH.png", fileUploadResult, t)
	//	err = doUploadSingleFile("POST", filerApi,
	//		"D:/workspace/pay_upload/WEIXIN.png", "/public/payment/bank_logo/WEIXIN.png", fileUploadResult, t)

	if err != nil {
		t.Error("Test_FilerRestHubApi error", err.Error())
	}
	filerApiMeta = &fileUploadResult.Result[0]
	if !strings.EqualFold(filerApiMeta.Name, "filerapi-echo.png") {
		t.Error("Test_FilerRestHubApi error")
	}
}
