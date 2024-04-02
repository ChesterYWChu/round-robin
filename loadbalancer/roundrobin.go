package main

import (
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

type RoundRobin struct {
	instances                    []RRInstance
	current                      uint32
	healthCheckIntervalInSeconds int
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
	next, err := rr.next()
	if err != nil {
		log.Printf("failed to find any alive instance")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	rr.instances[next].ServeHTTP(w, r)

	// log instance index for demo
	log.Printf("===========New Request===========\n")
	log.Printf("instance: %d\n", next)
}

func (rr *RoundRobin) next() (uint32, error) {
	length := uint32(len(rr.instances))
	if length == 0 {
		return 0, errors.New("instance list is empty")
	}
	for attempts := uint32(0); attempts < length; attempts++ {
		next := atomic.AddUint32(&rr.current, 1)
		instanceIdx := next % length

		if rr.instances[instanceIdx].IsAlive() {
			return instanceIdx, nil
		}
	}
	return 0, errors.New("failed to find any alive instance")
}

func (rr *RoundRobin) HealthCheck() {
	aliveness := make([]bool, len(rr.instances))
	for i, instance := range rr.instances {
		alive := instance.CheckAliveness()
		instance.SetAlive(alive)
		aliveness[i] = alive
	}

	// log health check result for demo
	log.Printf("===========Health Check===========\n")
	log.Printf("Aliveness: %+v\n", aliveness)
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
