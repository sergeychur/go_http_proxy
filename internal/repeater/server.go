package repeater

import (
	"github.com/go-chi/chi"
	"github.com/sergeychur/go_http_proxy/internal/config"
	"github.com/sergeychur/go_http_proxy/internal/database"
	"log"
	"net/http"
	"strconv"
)

type Server struct {
	router *chi.Mux
	db     *database.DB
	config *config.Config
}

func NewServer(pathToConfig string) (*Server, error) {
	newConfig, err := config.NewConfig(pathToConfig)
	if err != nil {
		return nil, err
	}
	server := new(Server)
	server.config = newConfig
	router := chi.NewRouter()
	subRouter := chi.NewRouter()
	subRouter.Get("/history", server.GetHistory)
	subRouter.Get("/request/{id:^[0-9]+$}", server.GetRequest)
	subRouter.Put("/request/{id:^[0-9]+$}", server.RepeatRequest)
	subRouter.Post("/request/", server.PerformRequest)
	router.Mount("/api/", subRouter)
	server.router = router
	dbPort, err := strconv.Atoi(server.config.DBPort)
	if err != nil {
		return nil, err
	}
	db := database.NewDB(server.config.DBUser, server.config.DBPass,
		server.config.DBName, server.config.DBHost, uint16(dbPort))
	server.db = db
	return server, nil
}

func (server *Server) Run() error {
	log.Printf("running https on port %s\n", server.config.HttpsPort)
	err := server.db.Start()
	if err != nil {
		log.Printf("Failed to connect to DB: %v", err)
		return err
	}
	defer server.db.Close()
	log.Fatal(http.ListenAndServe(":"+server.config.HttpsPort, server.router))
	return nil
}
