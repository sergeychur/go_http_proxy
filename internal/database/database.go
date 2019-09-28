package database

import (
	_ "github.com/lib/pq"
	"github.com/sergeychur/go_http_proxy/internal/models"
	"gopkg.in/jackc/pgx.v2"
	"time"
)

const (
	saveRequest   = "INSERT INTO requests(is_https, data) VALUES($1, $2) RETURNING req_id;"
	selectRequest = "SELECT * FROM requests WHERE req_id = $1;"
	pageRequests  = "SELECT req.* FROM requests req " +
		"JOIN ( SELECT req_id FROM requests req ORDER BY req_id " +
		"LIMIT $1 OFFSET $2) sub_q ON (req.req_id = sub_q.req_id) ORDER BY req_id;"
)

type DB struct {
	db           *pgx.ConnPool
	user         string
	password     string
	databaseName string
	host         string
	port         uint16
}

func NewDB(user string, password string, dataBaseName string,
	host string, port uint16) *DB {
	db := new(DB)
	db.user = user
	db.databaseName = dataBaseName
	db.password = password
	db.host = host
	db.port = port
	return db
}

func (db *DB) Start() error {
	conf := pgx.ConnConfig{
		Host:     db.host,
		Port:     db.port,
		User:     db.user,
		Password: db.password,
		Database: db.databaseName,
	}
	poolConf := pgx.ConnPoolConfig{
		ConnConfig:     conf,
		MaxConnections: 80,
		AcquireTimeout: time.Duration(7 * time.Second),
	}
	dataBase, err := pgx.NewConnPool(poolConf)
	if err != nil {
		return err
	}
	db.db = dataBase
	return nil
}

func (db *DB) Close() {
	db.db.Close()
}

func (db *DB) StartTransaction() (*pgx.Tx, error) {
	return db.db.Begin()
}

func (db *DB) SaveRequest(request *models.Request) (int, error) {
	row := db.db.QueryRow(saveRequest, request.IsHTTPS, request.Data)
	id := 0
	err := row.Scan(&id)
	return id, err
}

func (db *DB) GetRequest(reqId int) (*models.Request, error) {
	row := db.db.QueryRow(selectRequest, reqId)
	req := new(models.Request)
	err := row.Scan(&req)
	return req, err
}

func (db *DB) GetRequests(offset int, limit int) (models.Requests, error) {
	rows, err := db.db.Query(pageRequests, limit, offset)
	if err != nil {
		return nil, err
	}
	requests := make(models.Requests, 0)
	defer rows.Close()
	for rows.Next() {
		req := new(models.Request)
		err := rows.Scan(&req.Id, &req.IsHTTPS, &req.Data)
		if err != nil {
			return nil, err
		}
		requests = append(requests, req)
	}
	return requests, nil
}
