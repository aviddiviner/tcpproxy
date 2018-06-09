# tcpproxy

This is a fork of https://github.com/google/tcpproxy/

For usage, see https://godoc.org/github.com/aviddiviner/tcpproxy/

Notable differences with this package:

1. The old `Matcher` type has been replaced with more of a "routing" matcher. The old type did a simple boolean match against the found hostname:
   ```go
   // Matcher reports whether hostname matches the Matcher's criteria.
   type Matcher func(ctx context.Context, hostname string) bool
   ```
   The new `Matcher` also returns its `Target` (where an incoming matched connection is sent), allowing you to dynamically retarget based on the hostname:
   ```go
   // Matcher checks whether a hostname matches its criteria and, if true, returns
   // the target where the incoming matched connection should be sent to.
   type Matcher func(ctx context.Context, hostname string) (t Target, ok bool)
   ```

2. Some new matchers have been added;
    1. `SuffixMatcher` which directs all hosts with the given domain suffix to a target, and
    2. `ConsulMatcher` which directs hosts based on DNS lookups in Consul of service names matching that host's subdomain.

3. The logic around ACME tls-sni-01 challenges (and `AddStopACMESearch` function) has been removed. This feature was found vulnerable to certain exploits<sup>[1](#footnote1)</sup> and disabled by Let's Encrypt<sup>[2](#footnote2)</sup>.

---

1. <a name="footnote1"></a>https://labs.detectify.com/2018/01/12/how-i-exploited-acme-tls-sni-01-issuing-lets-encrypt-ssl-certs-for-any-domain-using-shared-hosting/
2. <a name="footnote2"></a>https://community.letsencrypt.org/t/2018-01-11-update-regarding-acme-tls-sni-and-shared-hosting-infrastructure/50188
