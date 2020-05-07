package server

import (
	"encoding/json"
	"io"
	"math/rand"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/wangfeiping/weeder/log"
	"github.com/wangfeiping/weeder/util"
	mysqlutil "github.com/wangfeiping/weeder/util/mysql"
	redisutil "github.com/wangfeiping/weeder/util/redis"
)

type Weed struct {
	Url  string
	Type string
}

type ProxyServer struct {
	Config       *util.WeederConfig
	Weeds        []Weed
	UploadWhites []*net.IPNet
	FilerWhites  []*net.IPNet
	Shadows      []Weed
	ShadowAccess bool
}

type DetailJson struct {
	Detail string `json:"detail,omitempty"`
	Uri    string `json:"uri,omitempty"`
}

var fidChecker = regexp.MustCompile(`^/[^/]*$`)

var unkonwnUriChecker *regexp.Regexp

var weedHttpClient *http.Client

var redisclient util.DbAdaptor
var dbclient util.DbAdaptor

/**
 * 初始化api处理路由（映射）
 */
func NewProxyServer(c *util.WeederConfig) *ProxyServer {
	uploadNum := len(c.UploadWhite)
	accessNum := len(c.FilerWhite)
	cph := c.MaxIdleConnsPerHost
	if cph < 1 {
		cph = 100
	}
	var tr = &http.Transport{
		MaxIdleConnsPerHost: cph,
	}
	weedHttpClient = &http.Client{Transport: tr}
	if c.UnkonwnUriChecker == "" {
		c.UnkonwnUriChecker = `\||\s|"`
	}
	unkonwnUriChecker = regexp.MustCompile(c.UnkonwnUriChecker)

	ps := &ProxyServer{
		Config:       c,
		UploadWhites: make([]*net.IPNet, uploadNum, uploadNum),
		FilerWhites:  make([]*net.IPNet, accessNum, accessNum)}
	log.DebugS("main", "config: maxIdleConnsPerHost ", cph)
	log.DebugS("main", "config: retry ", c.Retry)
	log.DebugS("main", "config: fileUrlPrefix ", c.FileUrlPrefix)
	log.DebugS("main", "config: uniSourceCheck ", c.UniSourceCheck)
	log.DebugS("main", "config: redisCacheTtl ", c.RedisCacheTtl)
	log.DebugS("main", "config: unkonwnUriChecker ", c.UnkonwnUriChecker)
	initProxyWeed(&(ps.Config.Server), &(ps.Weeds))
	initProxyWeed(&(ps.Config.Shadow), &(ps.Shadows))
	if len(ps.Shadows) > 0 {
		ps.ShadowAccess = true
	} else {
		ps.ShadowAccess = false
	}
	log.DebugS("main", "config: shadow access ", ps.ShadowAccess)
	log.DebugS("main", "config: proxy servers count ", len(ps.Weeds))
	log.DebugS("main", "config: proxy shadows count ", len(ps.Shadows))
	initUploadWhite(ps)
	initFilerWhite(ps)
	initRedisClient(ps)
	initMysqlClient(ps)
	log.DebugS("main", "config: volumeCheckDuration ", c.VolumeCheckDuration)
	log.DebugS("main", "config: volumeCheckUrl ", c.VolumeCheckUrl)
	log.DebugS("main", "config: nodeCheckBaseLine ", c.NodeCheckBaseLine)
	log.DebugS("main", "config: volumeCheckBaseLine ", c.VolumeCheckBaseLine)
	log.DebugS("main", "config: warn debugDetailLog ", c.DebugDetailLog)
	log.DebugS("main", "config: warn devEnvEnforcedTtl ", c.DevEnvEnforcedTtl)
	//按照配置顺序匹配
	http.HandleFunc("/health", ps.healthHandler)
	http.HandleFunc("/echo", ps.echoHandler)
	http.HandleFunc("/submit", ps.submitHandler)
	http.HandleFunc("/delete", ps.deleteHandler)
	http.HandleFunc("/", ps.reRouting)

	StartScheduleJob(c)
	log.DebugS("main", "serve: ", c.Ip, ":", c.Port)
	return ps
}

