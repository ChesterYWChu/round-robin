package main

import (
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWeightedRoundRobinNext(t *testing.T) {
	t.Parallel()

	url8081 := func() *url.URL { u, _ := url.Parse("http://localhost:8081"); return u }()
	url8082 := func() *url.URL { u, _ := url.Parse("http://localhost:8082"); return u }()
	url8083 := func() *url.URL { u, _ := url.Parse("http://localhost:8083"); return u }()

	tests := []struct {
		name               string
		weightedRoundRobin *WeightedRoundRobin
		exp                uint64
		expErr             error
	}{
		{
			name:               "empty WeightedRoundRobin instance list",
			weightedRoundRobin: &WeightedRoundRobin{},
			exp:                0,
			expErr:             errors.New("weight list is empty"),
		},
		{
			name: "return mod of 31 when all instances have the same weight",
			weightedRoundRobin: &WeightedRoundRobin{
				instances: []WRRInstance{
					&WRRInstanceImpl{
						RRInstanceImpl: RRInstanceImpl{
							URL:   url8081,
							alive: true,
						},
						alpha:       0.7,
						ewmaLatency: 100,
					},
					&WRRInstanceImpl{
						RRInstanceImpl: RRInstanceImpl{
							URL:   url8082,
							alive: true,
						},
						alpha:       0.7,
						ewmaLatency: 100,
					},
					&WRRInstanceImpl{
						RRInstanceImpl: RRInstanceImpl{
							URL:   url8083,
							alive: true,
						},
						alpha:       0.7,
						ewmaLatency: 100,
					},
				},
				current:                      30, // next = 31 % 3 = 1
				healthCheckIntervalInSeconds: 5,
				weights:                      []uint16{65535, 65535, 65535},
			},
			exp:    1,
			expErr: nil,
		},
		{
			name: "skip mod < weight instance",
			weightedRoundRobin: &WeightedRoundRobin{
				instances: []WRRInstance{
					&WRRInstanceImpl{
						RRInstanceImpl: RRInstanceImpl{
							URL:   url8081,
							alive: true,
						},
						alpha:       0.7,
						ewmaLatency: 100,
					},
					&WRRInstanceImpl{
						RRInstanceImpl: RRInstanceImpl{
							URL:   url8082,
							alive: true,
						},
						alpha:       0.7,
						ewmaLatency: 100,
					},
					&WRRInstanceImpl{
						RRInstanceImpl: RRInstanceImpl{
							URL:   url8083,
							alive: true,
						},
						alpha:       0.7,
						ewmaLatency: 100,
					},
				},
				current:                      300,
				healthCheckIntervalInSeconds: 5,
				// next = 31 % 3 = 1
				// round = 300 / 3 = 100
				// weight = weights[1] = 20000
				// mod = (20000 * 100) % 65535 = 33950 > weight = 20000 ===> skip
				weights: []uint16{65535, 20000, 65535},
			},
			exp:    2,
			expErr: nil,
		},
		{
			name: "skip not alive instance",
			weightedRoundRobin: &WeightedRoundRobin{
				instances: []WRRInstance{
					&WRRInstanceImpl{
						RRInstanceImpl: RRInstanceImpl{
							URL:   url8081,
							alive: true,
						},
						alpha:       0.7,
						ewmaLatency: 100,
					},
					&WRRInstanceImpl{
						RRInstanceImpl: RRInstanceImpl{
							URL:   url8082,
							alive: false,
						},
						alpha:       0.7,
						ewmaLatency: 100,
					},
					&WRRInstanceImpl{
						RRInstanceImpl: RRInstanceImpl{
							URL:   url8083,
							alive: true,
						},
						alpha:       0.7,
						ewmaLatency: 100,
					},
				},
				current:                      30, // next = 31 % 3 = 1
				healthCheckIntervalInSeconds: 5,
				weights:                      []uint16{65535, 65535, 65535},
			},
			exp:    2,
			expErr: nil,
		},
		{
			name: "no alive instance",
			weightedRoundRobin: &WeightedRoundRobin{
				instances: []WRRInstance{
					&WRRInstanceImpl{
						RRInstanceImpl: RRInstanceImpl{
							URL:   url8081,
							alive: false,
						},
						alpha:       0.7,
						ewmaLatency: 100,
					},
					&WRRInstanceImpl{
						RRInstanceImpl: RRInstanceImpl{
							URL:   url8082,
							alive: false,
						},
						alpha:       0.7,
						ewmaLatency: 100,
					},
					&WRRInstanceImpl{
						RRInstanceImpl: RRInstanceImpl{
							URL:   url8083,
							alive: false,
						},
						alpha:       0.7,
						ewmaLatency: 100,
					},
				},
				current:                      30, // next = 31 % 3 = 1
				healthCheckIntervalInSeconds: 5,
				weights:                      []uint16{65535, 65535, 65535},
			},
			exp:    0,
			expErr: errors.New("failed to find any alive instance"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next, err := tt.weightedRoundRobin.next()
			if tt.expErr != nil {
				assert.EqualError(t, err, tt.expErr.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.exp, next)
			}
		})
	}
}
