// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tcpproxy

import (
	"bufio"
	"bytes"
	"context"
	"net/http"
)

// AddHTTPHostRoute appends a route to the ipPort listener that
// routes to dest if the incoming HTTP/1.x Host header name is
// httpHost. If it doesn't match, rule processing continues for any
// additional routes on ipPort.
//
// The ipPort is any valid net.Listen TCP address.
func (p *Proxy) AddHTTPHostRoute(ipPort, httpHost string, dest Target) {
	p.AddHTTPHostMatcher(ipPort, equals(httpHost, dest))
}

// AddHTTPHostMatcher appends a route to the ipPort listener that
// routes to dest if the incoming HTTP/1.x Host header name is
// accepted by matcher. If it doesn't match, rule processing continues
// for any additional routes on ipPort.
//
// The ipPort is any valid net.Listen TCP address.
func (p *Proxy) AddHTTPHostMatcher(ipPort string, match Matcher) {
	p.addRoute(ipPort, httpHostMatch{match})
}

type httpHostMatch struct {
	matcher Matcher
}

func (m httpHostMatch) match(br *bufio.Reader) Target {
	if target, ok := m.matcher(context.TODO(), httpHostHeader(br)); ok {
		return target
	}
	return nil
}

// httpHostHeader returns the HTTP Host header from br without
// consuming any of its bytes. It returns "" if it can't find one.
func httpHostHeader(br *bufio.Reader) string {
	const maxPeek = 4 << 10
	peekSize := 0
	for {
		peekSize++
		if peekSize > maxPeek {
			b, _ := br.Peek(br.Buffered())
			return httpHostHeaderFromBytes(b)
		}
		b, err := br.Peek(peekSize)
		if n := br.Buffered(); n > peekSize {
			b, _ = br.Peek(n)
			peekSize = n
		}
		if len(b) > 0 {
			if b[0] < 'A' || b[0] > 'Z' {
				// Doesn't look like an HTTP verb
				// (GET, POST, etc).
				return ""
			}
			if bytes.Index(b, crlfcrlf) != -1 || bytes.Index(b, lflf) != -1 {
				req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(b)))
				if err != nil {
					return ""
				}
				if len(req.Header["Host"]) > 1 {
					// TODO(bradfitz): what does
					// ReadRequest do if there are
					// multiple Host headers?
					return ""
				}
				return req.Host
			}
		}
		if err != nil {
			return httpHostHeaderFromBytes(b)
		}
	}
}

var (
	lfHostColon = []byte("\nHost:")
	lfhostColon = []byte("\nhost:")
	crlf        = []byte("\r\n")
	lf          = []byte("\n")
	crlfcrlf    = []byte("\r\n\r\n")
	lflf        = []byte("\n\n")
)

func httpHostHeaderFromBytes(b []byte) string {
	if i := bytes.Index(b, lfHostColon); i != -1 {
		return string(bytes.TrimSpace(untilEOL(b[i+len(lfHostColon):])))
	}
	if i := bytes.Index(b, lfhostColon); i != -1 {
		return string(bytes.TrimSpace(untilEOL(b[i+len(lfhostColon):])))
	}
	return ""
}

// untilEOL returns v, truncated before the first '\n' byte, if any.
// The returned slice may include a '\r' at the end.
func untilEOL(v []byte) []byte {
	if i := bytes.IndexByte(v, '\n'); i != -1 {
		return v[:i]
	}
	return v
}

// HttpRedirect provides a target that will serve HTTP redirects to all incoming requests.
// The returned serve function will begin serving the redirects; it blocks on handling
// connections until started. Use it like:
//
//   redirect, serve := HttpRedirect("my.domain", "https://my.domain", http.StatusFound)
//   p.AddHTTPHostRoute(":80", "my.domain", redirect)
//   go serve()
//
func HttpRedirect(hostAddr, targetUrl string, statusCode int) (target Target, serve func() error) {
	redirect := &TargetListener{Address: hostAddr}
	handler := func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, targetUrl, statusCode)
	}
	return redirect, func() error {
		return http.Serve(redirect, http.HandlerFunc(handler))
	}
}
