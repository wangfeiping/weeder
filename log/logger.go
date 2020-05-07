package log

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/op/go-logging"
)

type LogHeader struct {
	TraceId    string
	Caddress   string
	Key        string
	UserId     string
	ThreadName string
	ClassName  string
	MethodName string
	Status     string
}

type FileMeta struct {
	Name      string `json:"fileName,omitempty"`
	Fid       string `json:"fid,omitempty"`
	Url       string `json:"fileUrl,omitempty"`
	Size      int    `json:"size,omitempty"`
	PublicUrl string `json:"publicUrl,omitempty"`
	Count     uint64 `json:"count,omitempty"`
	Error     string `json:"error,omitempty"`
}

type ApiResult struct {
	Result  []*FileMeta `json:"result"`
	Message string      `json:"message"`
	Status  int         `json:"status"`
	Detail  string      `json:"detail,omitempty"`
}

var logger = logging.MustGetLogger("weeder")

var defaultOutput = os.Stderr

var defaultLevel = logging.DEBUG

var logHost = ""

// 日志规范
// http://wiki.qianbaoqm.com/pages/viewpage.action?pageId=14190908
// {{appname}}[时间][logLevel][sessionId][traceId][cip:cport][sip:sport][自定义key][userId][线程名|类名|方法名|执行时间] – messageBody

var defaultFormat = `[%{time:2006-01-02 15:04:05.000}][%{level:.4s}][]%{message}`

func init() {
	SetLoggingFormat(defaultFormat, defaultOutput)
}

// SetLoggingFormat sets the logging format and the location of the log output
func SetLoggingFormat(formatString string, output io.Writer) {
	if formatString == "" {
		formatString = defaultFormat
	}
	format := logging.MustStringFormatter(formatString)

	initLoggingBackend(format, output)
}

