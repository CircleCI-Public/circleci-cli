// Copyright (c) 2026 Circle Internet Services, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// SPDX-License-Identifier: MIT

// Package bulkhead runs a slice of work with bounded parallelism.
package bulkhead

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// Do calls f for each element of input, running at most maxParallelism
// goroutines concurrently. f receives the element and its original index.
// The first error returned by any f cancels remaining work and is returned.
func Do[T any](ctx context.Context, maxParallelism int, input []T, f func(e T, i int) error) error {
	count := len(input)

	type elem struct {
		v T
		i int
	}

	ch := make(chan elem, count)
	defer close(ch)
	for i, v := range input {
		ch <- elem{v: v, i: i}
	}

	g, ctx := errgroup.WithContext(ctx)

	parallelism := min(maxParallelism, count)
	for range parallelism {
		g.Go(func() error {
			ctxDone := ctx.Done()
			for {
				select {
				case e := <-ch:
					if err := f(e.v, e.i); err != nil {
						return err
					}
				case <-ctxDone:
					return ctx.Err()
				default:
					return nil
				}
			}
		})
	}
	return g.Wait()
}