func initProxyWeed(servers *[]util.Server, weeds *[]Weed) {
	weedCount := len(*servers)
	*weeds = make([]Weed, weedCount, weedCount)
	for i := 0; i < len(*servers); i++ {
		s := (*servers)[i]
		(*weeds)[i] = Weed{
			Url:  "http://" + s.Host + ":" + strconv.Itoa(s.Port),
			Type: s.Type}
		log.DebugS("main", "config: weed ", s.Host, ":", s.Port, " ", s.Type)
	}
	log.DebugS("main", "config: weeds ", weedCount)
}

func initUploadWhite(ps *ProxyServer) {
	for i := 0; i < len(ps.Config.UploadWhite); i++ {
		s := ps.Config.UploadWhite[i]
		_, iprange, _ := net.ParseCIDR(s)
		ps.UploadWhites[i] = iprange
		log.DebugS("main", "config: upload white ", s)
	}
	log.DebugS("main", "config: upload whites ", len(ps.UploadWhites))
}

func initFilerWhite(ps *ProxyServer) {
	for i := 0; i < len(ps.Config.FilerWhite); i++ {
		s := ps.Config.FilerWhite[i]
		_, iprange, _ := net.ParseCIDR(s)
		ps.FilerWhites[i] = iprange
		log.DebugS("main", "config: filer white ", s)
	}
	log.DebugS("main", "config: filer whites ", len(ps.FilerWhites))
}

func initRedisClient(ps *ProxyServer) {
	if ps.Config.RedisCacheTtl == "" || ps.Config.Redis.Addr == "" {
		return
	}
	redisConf := ps.Config.Redis
	log.DebugS("main", "config: ", redisConf.Addr)
	//	log.DebugS("main", "config: ", redisConf.Password)
	log.DebugS("main", "config: ", redisConf.Database)
	var err error
	redisclient, err = redisutil.NewRedisClusterClient(redisConf.Addr,
		redisConf.Password, redisConf.Database)
	if err != nil {
		redisclient, err = redisutil.NewRedisClient(redisConf.Addr,
			redisConf.Password, redisConf.Database)
		log.DebugS("main", "redis client: ", redisConf.Addr)
	} else {
		log.DebugS("main", "redis cluster: ", redisConf.Addr)
	}
}

func initMysqlClient(ps *ProxyServer) {
	if ps.Config.Mysql.Hostname == "" {
		return
	}
	mysqlConf := ps.Config.Mysql
	log.DebugS("main", "mysql: ", mysqlConf.Hostname, ":", mysqlConf.Port)
	//	log.DebugS("main", "mysql: ", mysqlConf.Password)
	log.DebugS("main", "mysql: ", mysqlConf.Database)
	var err error
	dbclient, err = mysqlutil.NewMysqlClient(&mysqlConf)
	if err != nil {
		log.DebugS("main", "mysql client error: ", err.Error())
	} else {
		log.DebugS("main", "mysql client: ", mysqlConf.Hostname, " is ok.")
	}
}

/**
 * 根据路径规范调用指定方法。
 */
