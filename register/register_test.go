// SPDX-License-Identifier: Apache-2.0

package register

import (
	"sync"
	"testing"
)

func TestNamespaced(t *testing.T) {
	r := New()
	r.Register("namespace1", "name1", 42)
	r.AddNamespace("namespace1")
	r.AddNamespace("namespace2")
	r.Register("namespace2", "name2", true)

	nr, ok := r.Get("namespace1")
	if !ok {
		t.Error("namespace1 not found")
		return
	}
	if _, ok := nr.Get("name2"); ok {
		t.Error("name2 found into namespace1")
		return
	}
	v1, ok := nr.Get("name1")
	if !ok {
		t.Error("name1 not found")
		return
	}
	if i, ok := v1.(int); !ok || i != 42 {
		t.Error("unexpected value:", v1)
	}

	nr, ok = r.Get("namespace2")
	if !ok {
		t.Error("namespace2 not found")
		return
	}
	if _, ok := nr.Get("name1"); ok {
		t.Error("name1 found into namespace2")
		return
	}
	v2, ok := nr.Get("name2")
	if !ok {
		t.Error("name2 not found")
		return
	}
	if b, ok := v2.(bool); !ok || !b {
		t.Error("unexpected value:", v2)
	}
}

func TestUntyped_concurrentAccess(t *testing.T) {
	r := NewUntyped()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			r.Register("name", i)
		}
		close(done)
	}()
	for i := 0; i < 1000; i++ {
		r.Get("name")
		r.Clone()
	}
	<-done
	if v, ok := r.Get("name"); !ok || v != 999 {
		t.Errorf("unexpected final value: %v", v)
	}
}

func TestNamespaced_concurrentRegister(t *testing.T) {
	r := New()
	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				r.Register("ns", "name", g*100+i)
				if nr, ok := r.Get("ns"); !ok || nr == nil {
					t.Error("namespace lost during concurrent register")
					return
				}
			}
		}(g)
	}
	wg.Wait()
	nr, ok := r.Get("ns")
	if !ok || nr == nil {
		t.Fatal("namespace missing after concurrent registers")
	}
}
