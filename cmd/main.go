package main

import (
	"balancer/server"
	"encoding/json"
	logger "balancer/logs"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type Server struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type Config struct {
	Servers []Server `json:"servers"`
}

type LoadBalancer struct {
	servers   []Server
	current   int
	mutex     sync.Mutex
	algorithm string
}

func (lb *LoadBalancer) nextServer() Server {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	server := lb.servers[lb.current]
	lb.current = (lb.current + 1) % len(lb.servers)
	return server
}

func (lb *LoadBalancer) healthCheck() {
    for {
        lb.mutex.Lock()
        var activeServers []Server
        for _, server := range lb.servers {
            url := fmt.Sprintf("http://%s:%d", server.Host, server.Port)
            _, err := http.Get(url)
            if err != nil {
                log.Printf("Server %s:%d is down\n", server.Host, server.Port)
				logger.NewLogger().Error(fmt.Sprintf("Server %s:%d is down\n", server.Host, server.Port))
            } else {
				logger.NewLogger().Info(fmt.Sprintf("Server %s:%d is up\n", server.Host, server.Port))
                activeServers = append(activeServers, server)
            }
        }
        lb.servers = activeServers
        lb.mutex.Unlock()
        time.Sleep(10 * time.Second)
    }
}


func (lb *LoadBalancer) serveHTTP(w http.ResponseWriter, r *http.Request) {
	lb.mutex.Lock()
	if len(lb.servers) == 0 {
		http.Error(w, "No servers available", http.StatusServiceUnavailable)
		lb.mutex.Unlock()
		return
	}
	lb.mutex.Unlock()

	server := lb.nextServer()
	target := fmt.Sprintf("http://%s:%d?server=%d", server.Host, server.Port, lb.current)

	startTime := time.Now()

	resp, err := http.Get(target)
	if err != nil {
		logger.NewLogger().Error(fmt.Sprintf("Error forwarding request to %s:%d\n", server.Host, server.Port))
		log.Printf("Error forwarding request to %s:%d\n", server.Host, server.Port)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	responseTime := time.Since(startTime)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.NewLogger().Error(fmt.Sprintf("Error reading response from %s:%d\n", server.Host, server.Port))
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"server":        fmt.Sprintf("%s:%d", server.Host, server.Port),
		"response_time": responseTime.Seconds(),
		"body":          string(body),        
	}

	log.Printf("Request from %s served by %s:%d in %v\n", r.RemoteAddr, server.Host, server.Port, responseTime)
	logger.NewLogger().Info(fmt.Sprintf("Request served by %s:%d in %v\n", server.Host, server.Port, responseTime))

	w.Header().Set("Content-Type", "application/json")
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Error generating JSON response", http.StatusInternalServerError)
		return
	}
	w.Write(jsonResponse)
}


func loadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		logger.NewLogger().Error(fmt.Sprintf("Error opening config file: %v\n", err))
		return nil, err
	}
	defer file.Close()

	configData, err := ioutil.ReadAll(file)
	if err != nil {
		logger.NewLogger().Error(fmt.Sprintf("Error reading config file: %v\n", err))
		return nil, err
	}

	var config Config
	err = json.Unmarshal(configData, &config)
	if err != nil {
		logger.NewLogger().Error(fmt.Sprintf("Error unmarshalling config file: %v\n", err))
		return nil, err
	}

	return &config, nil
}

func main() {
	config, err := loadConfig("config.json")
	if err != nil {
		logger.NewLogger().Error(fmt.Sprintf("Error loading config: %v\n", err))
		log.Fatalf("Failed to load config: %v\n", err)
	}

	lb := &LoadBalancer{
		servers:   config.Servers,
		algorithm: "round-robin",
	}

	go lb.healthCheck()

	go server.Servers(":8081")
	go server.Servers(":8082")
	go server.Servers(":8083")

	http.HandleFunc("/", lb.serveHTTP)
	log.Println("Load balancer started on port :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
