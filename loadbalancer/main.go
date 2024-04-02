package main

import (
	"app/loadbalancer/balancer"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// Usage: go run loadbalancer/main.go -port 8080 -urls http://localhost:8081,http://localhost:8082,http://localhost:8083
// Example CURL: curl -d '{"game":"Mobile Legends", "gamerID":"GYUTDTE", "points":20}' -H "Content-Type: application/json" -X POST http://localhost:8080/echo

type Balancer interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
	HealthCheck()
	GetHealthCheckInterval() int
}

type LoadBalancerServer struct {
	balancer Balancer
	handler  http.Handler

	stopHealthCheck func()
}

func NewLoadBalancerServer(b Balancer) *LoadBalancerServer {
	// route all POST requests to loadbalancer
	r := mux.NewRouter()
	r.PathPrefix("/").Methods("POST").Handler(b)
	return &LoadBalancerServer{
		balancer: b,
		handler:  r,
	}
}

func (h *LoadBalancerServer) Start() {
	// run health check for the loadbalancer instances
	var ctx context.Context
	ctx, h.stopHealthCheck = context.WithCancel(context.Background())
	h.RunHealthCheck(ctx, h.balancer.GetHealthCheckInterval(), h.balancer.HealthCheck)
}

func (h *LoadBalancerServer) Close() {
	if h.stopHealthCheck != nil {
		h.stopHealthCheck()
		h.stopHealthCheck = nil
	}
}

func (h *LoadBalancerServer) RunHealthCheck(ctx context.Context, intervalInSeconds int, healthCheckFunc func()) {
	go func() {
		ticker := time.NewTicker(time.Second * time.Duration(intervalInSeconds))
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				healthCheckFunc()
			}
		}
	}()
}

func (h *LoadBalancerServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handler.ServeHTTP(w, r)
}

func main() {
	var port int
	var urls string
	flag.IntVar(&port, "port", 8080, "port to listen")
	flag.StringVar(&urls, "urls", "", "target urls seperate by comma, e.g., \"http://0.0.0.0:8081,http://0.0.0.0:8082\"")
	flag.Parse()

	if urls == "" {
		log.Fatal("Input urls is empty. See \"go run main.go -h\" for more info.")
	}

	// new a balancer to use
	// RoundRobin balancer support simple round robin algorithm
	// WeightedRoundRobin balancer support weighted round robin based on the request response time
	balancer, err := balancer.NewRoundRobin(strings.Split(urls, ","), 5)
	// balancer, err := balancer.NewWeightedRoundRobin(strings.Split(urls, ","), 5)
	if err != nil {
		log.Fatal(err)
	}

	// new a load balancer server and start the its health check
	lbSrv := NewLoadBalancerServer(balancer)
	lbSrv.Start()
	defer lbSrv.Close()

	// start http server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: lbSrv,
	}
	log.Printf("listen on: %s\n", srv.Addr)
	srv.ListenAndServe()
}
