package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/wangfeiping/weeder/log"

	_ "github.com/go-sql-driver/mysql"
)

const (
	sqlUrl                     = "%s:%s@tcp(%s:%d)/%s?charset=utf8"
	default_maxIdleConnections = 20
	default_maxOpenConnections = 20
	default_maxTableNums       = 1024
	tableName                  = "filer_mapping"
)

var (
	_connect_db    sync.Once
	_db_connection *sql.DB
)

type MysqlConfig struct {
	Hostname           string `json:"hostname"`
	Port               int    `json:"port"`
	Database           string `json:"database"`
	User               string `json:"user"`
	Password           string `json:"password"`
	MaxIdleConnections int    `json:"maxIdleConnections"`
	MaxOpenConnections int    `json:"maxOpenConnections"`
}

type MysqlClient struct {
	Client *sql.DB
}

func getDbConnection(conf *MysqlConfig) *sql.DB {
	_connect_db.Do(func() {
		sqlUrl := fmt.Sprintf(sqlUrl, conf.User, conf.Password, conf.Hostname, conf.Port, conf.Database)
		var dbErr error
		_db_connection, dbErr = sql.Open("mysql", sqlUrl)
		if dbErr != nil {
			_db_connection.Close()
			_db_connection = nil
			panic(dbErr)
		}
		var maxIdleConnections, maxOpenConnections int

		if conf.MaxIdleConnections != 0 {
			maxIdleConnections = conf.MaxIdleConnections
		} else {
			maxIdleConnections = default_maxIdleConnections
		}
		if conf.MaxOpenConnections != 0 {
			maxOpenConnections = conf.MaxOpenConnections
		} else {
			maxOpenConnections = default_maxOpenConnections
		}

		_db_connection.SetMaxIdleConns(maxIdleConnections)
		_db_connection.SetMaxOpenConns(maxOpenConnections)
	})
	return _db_connection
}

func NewMysqlClient(mysqlConf *MysqlConfig) (client *MysqlClient, err error) {
	client = &MysqlClient{
		Client: getDbConnection(mysqlConf),
	}
	return
}

func (m *MysqlClient) Close() {
	m.Client.Close()
}

func (m *MysqlClient) GetFileId(filepath string) (fid string, err error) {
	sqlStatement := "SELECT fid FROM filer_mapping WHERE uriPath=?"
	fid, err = m.Query(sqlStatement, filepath)
	//	if err == sql.ErrNoRows {
	if err != nil {
		err = errors.New("fid not found")
		log.ErrorS("mysql", "get file id error: ", filepath, " - ", err.Error())
	}
	return
}

func (m *MysqlClient) GetFileFullPath(fid string) (filepath string, err error) {
	sqlStatement := "SELECT uriPath FROM filer_mapping WHERE fid=?"
	filepath, err = m.Query(sqlStatement, fid)
	//	if err == sql.ErrNoRows {
	if err != nil {
		err = errors.New("filepath not found")
		log.ErrorS("mysql", "get file path error: ", fid, " - ", err.Error())
	}
	return
}

func (m *MysqlClient) SetPathMeta(path string, ttl string) (err error) {
	// 新插入数据的频率应该比修改数据的频率高很多
	sqlStatement := "INSERT INTO filer_path_ttl (folderPath,ttl,createTime) VALUES(?,?,?)"
	var rows int64
	rows, err = m.Insert(sqlStatement, path, ttl, time.Now().Unix())
	if err == nil {
		return
	}
	log.ErrorS("mysql", "set path meta insert error: ", err.Error(),
		" affected rows: ", rows)
	sqlStatement = "UPDATE filer_path_ttl SET ttl=?, updateTime=? WHERE folderPath=?"
	rows, err = m.Update(sqlStatement, ttl, time.Now().Unix(), path)
	if err != nil {
		log.ErrorS("mysql", "set path meta update error: ", err.Error(),
			" affected rows: ", rows)
	}
	return
}

func (m *MysqlClient) CacheFilePath(
	filepath string, fid string, ttl string) (err error) {
	// 该功能已在seaweedfs 主服务程序中的mysql 客户端实现，使用mysql 时，不再需要实现本方法。
	return
}

func (m *MysqlClient) Query(sqlStatement string, value string) (string, error) {
	row := m.Client.QueryRow(sqlStatement, value)
	var result string
	err := row.Scan(&result)
	if err != nil {
		return "", err
	}
	return result, nil
}

func (m *MysqlClient) QueryValues(sqlStatement string,
	value string) (*sql.Rows, error) {
	return m.Client.Query(sqlStatement, value)
}

func (m *MysqlClient) Update(sqlStatement string, args ...interface{}) (int64, error) {
	res, err := m.Client.Exec(sqlStatement, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (m *MysqlClient) Insert(sqlStatement string, args ...interface{}) (int64, error) {
	res, err := m.Client.Exec(sqlStatement, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
