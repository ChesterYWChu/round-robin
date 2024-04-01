package main

import (
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

type RoundRobin struct {
	instances                    []RRInstance
	current                      int
	healthCheckIntervalInSeconds int
	nextInstanceRWMutex          sync.RWMutex
}

func NewRoundRobin(urls []string, healthCheckIntervalInSeconds int) (*RoundRobin, error) {
	if len(urls) == 0 {
		return nil, errors.New("the input url list is empty")
	}
	instances := []RRInstance{}
	for _, u := range urls {
		instanceURL, err := url.Parse(u)
		if err != nil {
			log.Printf("failed to parse url:%s with error: %s\n", u, err.Error())
			return nil, err
		}
		proxy := httputil.NewSingleHostReverseProxy(instanceURL)
		instances = append(instances, &RRInstanceImpl{
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
	nextInstance.ServeHTTP(w, r)
}

func (rr *RoundRobin) nextInstance() RRInstance {
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

func (rr *RoundRobin) HealthCheck() {
	for _, i := range rr.instances {
		alive := i.CheckAliveness()
		i.SetAlive(alive)
	}
}

func (rr *RoundRobin) GetHealthCheckInterval() int {
	return rr.healthCheckIntervalInSeconds
}

type RRInstance interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	CheckAliveness() bool
	IsAlive() bool
	SetAlive(alive bool)
}

type RRInstanceImpl struct {
	URL          *url.URL
	ReverseProxy *httputil.ReverseProxy

	mu    sync.RWMutex
	alive bool
}

func (i *RRInstanceImpl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	i.ReverseProxy.ServeHTTP(w, r)
}

func (i *RRInstanceImpl) CheckAliveness() bool {
	conn, err := net.DialTimeout("tcp", i.URL.Host, 1*time.Second)
	if err != nil {
		log.Printf("failed to connect to url:%s with error:%s", i.URL.Host, err.Error())
		return false
	}
	defer conn.Close()
	return true
}

func (i *RRInstanceImpl) IsAlive() bool {
	var alive bool
	i.mu.RLock()
	alive = i.alive
	i.mu.RUnlock()
	return alive
}

func (i *RRInstanceImpl) SetAlive(alive bool) {
	i.mu.Lock()
	i.alive = alive
	i.mu.Unlock()
}
