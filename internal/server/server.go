package server

import (
	"crypto/tls"
	"fmt"
	"github.com/kabukky/httpscerts"
	"github.com/sergeychur/go_http_proxy/internal/config"
	"github.com/sergeychur/go_http_proxy/internal/database"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

type Server struct {
	httpHandler *http.Server
	httpsHandler *http.Server
	db *database.DB
	config *config.Config
}

func NewServer(pathToConfig string)  (*Server, error) {
	newConfig, err := config.NewConfig(pathToConfig)
	if err != nil {
		return nil, err
	}
	server := new(Server)
	server.config = newConfig

	err = httpscerts.Check(server.config.CertificatePath, server.config.KeyPath)
	if err != nil {
		err = httpscerts.Generate(server.config.CertificatePath,
			server.config.KeyPath, fmt.Sprintf("%s:%s",
				server.config.Host, server.config.HttpsPort))
		if err != nil {
			return nil, fmt.Errorf("no certificates generated")
		}
	}
	server.httpHandler = new(http.Server)
	server.httpHandler.Addr = fmt.Sprintf(":%s", server.config.HttpPort)
	server.httpHandler.Handler = http.HandlerFunc(server.ManageHttpRequest)
	server.httpHandler.ReadTimeout = 5 * time.Second
	server.httpHandler.WriteTimeout = 5 * time.Second


	cer, err := tls.LoadX509KeyPair(server.config.CertificatePath, server.config.KeyPath)
	if err != nil {
		return nil, err
	}
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{cer}}

	server.httpsHandler = new(http.Server)
	server.httpsHandler.Addr = fmt.Sprintf(":%s", server.config.HttpsPort)
	server.httpsHandler.Handler = http.HandlerFunc(server.ManageHttpsRequest)
	server.httpsHandler.TLSNextProto =  make(map[string]func(*http.Server, *tls.Conn, http.Handler))
	server.httpsHandler.TLSConfig = tlsConfig
	server.httpsHandler.ReadTimeout = 5 * time.Second
	server.httpsHandler.WriteTimeout = 5 * time.Second
	return server, nil
}


func (server *Server) Run() error {

	go func() {
		log.Printf("running https on port %s\n", server.httpsHandler.Addr)
		err := server.httpsHandler.ListenAndServe()
		if err != nil {
			panic(err)
		}
	}()
	log.Printf("running http on port %s\n", server.httpHandler.Addr)
	err :=server.httpHandler.ListenAndServe()
	if err != nil {
		return err
	}
	return nil
}

func (server *Server) ManageHttpRequest(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Header)
	response, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer func() {
		_ = response.Body.Close()
	}()
	for key, value := range response.Header {
		for _, subValue := range value {
			w.Header().Add(key, subValue)
		}
	}
	_, err = io.Copy(w, response.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}
}

func (server *Server) LaunchSecureConnection(w http.ResponseWriter, r *http.Request) {
	serv_conn, err := net.DialTimeout("tcp", r.Host, 15 * time.Second)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}
	w.WriteHeader(http.StatusOK)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		log.Println("Hijacking not supported")
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	client_conn, _, err := hijacker.Hijack()
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}

	f := func(dst io.WriteCloser, src io.ReadCloser) {
		defer func() {
			_ = dst.Close()
			_ = src.Close()
		}()
		_, err := io.Copy(dst, src)
		if err != nil {
			log.Println(err)
		}
	}

	go f(serv_conn, client_conn)
	go f(client_conn, serv_conn)
}

func (server *Server) ManageHttpsRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		server.LaunchSecureConnection(w, r)
	} else {
		server.ManageHttpRequest(w, r)
	}
}