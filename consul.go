package tcpproxy

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/miekg/dns"
)

// dns.Client interface used to make testing easier; we use a mock client for the tests
type dnsClient interface {
	ExchangeContext(ctx context.Context, m *dns.Msg, a string) (r *dns.Msg, rtt time.Duration, err error)
}

// ConsulMatcher returns a Matcher that directs any hostname ending with its matchSuffix
// to the service name registered in Consul corresponding to that host's subdomain.
//
// For example, if the chosen suffix is "localhost" then "foo.localhost" will be directed
// to the consul service named "foo".
// Note "foo.bar.localhost" will _not_ match the suffix "localhost", only "bar.localhost".
//
// A DNS lookup to consulDnsAddr is done to resolve the service ip:port, analogous to:
//
//  dig @127.0.0.1 -p 8600 _foo._tcp.consul SRV
//
func ConsulMatcher(matchSuffix, consulDnsAddr string) Matcher {
	m := consulMatcher{
		matchSuffix:   newSuffixMatcher(matchSuffix, nil),
		consulDnsAddr: consulDnsAddr,

		udpClient: &dns.Client{Net: "udp"},
		tcpClient: &dns.Client{Net: "tcp"},
		//targets:   make(map[string]*DialProxy),
	}
	return m.Lookup
}

type consulMatcher struct {
	matchSuffix   *suffixMatcher
	consulDnsAddr string

	udpClient dnsClient
	tcpClient dnsClient
	//targets   map[string]*DialProxy
}

func (m *consulMatcher) Lookup(ctx context.Context, hostname string) (ok bool, target Target) {
	ok, serviceName := m.matchSuffix.hasSuffix(ctx, hostname)
	if !ok {
		return
	}
	addr, err := m.lookupDns(ctx, serviceName)
	if err != nil {
		log.Printf("tcpproxy: consul lookup failed: %s", err)
		return false, nil
	}
	target = To(addr) // TODO: Reuse targets
	return
}

func (m *consulMatcher) dnsSrvQuery(ctx context.Context, fqdn string) (ans, extra []dns.RR, err error) {
	msg := new(dns.Msg)
	msg.SetQuestion(fqdn, dns.TypeSRV)

	var r *dns.Msg
	r, _, err = m.udpClient.ExchangeContext(ctx, msg, m.consulDnsAddr)
	if err != nil {
		return
	}
	if r.Truncated {
		r, _, err = m.tcpClient.ExchangeContext(ctx, msg, m.consulDnsAddr)
		if err != nil {
			return
		}
	}

	switch r.Rcode {
	case dns.RcodeSuccess:
		return r.Answer, r.Extra, nil // success

	case dns.RcodeNameError:
		err = fmt.Errorf("non-existent name: %s", fqdn)
		return
	}
	err = fmt.Errorf("lookup failed for: %s\n%s", fqdn, r)
	return
}

func (m *consulMatcher) lookupDns(ctx context.Context, serviceName string) (addr string, err error) {
	fqdn := "_" + serviceName + "._tcp.consul."
	ans, extra, err := m.dnsSrvQuery(ctx, fqdn)
	if err != nil {
		return
	}
	var (
		port uint16
		node string
		host net.IP
	)

	ok := false
	for _, rr := range ans { // "answer" section; SRV records
		var srv *dns.SRV
		srv, ok = rr.(*dns.SRV)
		if ok {
			port, node = srv.Port, srv.Target
			break
		}
	}
	if !ok {
		err = fmt.Errorf("no valid SRV record found for: %s\n%s", fqdn, ans)
		return
	}

	ok = false
	for _, rr := range extra { // "additional" section; A and TXT records
		var a *dns.A
		a, ok = rr.(*dns.A)
		if ok {
			if a.Hdr.Name == node {
				host = a.A
				break
			}
		}
	}
	if !ok {
		err = fmt.Errorf("no valid A record found for: %s\n%s", fqdn, ans)
		return
	}

	return fmt.Sprintf("%s:%d", host, port), nil
}
