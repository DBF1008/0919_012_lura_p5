// SPDX-License-Identifier: Apache-2.0

package register

import (
	"fmt"
	"sync"
	"testing"
)

// TestUntyped_Concurrent exercises the Untyped register with concurrent
// writers and readers. Run with -race.
func TestUntyped_Concurrent(t *testing.T) {
	r := NewUntyped()

	const workers = 32
	const iterations = 100

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		i := i
		wg.Add(3)
		go func() {
			defer wg.Done()
			name := fmt.Sprintf("key-%d", i)
			for j := 0; j < iterations; j++ {
				r.Register(name, j)
			}
		}()
		go func() {
			defer wg.Done()
			name := fmt.Sprintf("key-%d", i)
			for j := 0; j < iterations; j++ {
				r.Get(name)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				r.Clone()
			}
		}()
	}
	wg.Wait()
}

// TestNamespaced_Concurrent exercises the Namespaced register with
// concurrent writers and readers over a pre-existing namespace.
func TestNamespaced_Concurrent(t *testing.T) {
	n := New()
	n.AddNamespace("ns")

	const workers = 32
	const iterations = 100

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		i := i
		wg.Add(2)
		go func() {
			defer wg.Done()
			name := fmt.Sprintf("key-%d", i)
			for j := 0; j < iterations; j++ {
				n.Register("ns", name, j)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				reg, ok := n.Get("ns")
				if !ok {
					t.Errorf("namespace ns should exist")
					return
				}
				reg.Get(fmt.Sprintf("key-%d", i))
			}
		}()
	}
	wg.Wait()
}
