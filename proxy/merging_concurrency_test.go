// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestRace_parallelMerge_sharedBackendResponse drives parallelMerge with
// backend proxies that return the same Response/Data map on every call,
// exactly like static or cached backends do. combineData must never mutate the
// maps owned by those shared responses, otherwise concurrent requests race on
// the same map ("fatal error: concurrent map writes").
func TestRace_parallelMerge_sharedBackendResponse(t *testing.T) {
	timeout := 200 * time.Millisecond
	shared := []Proxy{
		dummyProxy(&Response{Data: map[string]interface{}{"a": 1}, IsComplete: true}),
		dummyProxy(&Response{Data: map[string]interface{}{"b": 2}, IsComplete: true}),
		dummyProxy(&Response{Data: map[string]interface{}{"c": 3}, IsComplete: true}),
	}
	p := parallelMerge(
		func(r *Request) *Request { res := r.Clone(); return &res },
		timeout,
		combineData,
		nil,
		shared...,
	)

	var wg sync.WaitGroup
	for g := 0; g < 32; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				out, err := p(context.Background(), &Request{Params: map[string]string{}})
				if err != nil || out == nil || len(out.Data) != 3 {
					t.Errorf("unexpected merge result: %v err=%v", out, err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// TestRace_incrementalMergeAccumulator_concurrentMerge hammers Merge and Result
// from several goroutines to ensure the accumulator (and the combiner call it
// performs) is fully serialized.
func TestRace_incrementalMergeAccumulator_concurrentMerge(t *testing.T) {
	for g := 0; g < 50; g++ {
		acc := newIncrementalMergeAccumulator(4, combineData)
		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				acc.Merge(&Response{
					Data:       map[string]interface{}{"k": i},
					IsComplete: true,
				}, nil)
			}(i)
		}
		wg.Wait()
		res, err := acc.Result()
		if err != nil || res == nil || len(res.Data) != 1 {
			t.Fatalf("unexpected result: %v err=%v", res, err)
		}
	}
}

// TestCombineData_doesNotMutatePartMaps locks in the ownership rule: the
// collected parts keep their original Data untouched and the merged response
// owns a fresh map.
func TestCombineData_doesNotMutatePartMaps(t *testing.T) {
	first := &Response{Data: map[string]interface{}{"a": 1}, IsComplete: true}
	second := &Response{Data: map[string]interface{}{"b": 2}, IsComplete: true}

	merged := combineData(2, []*Response{first, second})

	if len(first.Data) != 1 {
		t.Errorf("first part map mutated: %v", first.Data)
	}
	if len(second.Data) != 1 {
		t.Errorf("second part map mutated: %v", second.Data)
	}
	if len(merged.Data) != 2 {
		t.Errorf("unexpected merged data: %v", merged.Data)
	}

	// mutating the result must not reach the backend-owned maps
	merged.Data["a"] = 99
	if first.Data["a"] != 1 {
		t.Errorf("merged map shares storage with part map: %v", first.Data)
	}
}

// TestRace_cloneSequentialReplacements_independentBackingArrays verifies that
// each built sequential proxy gets its own copy of the shared replacement
// matrix, so per-request changes cannot bleed into another request.
func TestRace_cloneSequentialReplacements_independentBackingArrays(t *testing.T) {
	original := [][]sequentialBackendReplacement{
		nil,
		{{backendIndex: 0, destination: "Resp0_id", source: []string{"id"}, fullResponse: false}},
	}
	clones := make([][][]sequentialBackendReplacement, 16)
	for i := range clones {
		clones[i] = cloneSequentialReplacements(original)
	}
	var wg sync.WaitGroup
	for i := range clones {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			clones[i][1][0].source[0] = "mutated"
			clones[i][1][0].destination = "changed"
		}(i)
	}
	wg.Wait()
	for _, c := range clones {
		if c[1][0].source[0] != "mutated" || c[1][0].destination != "changed" {
			t.Errorf("clone was overwritten by another goroutine: %+v", c[1][0])
		}
	}
	if original[1][0].source[0] != "id" || original[1][0].destination != "Resp0_id" {
		t.Errorf("original config mutated: %+v", original[1][0])
	}
}

// TestRace_combinerRegister_concurrentAccess exercises Set/Get through both the
// package-level register and a fresh one while other goroutines read.
func TestRace_combinerRegister_concurrentAccess(t *testing.T) {
	r := newCombinerRegister(nil, combineData)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				name := "combiner"
				r.SetResponseCombiner(name, combineData)
				if c, ok := r.GetResponseCombiner(name); c == nil || !ok {
					t.Error("missing registered combiner")
					return
				}
			}
		}(g)
	}
	wg.Wait()
}
