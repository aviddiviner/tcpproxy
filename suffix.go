package tcpproxy

import (
	"context"
	"strings"
)

// SuffixMatcher directs all hosts with the given domain suffix to a single target.
func SuffixMatcher(suffix string, target Target) Matcher {
	return newSuffixMatcher(suffix, target).Lookup
}

func newSuffixMatcher(suffix string, target Target) *suffixMatcher {
	suffix = "." + strings.TrimLeft(suffix, ".")
	return &suffixMatcher{suffix, target}
}

type suffixMatcher struct {
	suffix string
	target Target
}

func (m *suffixMatcher) Lookup(ctx context.Context, hostname string) (bool, Target) {
	ok, _ := m.hasSuffix(ctx, hostname)
	if ok {
		return true, m.target
	}
	return false, nil
}

func (m *suffixMatcher) hasSuffix(_ context.Context, hostname string) (bool, string) {
	if strings.HasSuffix(hostname, m.suffix) {
		prefix := hostname[:len(hostname)-len(m.suffix)]
		if prefix == "" || strings.Contains(prefix, ".") {
			return false, ""
		}
		return true, prefix
	}
	return false, ""
}
