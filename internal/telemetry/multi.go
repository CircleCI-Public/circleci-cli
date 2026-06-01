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

package telemetry

import (
	"errors"

	"github.com/segmentio/analytics-go/v3"
)

type multiClient struct {
	delegates []analytics.Client
}

func (mc *multiClient) Close() error {
	errs := make([]error, 0, len(mc.delegates))
	for _, d := range mc.delegates {
		errs = append(errs, d.Close())
	}
	return errors.Join(errs...)
}

func (mc *multiClient) Enqueue(m analytics.Message) error {
	errs := make([]error, 0, len(mc.delegates))
	for _, d := range mc.delegates {
		errs = append(errs, d.Enqueue(m))
	}
	return errors.Join(errs...)
}

func (mc *multiClient) Add(client analytics.Client) {
	mc.delegates = append(mc.delegates, client)
}
