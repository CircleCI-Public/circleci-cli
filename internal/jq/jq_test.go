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

package jq_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/MakeNowJust/heredoc"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/jq"
)

func TestEvaluate(t *testing.T) {
	tests := []struct {
		name        string
		input       io.Reader
		opts        jq.Options
		expectError string
	}{
		{
			name:  "simple",
			input: strings.NewReader(`{"name":"Mona", "arms":8}`),
			opts:  jq.Options{Expr: `.name`},
		},
		{
			name:  "multiple_queries",
			input: strings.NewReader(`{"name":"Mona", "arms":8}`),
			opts:  jq.Options{Expr: `.name,.arms`},
		},
		{
			name:  "object_as_json",
			input: strings.NewReader(`{"user":{"login":"monalisa"}}`),
			opts:  jq.Options{Expr: `.user`},
		},
		{
			name:  "object_as_json_indented",
			input: strings.NewReader(`{"user":{"login":"monalisa"}}`),
			opts:  jq.Options{Expr: `.user`, Indent: "  "},
		},
		{
			name:  "object_as_json_indented_colorized",
			input: strings.NewReader(`{"user":{"login":"monalisa"}}`),
			opts:  jq.Options{Expr: `.user`, Indent: "  ", Colorize: true},
		},
		{
			name:  "empty_array",
			input: strings.NewReader(`[]`),
			opts:  jq.Options{Expr: `., [], unique`},
		},
		{
			name:  "empty_array_colorized",
			input: strings.NewReader(`[]`),
			opts:  jq.Options{Expr: `.`, Colorize: true},
		},
		{
			name: "complex",
			input: strings.NewReader(heredoc.Doc(`[
				{
					"title": "First title",
					"labels": [{"name":"bug"}, {"name":"help wanted"}]
				},
				{
					"title": "Second but not last",
					"labels": []
				},
				{
					"title": "Alas, tis' the end",
					"labels": [{}, {"name":"feature"}]
				}
			]`)),
			opts: jq.Options{Expr: `.[] | [.title,(.labels | map(.name) | join(","))] | @tsv`},
		},
		{
			name: "scalars_arrays_objects",
			input: strings.NewReader(heredoc.Doc(`[
				"foo",
				true,
				42,
				[17, 23],
				{"foo": "bar"}
			]`)),
			opts: jq.Options{Expr: `.[]`, Indent: "  ", Colorize: true},
		},
		{
			name:  "halt_function",
			input: strings.NewReader("{}"),
			opts:  jq.Options{Expr: `1,halt,2`},
		},
		{
			name:        "halt_error_function",
			input:       strings.NewReader("{}"),
			opts:        jq.Options{Expr: `1,halt_error,2`},
			expectError: "halt error: {}",
		},
		{
			name:  "invalid_one_line_query",
			input: strings.NewReader("{}"),
			opts:  jq.Options{Expr: `[1,2,,3]`},
			expectError: `failed to parse jq expression at line 1, column 6:
    [1,2,,3]
         ^  unexpected token ","`,
		},
		{
			name:  "invalid_multi_line_query",
			input: strings.NewReader("{}"),
			opts: jq.Options{Expr: `[
  1,,2
  ,3]`},
			expectError: `failed to parse jq expression at line 2, column 5:
      1,,2
        ^  unexpected token ","`,
		},
		{
			name:  "invalid_unterminated_query",
			input: strings.NewReader("{}"),
			opts:  jq.Options{Expr: `[1,`},
			expectError: `failed to parse jq expression at line 1, column 4:
    [1,
       ^  unexpected EOF`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w bytes.Buffer
			err := jq.Evaluate(tt.input, &w, tt.opts)
			if tt.expectError != "" {
				assert.Check(t, cmp.ErrorContains(err, tt.expectError))
				return
			}
			assert.NilError(t, err)
			assert.Check(t, golden.String(w.String(), tt.name+".txt"))
		})
	}
}

