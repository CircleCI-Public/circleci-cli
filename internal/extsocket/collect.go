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

package extsocket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"
)

// Collect accepts the first connection and reads one payload from it. One
// deadline covers the accept and the read together, and ctx can only shorten
// it. Call it after the extension exits: the kernel has queued the connection
// by then, so the accept waits at most acceptWait.
//
// Errors are ErrNoConnection, ErrNoPayload or ErrUnreadable, and never carry
// any of the payload.
func (l *Listener) Collect(ctx context.Context) (Payload, error) {
	ctx, cancel := context.WithTimeout(ctx, collectDeadline)
	defer cancel()
	deadline, _ := ctx.Deadline()

	// A cancelled context must end a blocked Accept or Read at once.
	acceptBy := time.Now().Add(acceptWait)
	if acceptBy.After(deadline) {
		acceptBy = deadline
	}
	_ = l.ln.SetDeadline(acceptBy)
	stop := context.AfterFunc(ctx, func() { _ = l.ln.SetDeadline(time.Now()) })
	conn, err := l.ln.Accept()
	stop()
	_ = l.ln.Close()
	if err != nil {
		return Payload{}, ErrNoConnection
	}
	defer func() { _ = conn.Close() }()

	_ = conn.SetReadDeadline(deadline)
	stopConn := context.AfterFunc(ctx, func() { _ = conn.SetReadDeadline(time.Now()) })
	defer stopConn()
	data, err := io.ReadAll(io.LimitReader(conn, maxPayload+1))
	switch {
	case err != nil, len(data) > maxPayload:
		return Payload{}, ErrUnreadable
	case len(data) == 0:
		return Payload{}, ErrNoPayload
	}
	return decode(data)
}

// unreadableError is ErrUnreadable with a fixed reason. The reason is never
// taken from a decoder error, which can quote the payload.
type unreadableError struct{ reason string }

func (e unreadableError) Error() string { return ErrUnreadable.Error() + ": " + e.reason }
func (e unreadableError) Is(target error) bool {
	return target == ErrUnreadable
}

// decode reads a single JSON object token by token, because encoding/json
// keeps the last of two equal keys silently. Only the top-level object and
// the attributes object are checked for duplicates; attribute values stay raw.
func decode(data []byte) (Payload, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := expectDelim(dec, '{', "payload is not an object"); err != nil {
		return Payload{}, err
	}
	var p Payload
	seen := map[string]bool{}
	for dec.More() {
		key, err := readKey(dec, seen)
		if err != nil {
			return Payload{}, err
		}
		switch key {
		case "version":
			var v *int
			if err := dec.Decode(&v); err != nil {
				return Payload{}, unreadableError{"version is not a whole number"}
			}
			if v == nil {
				return Payload{}, unreadableError{"version is null"}
			}
			p.Version = *v
		case "attributes":
			attrs, err := decodeAttributes(dec)
			if err != nil {
				return Payload{}, err
			}
			p.Attributes = attrs
		default:
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				return Payload{}, unreadableError{"invalid JSON"}
			}
		}
	}
	if err := expectDelim(dec, '}', "invalid JSON"); err != nil {
		return Payload{}, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Payload{}, unreadableError{"text after the object"}
	}
	return p, nil
}

func decodeAttributes(dec *json.Decoder) (map[string]json.RawMessage, error) {
	if err := expectDelim(dec, '{', "attributes is not an object"); err != nil {
		return nil, err
	}
	attrs := map[string]json.RawMessage{}
	seen := map[string]bool{}
	for dec.More() {
		key, err := readKey(dec, seen)
		if err != nil {
			return nil, err
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, unreadableError{"invalid JSON"}
		}
		attrs[key] = v
	}
	if err := expectDelim(dec, '}', "invalid JSON"); err != nil {
		return nil, err
	}
	return attrs, nil
}

func expectDelim(dec *json.Decoder, want json.Delim, reason string) error {
	tok, err := dec.Token()
	if err != nil {
		return unreadableError{"invalid JSON"}
	}
	if d, ok := tok.(json.Delim); !ok || d != want {
		return unreadableError{reason}
	}
	return nil
}

// readKey reads an object key and rejects one already in seen.
func readKey(dec *json.Decoder, seen map[string]bool) (string, error) {
	tok, err := dec.Token()
	if err != nil {
		return "", unreadableError{"invalid JSON"}
	}
	key, ok := tok.(string)
	if !ok {
		return "", unreadableError{"invalid JSON"}
	}
	if seen[key] {
		return "", unreadableError{"duplicate key"}
	}
	seen[key] = true
	return key, nil
}
