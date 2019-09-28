package models

type Request struct {
	Data    []byte `json:"data"`
	Id      int    `json:"id"`
	IsHTTPS bool   `json:"is_https"`
}

type Requests []*Request
