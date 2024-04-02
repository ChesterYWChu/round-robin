package main

import (
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoundRobinNext(t *testing.T) {
	t.Parallel()

	url8081 := func() *url.URL { u, _ := url.Parse("http://localhost:8081"); return u }()
	url8082 := func() *url.URL { u, _ := url.Parse("http://localhost:8082"); return u }()
	url8083 := func() *url.URL { u, _ := url.Parse("http://localhost:8083"); return u }()

	tests := []struct {
		name       string
		roundRobin *RoundRobin
		exp        uint32
		expErr     error
	}{
		{
			name:       "empty RoundRobin instance list",
			roundRobin: &RoundRobin{},
			exp:        0,
			expErr:     errors.New("instance list is empty"),
		},
		{
			name: "return mod of 31",
			roundRobin: &RoundRobin{
				instances: []RRInstance{
					&RRInstanceImpl{
						URL:   url8081,
						alive: true,
					},
					&RRInstanceImpl{
						URL:   url8082,
						alive: true,
					},
					&RRInstanceImpl{
						URL:   url8083,
						alive: true,
					},
				},
				current:                      30, // next = 31 % 3 = 1
				healthCheckIntervalInSeconds: 5,
			},
			exp:    1,
			expErr: nil,
		},
		{
			name: "skip not alive instance",
			roundRobin: &RoundRobin{
				instances: []RRInstance{
					&RRInstanceImpl{
						URL:   url8081,
						alive: true,
					},
					&RRInstanceImpl{
						URL:   url8082,
						alive: false,
					},
					&RRInstanceImpl{
						URL:   url8083,
						alive: true,
					},
				},
				current:                      30, // next = 31 % 3 = 1
				healthCheckIntervalInSeconds: 5,
			},
			exp:    2,
			expErr: nil,
		},
		{
			name: "no alive instance",
			roundRobin: &RoundRobin{
				instances: []RRInstance{
					&RRInstanceImpl{
						URL:   url8081,
						alive: false,
					},
					&RRInstanceImpl{
						URL:   url8082,
						alive: false,
					},
					&RRInstanceImpl{
						URL:   url8083,
						alive: false,
					},
				},
				current:                      30, // next = 31 % 3 = 1
				healthCheckIntervalInSeconds: 5,
			},
			exp:    0,
			expErr: errors.New("failed to find any alive instance"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next, err := tt.roundRobin.next()
			if tt.expErr != nil {
				assert.EqualError(t, err, tt.expErr.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.exp, next)
			}
		})
	}
}
