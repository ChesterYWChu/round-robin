package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

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
	log.Printf("MyHTTPHandler URL: %s\n", r.URL)
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

	// balancer, err := NewRoundRobin(strings.Split(urls, ","), 5)
	balancer, err := NewWeightedRoundRobin(strings.Split(urls, ","), 5)
	if err != nil {
		log.Fatal(err)
	}
	lbSrv := NewLoadBalancerServer(balancer)
	lbSrv.Start()
	defer lbSrv.Close()

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: lbSrv,
	}

	log.Printf("listen on: %s\n", srv.Addr)
	srv.ListenAndServe()
}
