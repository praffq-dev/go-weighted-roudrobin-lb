package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

type Server struct {
	URL           *url.URL
	Alive         bool
	ReverseProxy  *httputil.ReverseProxy
	Weight        int
	CurrentWeight int
	mux           sync.RWMutex
}

func (s *Server) SetAlive(alive bool) {
	s.mux.Lock()
	s.Alive = alive
	s.mux.Unlock()
}

func (s *Server) IsAlive() bool {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.Alive
}

type LoadBalancer struct {
	Servers []*Server
	mu      sync.RWMutex
}

func (lb *LoadBalancer) GetNextServer() *Server {
	var bestServer *Server

	totalWeight := 0

	for _, server := range lb.Servers {

		if server.IsAlive() {
			totalWeight += server.Weight
		}
	}

	if totalWeight == 0 {
		return nil
	}

	for _, server := range lb.Servers {
		if !server.IsAlive() {
			continue
		}
		server.CurrentWeight += server.Weight
		if bestServer == nil || server.CurrentWeight > bestServer.CurrentWeight {
			bestServer = server
		}
	}
	bestServer.CurrentWeight -= totalWeight
	return bestServer
}

func IsServerAlive(u *url.URL) bool {
	client := http.Client{
		Timeout: 2 * time.Second,
	}

	healthURL := u.ResolveReference(&url.URL{Path: "/health"})

	resp, err := client.Get(healthURL.String())
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (lb *LoadBalancer) HealthCheck() {
	for _, server := range lb.Servers {
		if IsServerAlive(server.URL) {
			server.SetAlive(true)
			log.Printf("Server %s is alive", server.URL)
		} else {
			server.SetAlive(false)
			log.Printf("Server %s is not alive", server.URL)
		}
	}
}

func (lb *LoadBalancer) PeriodicHealthCheck(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		lb.HealthCheck()
	}
}

func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodPost && r.URL.Path == "/update-servers" {
		lb.handleServerUpdate(w, r)
		return
	}

	server := lb.GetNextServer()
	if server == nil {
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}

	server.ReverseProxy.ServeHTTP(w, r)
}

func (lb *LoadBalancer) UpdateServers(servers []*Server) {
	lb.mu.Lock()
	lb.Servers = servers
	lb.mu.Unlock()
}

func (lb *LoadBalancer) handleServerUpdate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ServerUrls map[string]int `json:"server_urls"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	servers := ProcessServerMap(payload.ServerUrls)
	lb.UpdateServers(servers)
	lb.HealthCheck()

	w.WriteHeader(http.StatusOK)
}

func ProcessServerMap(serverMap map[string]int) []*Server {
	var servers []*Server
	for server, weight := range serverMap {

		serverUrl, err := url.Parse(server)
		if err != nil {
			log.Fatal(err)
		}

		proxy := httputil.NewSingleHostReverseProxy(serverUrl)
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("Error: %v", err)
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		}

		servers = append(servers, &Server{
			URL:           serverUrl,
			Weight:        weight,
			ReverseProxy:  proxy,
			CurrentWeight: 0,
		})
	}

	return servers

}

func main() {
	port := flag.Int("port", 8080, "Port to listen on")
	flag.Parse()

	serverList := map[string]int{
		"http://localhost:8001": 1,
		"http://localhost:8002": 2,
		"http://localhost:8003": 3,
		//"http://localhost:8004": 4,
	}

	servers := ProcessServerMap(serverList)

	loadBalancer := &LoadBalancer{
		Servers: servers,
	}

	loadBalancer.HealthCheck()
	go loadBalancer.PeriodicHealthCheck(10 * time.Second)

	server := http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: loadBalancer,
	}
	log.Printf("Listening on port %d", *port)
	err := server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}

}
