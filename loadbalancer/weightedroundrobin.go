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
	log.Printf("WeightedRoundRobin URL: %s\n", r.URL)

	if len(wrr.weights) == 0 {
		log.Printf("failed to find any alive instance")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	for {
		next := uint64(atomic.AddUint32(&wrr.current, 1))

		instanceIdx := next % uint64(len(wrr.weights))
		round := next / uint64(len(wrr.weights))
		weight := uint64(wrr.weights[instanceIdx])

		mod := (weight * round) % MaxWeight

		log.Printf("next: %d\n", next)
		log.Printf("instanceIdx: %d\n", instanceIdx)
		log.Printf("round: %d\n", round)
		log.Printf("weight: %d\n", weight)
		log.Printf("mod: %d\n", mod)

		if mod > weight {
			continue
		}
		if !wrr.instances[instanceIdx].IsAlive() {
			continue
		}

		startTime := time.Now()
		wrr.instances[instanceIdx].ServeHTTP(w, r)

		responseTime := time.Since(startTime).Nanoseconds()
		wrr.instances[instanceIdx].SetEWMALatency(responseTime)
		log.Printf("========responseTime: %d\n", responseTime)
		return
	}
}

func (wrr *WeightedRoundRobin) HealthCheck() {
	for _, i := range wrr.instances {
		alive := i.CheckAliveness()
		i.SetAlive(alive)
	}

	length := len(wrr.instances)
	weights := make([]float64, length)
	max := float64(0.0)
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
