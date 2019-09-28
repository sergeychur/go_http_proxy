package proxy

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"github.com/sergeychur/go_http_proxy/internal/certificates"
	"github.com/sergeychur/go_http_proxy/internal/config"
	"github.com/sergeychur/go_http_proxy/internal/database"
	"github.com/sergeychur/go_http_proxy/internal/models"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"strconv"
	"time"
)

const (
	OKHeader = "HTTP/1.1 200 OK\r\n\r\n"
)

type Server struct {
	ca tls.Certificate
	httpClient *http.Client
	db           *database.DB
	config       *config.Config
}

func NewServer(pathToConfig string) (*Server, error) {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	newConfig, err := config.NewConfig(pathToConfig)
	if err != nil {
		return nil, err
	}
	server := new(Server)
	server.config = newConfig

	dbPort, err := strconv.Atoi(server.config.DBPort)
	server.db = database.NewDB(server.config.DBUser, server.config.DBPass, server.config.DBName,
		server.config.DBHost, uint16(dbPort))
	err = server.db.Start()
	if err != nil {
		return nil, err
	}
	server.ca, err = certificates.LoadCA()
	if err != nil {
		return nil, err
	}

	server.httpClient = new(http.Client)
	server.httpClient.Timeout = 5 * time.Second
	return server, nil
}

func (server *Server) Run() error {
	//log.Printf("running https on port %s\n", proxy.httpsHandler.Addr)
	err := http.ListenAndServe(":" + server.config.HttpsPort, server)
	if err != nil {
		panic(err)
	}
	return nil
}

func (server *Server) ManageHttpRequest(w http.ResponseWriter, r *http.Request) {
	//log.Println(r.Header)
	requestToSave, err := httputil.DumpRequest(r, true)
	if err == nil {
		server.saveRequest(requestToSave, false)
	}
	response, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		//log.Printf("round trip error: %s", err)
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
	host := r.Host
	leafCert, err := certificates.GenerateCert(&server.ca, host)

	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	curCert := make([]tls.Certificate, 1)
	curCert[0] = *leafCert
	curConfig := &tls.Config{
		Certificates: curCert,
		GetCertificate: func(info *tls.ClientHelloInfo) (certificate *tls.Certificate, e error) {
			return certificates.GenerateCert(&server.ca, info.ServerName)
		},
	}
	serverConn, err := tls.Dial("tcp", r.Host, curConfig)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		//log.Println("Hijacking not supported")
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	conn, _, err := hijacker.Hijack()
	if err != nil{
		//log.Println(err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}
	_, err = conn.Write([]byte(OKHeader))
	if err != nil {
		_ = conn.Close()
		return
	}

	clientConn := tls.Server(conn, curConfig)
	err = clientConn.Handshake()
	if err != nil {
		_ = clientConn.Close()
		_ = conn.Close()
		return
	}

	f := func(dst io.WriteCloser, src io.ReadCloser, isSaved bool) {
		if src != nil && dst != nil {
			defer func() {
				_ = dst.Close()
			}()
			defer func() {
				_ = src.Close()
			}()
			buf := new(bytes.Buffer)
			multiWriter := io.MultiWriter(dst, buf)
			_, err = io.Copy(multiWriter, src)
			if err != nil {
				log.Println(err)
				return
			}
			if isSaved {
				fmt.Println(string(buf.Bytes()))
				server.saveRequest(buf.Bytes(), true)
			}
		}
	}

	go f(serverConn, clientConn, true)
	go f(clientConn, serverConn, false)
}

func (server *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		server.LaunchSecureConnection(w, r)
	} else {
		server.ManageHttpRequest(w, r)
	}
}

func (server *Server) saveRequest(request []byte, isHTTPS bool) {
	//buf, err := httputil.DumpRequest(r, true)
	req := &models.Request{
		Data:    request,
		IsHTTPS: isHTTPS,
	}
	_, err := server.db.SaveRequest(req)
	if err != nil {
		log.Printf("Request wasn't saved: %s", err)
	}
}