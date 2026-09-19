// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
)

// TestParallelMerge_Concurrent runs many requests through a parallel merger
// concurrently. Backends return a fresh Response on every call, so any data
// race detected here (run with -race) comes from the merging machinery and
// not from shared fixtures.
func TestParallelMerge_Concurrent(t *testing.T) {
	timeout := 500 * time.Millisecond
	endpoint := config.EndpointConfig{
		Backend: []*config.Backend{
			{URLPattern: "/a"},
			{URLPattern: "/b"},
			{URLPattern: "/c"},
		},
		Timeout: timeout,
	}

	backend := func(key string) Proxy {
		return func(_ context.Context, _ *Request) (*Response, error) {
			return &Response{
				Data:       map[string]interface{}{key: key},
				IsComplete: true,
			}, nil
		}
	}

	mw := NewMergeDataMiddleware(logging.NoOp, &endpoint)
	p := mw(backend("a"), backend("b"), backend("c"))

	const workers = 32
	const iterations = 50

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				resp, err := p(context.Background(), &Request{Params: map[string]string{}})
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if resp == nil || len(resp.Data) != 3 {
					t.Errorf("unexpected response: %+v", resp)
					return
				}
				if !resp.IsComplete {
					t.Errorf("response should be complete")
					return
				}
			}
		}()
	}
	wg.Wait()
}

// TestSequentialMerge_Concurrent runs many requests through a sequential
// merger concurrently. The merger is built once, so the sequential
// replacements configuration is shared by all the requests, and any
// unsynchronized access to it (or to the per-request replacement registry)
// is caught by the race detector.
func TestSequentialMerge_Concurrent(t *testing.T) {
	timeout := 500 * time.Millisecond
	endpoint := config.EndpointConfig{
		Backend: []*config.Backend{
			{URLPattern: "/"},
			{URLPattern: "/aaa/{{.Resp0_int}}/{{.Resp0_string}}"},
			{URLPattern: "/bbb/{{.Resp0_struct.foo}}?x={{.Resp1_tupu}}"},
		},
		Timeout: timeout,
		ExtraConfig: config.ExtraConfig{
			Namespace: map[string]interface{}{
				isSequentialKey:        true,
				sequentialPropagateKey: []interface{}{"resp0_propagated"},
			},
		},
	}

	mw := NewMergeDataMiddleware(logging.NoOp, &endpoint)
	p := mw(
		func(_ context.Context, _ *Request) (*Response, error) {
			return &Response{
				Data: map[string]interface{}{
					"int":        42,
					"string":     "some",
					"struct":     map[string]interface{}{"foo": "bar"},
					"propagated": "everywhere",
				},
				IsComplete: true,
			}, nil
		},
		func(_ context.Context, r *Request) (*Response, error) {
			if r.Params["Resp0_int"] != "42" || r.Params["Resp0_string"] != "some" {
				return nil, fmt.Errorf("unexpected params: %v", r.Params)
			}
			return &Response{
				Data:       map[string]interface{}{"tupu": "foo"},
				IsComplete: true,
			}, nil
		},
		func(_ context.Context, r *Request) (*Response, error) {
			if r.Params["Resp0_struct.foo"] != "bar" || r.Params["Resp1_tupu"] != "foo" {
				return nil, fmt.Errorf("unexpected params: %v", r.Params)
			}
			if r.Params["Resp0_propagated"] != "everywhere" {
				return nil, fmt.Errorf("unexpected params: %v", r.Params)
			}
			return &Response{
				Data:       map[string]interface{}{"done": true},
				IsComplete: true,
			}, nil
		},
	)

	const workers = 32
	const iterations = 50

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				resp, err := p(context.Background(), &Request{Params: map[string]string{}})
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if resp == nil || !resp.IsComplete {
					t.Errorf("unexpected response: %+v", resp)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// TestCombinerRegister_Concurrent exercises a response combiner register
// with concurrent writers and readers. It uses a local register so the
// global one is not polluted for other tests.
func TestCombinerRegister_Concurrent(t *testing.T) {
	cr := newCombinerRegister(map[string]ResponseCombiner{defaultCombinerName: combineData}, combineData)

	const workers = 32
	const iterations = 100

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		i := i
		wg.Add(2)
		go func() {
			defer wg.Done()
			name := fmt.Sprintf("combiner-%d", i)
			for j := 0; j < iterations; j++ {
				cr.SetResponseCombiner(name, combineData)
			}
		}()
		go func() {
			defer wg.Done()
			name := fmt.Sprintf("combiner-%d", i)
			for j := 0; j < iterations; j++ {
				cr.GetResponseCombiner(name)
				if c, ok := cr.GetResponseCombiner(defaultCombinerName); !ok || c == nil {
					t.Errorf("the default combiner should never be nil")
					return
				}
			}
		}()
	}
	wg.Wait()
}