// initialize the logging backend based on the provided logging formatter
// and I/O writer
func initLoggingBackend(logFormatter logging.Formatter, output io.Writer) {
	backend := logging.NewLogBackend(output, "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, logFormatter)
	logging.SetBackend(backendFormatter).SetLevel(defaultLevel, "")
}

func InitLogHost(host string) {
	logHost = host
}

func InfoDetail(traceId string, caddress string, saddress string,
	key string, userId string, threadName string, className string,
	methodName string, status string,
	msg ...interface{}) {
	var buf bytes.Buffer
	buf.WriteString("[")
	buf.WriteString(traceId)
	buf.WriteString("][")
	buf.WriteString(caddress)
	buf.WriteString("][")
	buf.WriteString(saddress)
	buf.WriteString("][")
	buf.WriteString(key)
	if status != "" {
		buf.WriteString("-")
		buf.WriteString(status)
	}
	buf.WriteString("][")
	buf.WriteString(userId)
	buf.WriteString("][")
	buf.WriteString(threadName)
	buf.WriteString("|")
	buf.WriteString(className)
	buf.WriteString("|")
	buf.WriteString(methodName)
	buf.WriteString("|] -")
	logger.Info(buf.String(), fmt.Sprint(msg...))
}

// log request and response
func Info(logHeader *LogHeader,
	msg ...interface{}) {
	InfoDetail(logHeader.TraceId, logHeader.Caddress, logHost,
		logHeader.Key, logHeader.UserId, logHeader.ThreadName, logHeader.ClassName,
		logHeader.MethodName, logHeader.Status,
		fmt.Sprint(msg...))
}

func InfoResponse(logHeader *LogHeader, result *ApiResult, w http.ResponseWriter) {
	w.WriteHeader(result.Status)
	if bs, err := json.Marshal(&result); err != nil {
		Info(logHeader, err.Error(),
			"; ", result.Status, "; ", result.Message, "; ", result.Detail)
		w.Write([]byte("{\"result\":[], \"message\":\""))
		w.Write([]byte(result.Message))
		w.Write([]byte("\", \"status\":"))
		w.Write([]byte(fmt.Sprint(result.Status)))
		w.Write([]byte(", \"detail\":\""))
		w.Write([]byte(result.Detail))
		w.Write([]byte("\"}"))
	} else {
		Info(logHeader, string(bs))
		w.Write(bs)
	}
	w.Write([]byte("\n"))
}

func DebugDetail(traceId string, caddress string, saddress string,
	key string, userId string, threadName string, className string,
	methodName string, status string,
	msg ...interface{}) {
	var buf bytes.Buffer
	buf.WriteString("[")
	buf.WriteString(traceId)
	buf.WriteString("][")
	buf.WriteString(caddress)
	buf.WriteString("][")
	buf.WriteString(saddress)
	buf.WriteString("][")
	buf.WriteString(key)
	if status != "" {
		buf.WriteString("-")
		buf.WriteString(status)
	}
	buf.WriteString("][")
	buf.WriteString(userId)
	buf.WriteString("][")
	buf.WriteString(threadName)
	buf.WriteString("|")
	buf.WriteString(className)
	buf.WriteString("|")
	buf.WriteString(methodName)
	buf.WriteString("|] -")
	logger.Debug(buf.String(), fmt.Sprint(msg...))
}

func Debug(logHeader *LogHeader, msg ...interface{}) {
	DebugDetail(logHeader.TraceId, logHeader.Caddress, logHost,
		logHeader.Key, logHeader.UserId, logHeader.ThreadName, logHeader.ClassName,
		logHeader.MethodName, logHeader.Status,
		`{"detail":"`, fmt.Sprint(msg...), `"}`)
}

/**
 * 部署日志记录方法
 */
func DebugS(name string, msg ...interface{}) {
	DebugDetail(name, "", logHost,
		"", "", "", "", "", "",
		`{"detail":"`, fmt.Sprint(msg...), `"}`)
}

func DebugT(name string, msg ...interface{}) {
	DebugDetail(name, "", logHost,
		"", "", "", "", "", "", fmt.Sprint(msg...))
}

func DebugResponse(logHeader *LogHeader, result *ApiResult) {
	if bs, err := json.Marshal(&result); err != nil {
		DebugDetail(logHeader.TraceId, logHeader.Caddress, logHost,
			logHeader.Key, logHeader.UserId, logHeader.ThreadName, logHeader.ClassName,
			logHeader.MethodName, logHeader.Status,
			err.Error(),
			"; ", result.Status, "; ", result.Message, "; ", result.Detail)
	} else {
		DebugDetail(logHeader.TraceId, logHeader.Caddress, logHost,
			logHeader.Key, logHeader.UserId, logHeader.ThreadName, logHeader.ClassName,
			logHeader.MethodName, logHeader.Status,
			string(bs))
	}
}

func ErrorDetail(traceId string, caddress string, saddress string,
	key string, userId string, threadName string, className string,
	methodName string, status string,
	msg ...interface{}) {
	var buf bytes.Buffer
	buf.WriteString("[")
	buf.WriteString(traceId)
	buf.WriteString("][")
	buf.WriteString(caddress)
	buf.WriteString("][")
	buf.WriteString(saddress)
	buf.WriteString("][")
	buf.WriteString(key)
	if status != "" {
		buf.WriteString("-")
		buf.WriteString(status)
	}
	buf.WriteString("][")
	buf.WriteString(userId)
	buf.WriteString("][")
	buf.WriteString(threadName)
	buf.WriteString("|")
	buf.WriteString(className)
	buf.WriteString("|")
	buf.WriteString(methodName)
	buf.WriteString("|] -")
	logger.Error(buf.String(), fmt.Sprint(msg...))
}

func Error(logHeader *LogHeader, msg ...interface{}) {
	ErrorDetail(logHeader.TraceId, logHeader.Caddress, logHost,
		logHeader.Key, logHeader.UserId, logHeader.ThreadName, logHeader.ClassName,
		logHeader.MethodName, logHeader.Status,
		fmt.Sprint(msg...))
}

/**
 * 部署日志记录方法
 */
func ErrorS(name string, msg ...interface{}) {
	ErrorDetail(name, "", logHost,
		"", "", "", "", "", "",
		`{"detail":"`, fmt.Sprint(msg...), `"}`)
}

func ErrorResponse(logHeader *LogHeader, result *ApiResult, w http.ResponseWriter) {
	w.WriteHeader(result.Status)
	if bs, err := json.Marshal(&result); err != nil {
		Error(logHeader, err.Error(),
			"; ", result.Status, "; ", result.Message, "; ", result.Detail)
		w.Write([]byte("{\"result\":[], \"message\":\""))
		w.Write([]byte(result.Message))
		w.Write([]byte("\", \"status\":"))
		w.Write([]byte(fmt.Sprint(result.Status)))
		w.Write([]byte(", \"detail\":\""))
		w.Write([]byte(result.Detail))
		w.Write([]byte("\"}"))
	} else {
		Error(logHeader, string(bs))
		w.Write(bs)
	}
	w.Write([]byte("\n"))
}
