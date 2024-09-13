package server

import (
	logger "balancer/logs"
	"fmt"
	"log"
	"net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
    serverName := r.URL.Query().Get("server")
    response := fmt.Sprintf("Server %s response", serverName)
    fmt.Fprint(w, response)
}

func Servers(port string) {
    mux := http.NewServeMux()
    mux.HandleFunc("/", handler)
    log.Println("Starting server on " + port)
    logger.NewLogger().Info("Starting server on " + port)
    log.Fatal(http.ListenAndServe(port, mux))
}