// TestErrorClassification checks that failures originating in the jq expression
// (parse, compile, runtime eval) surface as *jq.Error, while failures in the
// input data do not — so callers can tell a bad --jq expression apart from bad
// data.
func TestErrorClassification(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expr   string
		stream bool // use EvaluateStream instead of Evaluate
		wantJQ bool // expect the error to be a *jq.Error
	}{
		{name: "parse_error", input: `{}`, expr: `[1,`, wantJQ: true},
		{name: "compile_error", input: `{}`, expr: `.[] | undefined_func`, wantJQ: true},
		{name: "eval_error", input: `{"name":"a"}`, expr: `.name + 1`, wantJQ: true},
		{name: "eval_error_stream", input: `{"result":1}`, expr: `{(.result): 1}`, stream: true, wantJQ: true},
		// inputs is only defined for the stream evaluator; a single-value
		// Evaluate rejects it at compile time — still a jq expression error.
		{name: "inputs_without_stream", input: `{}`, expr: `[.,inputs]`, wantJQ: true},
		// A malformed value in the input stream is a data error, not a jq error.
		{name: "malformed_input", input: `{oops}`, expr: `.`, stream: true, wantJQ: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w bytes.Buffer
			var err error
			if tt.stream {
				err = jq.EvaluateStream(strings.NewReader(tt.input), &w, jq.Options{Expr: tt.expr})
			} else {
				err = jq.Evaluate(strings.NewReader(tt.input), &w, jq.Options{Expr: tt.expr})
			}
			assert.Assert(t, err != nil)

			var jqErr *jq.Error
			isJQ := errors.As(err, &jqErr)
			assert.Check(t, cmp.Equal(isJQ, tt.wantJQ), "err: %v", err)
			if isJQ {
				assert.Check(t, cmp.Equal(jqErr.Expr, tt.expr))
			}
		})
	}
}

// stream is three JSONL test-result records, as a JSON value stream would be
// produced downstream of a JSONL writer.
const stream = `{"name":"a","result":"failure","run_time":1.5}
{"name":"b","result":"failure","run_time":2}
{"name":"c","result":"skipped","run_time":0.5}
`

func TestEvaluateStream(t *testing.T) {
	tests := []struct {
		name        string
		input       io.Reader
		opts        jq.Options
		expectError string
	}{
		{
			// A simple filter runs once per record: one result per line.
			name:  "stream_per_record",
			input: strings.NewReader(stream),
			opts:  jq.Options{Expr: `.name`},
		},
		{
			// inputs pulls the remaining records so the whole set is aggregated.
			name:  "stream_count",
			input: strings.NewReader(stream),
			opts:  jq.Options{Expr: `[.,inputs] | length`},
		},
		{
			name:  "stream_group_by",
			input: strings.NewReader(stream),
			opts:  jq.Options{Expr: `[.,inputs] | group_by(.result) | map({(.[0].result): length})`, Indent: "  "},
		},
		{
			name:  "stream_reduce_sum",
			input: strings.NewReader(stream),
			opts:  jq.Options{Expr: `reduce inputs as $x (.run_time; . + $x.run_time)`},
		},
		{
			// An empty stream yields no output (like jq without --null-input).
			name:  "stream_empty",
			input: strings.NewReader(""),
			opts:  jq.Options{Expr: `[.,inputs] | length`},
		},
		{
			// halt stops the whole stream, not just the current record.
			name:  "stream_halt",
			input: strings.NewReader(stream),
			opts:  jq.Options{Expr: `.name, halt`},
		},
		{
			name:        "stream_malformed",
			input:       strings.NewReader(`{"name":"a"} {oops}`),
			opts:        jq.Options{Expr: `.name`},
			expectError: "invalid character",
		},
		{
			name:  "stream_invalid_query",
			input: strings.NewReader(stream),
			opts:  jq.Options{Expr: `[1,2,,3]`},
			expectError: `failed to parse jq expression at line 1, column 6:
    [1,2,,3]
         ^  unexpected token ","`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w bytes.Buffer
			err := jq.EvaluateStream(tt.input, &w, tt.opts)
			if tt.expectError != "" {
				assert.Check(t, cmp.ErrorContains(err, tt.expectError))
				return
			}
			assert.NilError(t, err)
			assert.Check(t, golden.String(w.String(), tt.name+".txt"))
		})
	}
}