func (ps *ProxyServer) reRouting(w http.ResponseWriter, r *http.Request) {
	logHeader := &log.LogHeader{
		TraceId:  checkGid(r),
		Caddress: checkRealIp(r),
		UserId:   checkUniSource(r),
		Key:      "request",
		//		ThreadName: r.URL.Path,
		ClassName:  "rerouting",
		MethodName: r.Method,
	}

	detail := &DetailJson{
		Uri: r.URL.Path,
	}
	if unkonwnUriChecker.MatchString(r.URL.Path) {
		logHeader.ThreadName = "unknown"
		infoLog(logHeader, detail)
		logHeader.Key = "response"
		logHeader.Status = "denied"
		detail.Detail = "Does not allow access."
		infoLog(logHeader, detail)
		return
	} else {
		infoLog(logHeader, detail)
	}

	switch {
	case strings.EqualFold(r.URL.Path, "/"):
		// 访问根路径需要白名单许可
		ps.accessCheckAndGetFiler(w, r, logHeader)
	case strings.HasPrefix(r.URL.Path, "/filer") &&
		strings.EqualFold(r.Method, "post"):
		// 通过filer 指定路径与文件名上传文件
		ps.submit(w, r, true, logHeader)
	case fidChecker.MatchString(r.URL.Path):
		// fid 获取文件 - 不包含任何路径，认为是fid，文件不允许上传到根目录下，只允许GET 获取文件
		if _, exist := r.URL.Query()["filepath"]; exist &&
			strings.EqualFold(r.Method, "get") {
			ps.queryFilePathHandler(w, r, logHeader)
		} else if strings.EqualFold(r.Method, "get") {
			ps.getFileHandler(w, r, false, logHeader)
		} else if strings.EqualFold(r.Method, "delete") {
			ps.deleteFile(w, r.URL.Path, r.RemoteAddr, false, logHeader)
		}
	case strings.HasSuffix(r.URL.Path, "/"):
		// 访问路径需要白名单许可
		if strings.EqualFold(r.Method, "get") {
			ps.accessCheckAndGetFiler(w, r, logHeader)
		} else if strings.EqualFold(r.Method, "delete") {
			ps.deleteFile(w, r.URL.Path, r.RemoteAddr, true, logHeader)
		}
	case strings.HasPrefix(r.URL.Path, "/public/"):
		// 允许公开访问的路径
		if _, exist := r.URL.Query()["fid"]; exist &&
			strings.EqualFold(r.Method, "get") {
			ps.queryFileIdHandler(w, r, logHeader)
		} else if strings.EqualFold(r.Method, "get") {
			ps.getFileHandler(w, r, true, logHeader)
		} else if strings.EqualFold(r.Method, "delete") {
			ps.deleteFile(w, r.URL.Path, r.RemoteAddr, true, logHeader)
		}
	default:
		// 路径+文件：包含多个‘/’，并以非‘/’字符结尾，支持GET 获取文件，需要白名单许可
		if _, exist := r.URL.Query()["fid"]; exist &&
			strings.EqualFold(r.Method, "get") {
			ps.queryFileIdHandler(w, r, logHeader)
		} else if strings.EqualFold(r.Method, "get") {
			ps.accessCheckAndGetFiler(w, r, logHeader)
		} else if strings.EqualFold(r.Method, "delete") {
			ps.deleteFile(w, r.URL.Path, r.RemoteAddr, true, logHeader)
		}
	}
}

func checkGid(r *http.Request) string {
	//根据resthub 规范，通过resthub 的请求，可以获取API请求响应的唯一串号（Request-Id）作为唯一id
	gid := r.Header.Get("Request-Id")
	if gid == "" {
		//gid获取比较麻烦，通过生成一个随机数用以在并发时区分不同的处理请求的Goroutine
		random := rand.New(rand.NewSource(time.Now().UnixNano()))
		gid = strconv.Itoa(random.Intn(100000000))
	}
	return gid
}

