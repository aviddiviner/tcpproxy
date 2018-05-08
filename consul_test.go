package tcpproxy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

type mockDnsClient struct {
	Answers, Extras []string
	Truncate        bool
	Error           error
}

func (c mockDnsClient) ExchangeContext(_ context.Context, req *dns.Msg, _ string) (r *dns.Msg, _ time.Duration, err error) {
	if c.Error != nil {
		err = c.Error
		return
	}
	r = new(dns.Msg)
	r.SetReply(req)
	r.Truncated = c.Truncate
	for _, a := range c.Answers {
		rr, _ := dns.NewRR(a)
		r.Answer = append(r.Answer, rr)
	}
	for _, e := range c.Extras {
		rr, _ := dns.NewRR(e)
		r.Extra = append(r.Extra, rr)
	}
	return
}

func TestConsulMatcher(t *testing.T) {
	tests := []struct {
		name   string
		body   io.Reader
		suffix string
		match  bool

		answer, extra []string
		target        string
	}{
		{
			name:   "match",
			body:   strings.NewReader("GET / HTTP/1.1\r\nHost: foo.localhost\r\n\r\n"),
			suffix: ".localhost",
			match:  true,

			answer: []string{
				`_foo._tcp.consul.	0	IN	SRV	1 1 51885 MacBook-Pro.local.node.dc1.consul.`,
			},
			extra: []string{
				`MacBook-Pro.local.node.dc1.consul. 0 IN A	127.0.0.1`,
				`MacBook-Pro.local.node.dc1.consul. 0 IN TXT "consul-network-segment="`,
			},
			target: "127.0.0.1:51885",
		},
		{
			name:   "no-match",
			body:   strings.NewReader("GET / HTTP/1.1\r\nHost: foo.bar.localhost\r\n\r\n"),
			suffix: ".localhost",
			match:  false,
		},
	}
	for i, tt := range tests {
		name := tt.name
		if name == "" {
			name = fmt.Sprintf("test_index_%d", i)
		}
		t.Run(name, func(t *testing.T) {
			br := bufio.NewReader(tt.body)
			m := consulMatcher{
				matchSuffix: newSuffixMatcher(tt.suffix, nil),
				udpClient:   mockDnsClient{Answers: tt.answer, Extras: tt.extra},
			}
			r := httpHostMatch{m.Lookup}
			target := r.match(br)
			got := target != nil
			if got != tt.match {
				t.Fatalf("match = %v; valid %v", got, tt.match)
			}
			if tt.match {
				proxy := target.(*DialProxy)
				if proxy.Addr != tt.target {
					t.Fatalf("target = %v; valid %v", proxy.Addr, tt.target)
				}
			}
		})
	}
}
