package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

type LoadBalancerServer struct {
	lb      LoadBalancer
	handler http.Handler
}

func NewLoadBalancerServer(lb LoadBalancer) *LoadBalancerServer {
	// run health check for the loadbalancer instances
	go lb.RunHealthCheck()

	// route all POST requests to loadbalancer
	r := mux.NewRouter()
	r.PathPrefix("/").Methods("POST").Handler(lb)
	return &LoadBalancerServer{
		lb:      lb,
		handler: r,
	}
}

func (h *LoadBalancerServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("MyHTTPHandler URL: %s\n", r.URL)
	h.handler.ServeHTTP(w, r)
}

type LoadBalancer interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
	RunHealthCheck()
}

type WeightedRoundRobin struct {
	RoundRobin
}

type RoundRobin struct {
	instances                    []*Instance
	current                      int
	healthCheckIntervalInSeconds int
	nextInstanceRWMutex          sync.RWMutex
}

func NewRoundRobin(urls []string, healthCheckIntervalInSeconds int) (*RoundRobin, error) {
	if len(urls) == 0 {
		return nil, errors.New("the input url list is empty")
	}
	instances := []*Instance{}
	for _, u := range urls {
		instanceURL, err := url.Parse(u)
		if err != nil {
			log.Printf("failed to parse url:%s with error: %s\n", u, err.Error())
			return nil, err
		}
		proxy := httputil.NewSingleHostReverseProxy(instanceURL)
		instances = append(instances, &Instance{
			URL:          instanceURL,
			ReverseProxy: proxy,
			alive:        true,
		})
	}
	return &RoundRobin{
		instances:                    instances,
		current:                      0,
		healthCheckIntervalInSeconds: healthCheckIntervalInSeconds,
	}, nil
}

func (rr *RoundRobin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("RoundRobin URL: %s\n", r.URL)
	nextInstance := rr.nextInstance()
	if nextInstance == nil {
		log.Printf("failed to find any alive instance")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	nextInstance.ReverseProxy.ServeHTTP(w, r)
}

func (wrr *WeightedRoundRobin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("WeightedRoundRobin URL: %s\n", r.URL)

}

func (rr *RoundRobin) nextInstance() *Instance {
	rr.nextInstanceRWMutex.Lock()
	defer rr.nextInstanceRWMutex.Unlock()

	for attempts := 0; attempts < len(rr.instances); attempts++ {
		rr.current++
		if rr.current >= len(rr.instances) {
			rr.current = 0
		}
		if rr.instances[rr.current].IsAlive() {
			return rr.instances[rr.current]
		}
	}
	return nil
}

func (rr *RoundRobin) RunHealthCheck() {
	for {
		rr.healthCheck()
		time.Sleep(time.Second * time.Duration(rr.healthCheckIntervalInSeconds))
	}
}

func (rr *RoundRobin) healthCheck() {
	for _, i := range rr.instances {
		alive := i.CheckAliveness()
		i.SetAlive(alive)
	}
}

// type Instance interface {
// 	CheckAliveness() bool
// 	IsAlive() bool
// 	SetAlive(alive bool)
// }

type Instance struct {
	URL          *url.URL
	ReverseProxy *httputil.ReverseProxy

	mu    sync.RWMutex
	alive bool
}

// type WeightedInstance struct {
// 	Instance
// }

func (i *Instance) CheckAliveness() bool {
	conn, err := net.DialTimeout("tcp", i.URL.Host, 1*time.Second)
	if err != nil {
		log.Printf("failed to connect to url:%s with error:%s", i.URL.Host, err.Error())
		return false
	}
	defer conn.Close()
	return true
}

func (i *Instance) IsAlive() bool {
	var alive bool
	i.mu.RLock()
	alive = i.alive
	i.mu.RUnlock()
	return alive
}

func (i *Instance) SetAlive(alive bool) {
	i.mu.Lock()
	i.alive = alive
	i.mu.Unlock()
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
	roundRobin, err := NewRoundRobin(strings.Split(urls, ","), 5)
	if err != nil {
		log.Fatal(err)
	}
	handler := NewLoadBalancerServer(roundRobin)
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,
	}

	log.Printf("listen on: %s\n", srv.Addr)
	srv.ListenAndServe()
}