func (ps *ProxyServer) writeResponseContent(resp *http.Response,
	w http.ResponseWriter, r *http.Request) (err error) {
	if resp != nil {
		for k, v := range resp.Header {
			for _, vv := range v {
				if ps.Config.DebugDetailLog {
					log.DebugS("detail", "response header - ", k, " : ", vv)
				}
				w.Header().Add(k, vv)
			}
		}
		cd := resp.Header.Get("Content-Disposition")
		cd = isXlsFile(cd)
		if cd != "" {
			// 业务开发使用的某种excel 生成sdk 生成的xls 文件实际为zip 压缩文件，
			// 访问此类文件时出现Content-Type 为application/zip 的情况。
			// 导致在ie 8 下载该文件时自动将文件名改为*.zip 下载，
			// 造成无法通过excel 软件打开的问题。
			// 因此通过检查文件扩展名.xls，
			// 并强行设置Content-Type 为application/vnd.ms-excel
			// 或者
			// *.xlsx
			// application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
			w.Header().Set("Content-Type", cd)
		}

	}
	origin := r.Header.Get("origin")
	if strings.EqualFold("", origin) {
		origin = "*"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Expose-Headers", "Request-Id")
	w.Header().Set("Access-Control-Allow-Headers", "X-Requested-With,Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,PATCH")

	// 传输代码必须放在最后？否则前面判断文件类型的代码失效！！！
	buf := make([]byte, util.GET_FILE_BUF_SIZE)
	written, err := io.CopyBuffer(w, resp.Body, buf)
	defer resp.Body.Close()
	if written == 0 {
		err = ErrNotFound
		return
	}
	return
}

// 是否可上传/可删除，对应文件中的uploadWhite 配置
func (ps *ProxyServer) isWritable(remoteAddr string) (string, bool) {
	l := len(ps.UploadWhites)
	if l < 1 {
		return "", true
	}
	i := strings.Index(remoteAddr, ":")
	if i > -1 {
		remoteAddr = remoteAddr[:i]
	}
	ipRemote := net.ParseIP(remoteAddr)
	for i := 0; i < l; i++ {
		if ps.UploadWhites[i].Contains(ipRemote) {
			return "", true
		}
	}
	return remoteAddr, false
}

// 是否可访问（可读），对应文件中的filerWhite 配置
func (ps *ProxyServer) isAccessible(remoteAddr string) bool {
	i := strings.Index(remoteAddr, ":")
	if i > -1 {
		rs := []rune(remoteAddr)
		remoteAddr = string(rs[0:i])
	}
	ipRemote := net.ParseIP(remoteAddr)
	for i := 0; i < len(ps.FilerWhites); i++ {
		if ps.FilerWhites[i].Contains(ipRemote) {
			return true
		}
	}
	return false
}

/**
 * http://wiki.qianbaoqm.com/pages/viewpage.action?pageId=14190569
 * RestHub API 规范
 *
 * 获取客户端真实ip
 */
func checkRealIp(r *http.Request) (realIp string) {
	realIp = r.Header.Get("X-Real-IP")
	if realIp == "" {
		realIp = r.RemoteAddr
	}
	return
}

/**
 * http://wiki.qianbaoqm.com/pages/viewpage.action?pageId=14190569
 * RestHub API 规范
 *
 * 获取客户端业务名称
 */
func checkUniSource(r *http.Request) string {
	return r.Header.Get("Uni-Source")
}

func isXlsFile(cd string) string {
	// return strings.Contains(cd, ".xls")
	i := strings.Index(cd, "filename")
	begin := -1
	end := -1
	cds := []rune(cd)
	l := len(cds)
	for i = i + 8; i < l; i++ {
		// log.DebugS("isXlsFile", "cds[", i, "]: ", cds[i])
		if cds[i] == '"' {
			if begin < 0 {
				begin = i
			} else {
				end = i
				break
			}
		}
	}
	if end > begin {
		filename := cd[begin+1 : end]
		filename = strings.TrimSpace(filename)
		filename = strings.ToLower(filename)
		// log.DebugS("isXlsFile", "filename: ", filename)
		if strings.HasSuffix(filename, ".xls") {
			return "application/vnd.ms-excel"
		} else if strings.HasSuffix(filename, ".xlsx") {
			return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		} else if strings.HasSuffix(filename, ".csv") {
			return "text/csv"
		}
	}
	return ""
}

func infoLog(logHeader *log.LogHeader, detail *DetailJson) {
	if bs, e := json.Marshal(&detail); e == nil {
		log.Info(logHeader, string(bs))
	} else {
		log.Error(logHeader, `{"detail":"`, detail.Detail, `", "uri":"`,
			detail.Uri, `", "error":"`, e.Error(), `"}`)
	}
}
