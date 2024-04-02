package main

import (
	"errors"
	"log"
	"math"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

type WeightedRoundRobin struct {
	instances                    []WRRInstance
	current                      uint32
	healthCheckIntervalInSeconds int
	weights                      []uint16
	mu                           sync.RWMutex
}

func NewWeightedRoundRobin(urls []string, healthCheckIntervalInSeconds int) (*WeightedRoundRobin, error) {
	if len(urls) == 0 {
		return nil, errors.New("the input url list is empty")
	}
	instances := []WRRInstance{}
	for _, u := range urls {
		instanceURL, err := url.Parse(u)
		if err != nil {
			log.Printf("failed to parse url:%s with error: %s\n", u, err.Error())
			return nil, err
		}
		proxy := httputil.NewSingleHostReverseProxy(instanceURL)
		instances = append(instances, &WRRInstanceImpl{
			RRInstanceImpl: RRInstanceImpl{
				URL:          instanceURL,
				ReverseProxy: proxy,
				alive:        true,
			},
			alpha:       0.7,
			ewmaLatency: 1,
		})
	}
	return &WeightedRoundRobin{
		instances:                    instances,
		current:                      0,
		healthCheckIntervalInSeconds: healthCheckIntervalInSeconds,
	}, nil
}

const MaxWeight = math.MaxUint16

func (wrr *WeightedRoundRobin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	next, err := wrr.next()
	if err != nil {
		log.Printf("failed to find any alive instance")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	startTime := time.Now()
	wrr.instances[next].ServeHTTP(w, r)

	responseTime := time.Since(startTime).Nanoseconds()
	wrr.instances[next].SetEWMALatency(responseTime)

	// log instance index for demo
	log.Printf("===========New Request===========\n")
	log.Printf("instance: %d, responseTime: %d\n", next, responseTime)
}

func (wrr *WeightedRoundRobin) next() (uint64, error) {
	wrr.mu.RLock()
	defer wrr.mu.RUnlock()

	length := len(wrr.weights)
	if length == 0 {
		return 0, errors.New("weight list is empty")
	}

	for i := 0; i < length; i++ {
		next := uint64(atomic.AddUint32(&wrr.current, 1))

		instanceIdx := next % uint64(length)
		round := next / uint64(length)
		weight := uint64(wrr.weights[instanceIdx])

		mod := (weight * round) % MaxWeight
		if mod > weight {
			continue
		}
		if !wrr.instances[instanceIdx].IsAlive() {
			continue
		}
		return instanceIdx, nil
	}
	return 0, errors.New("failed to find any alive instance")
}

func (wrr *WeightedRoundRobin) HealthCheck() {
	wrr.mu.Lock()
	defer wrr.mu.Unlock()

	for _, i := range wrr.instances {
		alive := i.CheckAliveness()
		i.SetAlive(alive)
	}

	length := len(wrr.instances)
	weights := make([]float64, length)
	max := float64(0.0)

	// log health check result for demo
	log.Printf("===========Health Check===========\n")
	for i, instance := range wrr.instances {
		if !instance.IsAlive() {
			weights[i] = 0
			continue
		}
		latency := instance.GetEWMALatency()
		weights[i] = 1 / latency
		if weights[i] > max {
			max = weights[i]
		}
		log.Printf("EWMALatency: %f, Weight: %f\n", latency, weights[i])
	}

	scaledWeights := make([]uint16, length)
	scalingFactor := MaxWeight / max
	for i, w := range weights {
		scaledWeights[i] = uint16(math.Round(scalingFactor * w))
	}
	wrr.weights = scaledWeights
	log.Printf("weights: %+v\n", wrr.weights)
}

func (wrr *WeightedRoundRobin) GetHealthCheckInterval() int {
	return wrr.healthCheckIntervalInSeconds
}

type WRRInstance interface {
	RRInstance

	SetEWMALatency(newLatency int64)
	GetEWMALatency() float64
}

type WRRInstanceImpl struct {
	RRInstanceImpl

	mu          sync.RWMutex
	alpha       float64
	ewmaLatency float64
}

func (i *WRRInstanceImpl) SetEWMALatency(newLatency int64) {
	i.mu.Lock()
	defer i.mu.Unlock()

	newLatencyFloat := float64(newLatency)
	i.ewmaLatency = i.alpha*newLatencyFloat + (1-i.alpha)*i.ewmaLatency
}

func (i *WRRInstanceImpl) GetEWMALatency() float64 {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.ewmaLatency
}
