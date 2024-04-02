package balancer

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

// RoundRobin implements balancer interface
type RoundRobin struct {
	instances                    []RRInstance
	current                      uint32
	healthCheckIntervalInSeconds int
}

// NewRoundRobin new a RoundRobin balancer
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

// ServeHTTP implements http.Handler
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

// next decides which instanceIndex the balancer should send the next request to
func (rr *RoundRobin) next() (uint32, error) {
	length := uint32(len(rr.instances))
	if length == 0 {
		return 0, errors.New("instance list is empty")
	}
	// loop to find an alive instance and retry no more than `length` times
	for i := uint32(0); i < length; i++ {
		next := atomic.AddUint32(&rr.current, 1)
		instanceIdx := next % length

		if rr.instances[instanceIdx].IsAlive() {
			return instanceIdx, nil
		}
		// continue until finding an alive instance
	}
	// all registered instances are not alive
	return 0, errors.New("failed to find any alive instance")
}

// HealthCheck run a round of health check on its instances
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

// GetHealthCheckInterval return its health check interval configuration
func (rr *RoundRobin) GetHealthCheckInterval() int {
	return rr.healthCheckIntervalInSeconds
}

// RRInstance defines the instance interface
type RRInstance interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	CheckAliveness() bool
	IsAlive() bool
	SetAlive(alive bool)
}

// RRInstanceImpl implements the RRInstance interface
type RRInstanceImpl struct {
	URL          *url.URL
	ReverseProxy *httputil.ReverseProxy

	mu    sync.RWMutex
	alive bool
}

// ServeHTTP implements http.Handler
func (i *RRInstanceImpl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	i.ReverseProxy.ServeHTTP(w, r)
}

// CheckAliveness dials a TCP connection to instance to check its aliveness
func (i *RRInstanceImpl) CheckAliveness() bool {
	conn, err := net.DialTimeout("tcp", i.URL.Host, 1*time.Second)
	if err != nil {
		log.Printf("failed to connect to url:%s with error:%s", i.URL.Host, err.Error())
		return false
	}
	defer conn.Close()
	return true
}

// IsAlive returns the alive field
func (i *RRInstanceImpl) IsAlive() bool {
	var alive bool
	i.mu.RLock()
	alive = i.alive
	i.mu.RUnlock()
	return alive
}

// SetAlive sets the alive field
func (i *RRInstanceImpl) SetAlive(alive bool) {
	i.mu.Lock()
	i.alive = alive
	i.mu.Unlock()
}
