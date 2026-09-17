package main

// match.go - deciding which computers an alert is for.

import (
	"net"
	"strings"
	"unicode"
)

var validFields = map[string]string{
	"hostname": "Computer name",
	"user":     "Username",
	"ou":       "AD organisational unit",
	"group":    "AD group",
	"ip":       "IP address",
}

func equalFold(a, b string) bool { return strings.EqualFold(a, b) }

func hasWildcard(p string) bool { return strings.ContainsAny(p, "*?") }

// wildcardMatch is a case-insensitive glob: * matches any run, ? matches one character.
func wildcardMatch(pattern, s string) bool {
	p := []rune(strings.ToLower(pattern))
	t := []rune(strings.ToLower(s))
	pi, ti := 0, 0
	star, mark := -1, 0
	for ti < len(t) {
		switch {
		case pi < len(p) && (p[pi] == '?' || p[pi] == t[ti]):
			pi++
			ti++
		case pi < len(p) && p[pi] == '*':
			star, mark = pi, ti
			pi++
		case star >= 0:
			pi = star + 1
			mark++
			ti = mark
		default:
			return false
		}
	}
	for pi < len(p) && p[pi] == '*' {
		pi++
	}
	return pi == len(p)
}

func ruleMatches(r Rule, c *ClientInfo) bool {
	p := strings.TrimSpace(r.Pattern)
	if p == "" {
		return false
	}
	switch r.Field {
	case "hostname":
		return wildcardMatch(p, c.Hostname)
	case "user":
		return wildcardMatch(p, c.User) || wildcardMatch(p, c.Domain+`\`+c.User)
	case "ou":
		// OUs are matched against the computer's distinguished name.
		// "Library" matches any DN containing it; wildcards give full control.
		if hasWildcard(p) {
			return wildcardMatch(p, c.OU)
		}
		return strings.Contains(strings.ToLower(c.OU), strings.ToLower(p))
	case "group":
		for _, g := range c.Groups {
			short := g
			if i := strings.LastIndex(g, `\`); i >= 0 {
				short = g[i+1:]
			}
			if wildcardMatch(p, g) || wildcardMatch(p, short) {
				return true
			}
		}
		return false
	case "ip":
		if _, cidr, err := net.ParseCIDR(p); err == nil {
			ip := net.ParseIP(c.IP)
			return ip != nil && cidr.Contains(ip)
		}
		return wildcardMatch(p, c.IP)
	}
	return false
}

func rulesMatch(rules []Rule, c *ClientInfo) bool {
	for _, r := range rules {
		if ruleMatches(r, c) {
			return true
		}
	}
	return false
}

// alertMatches reports whether alert a is meant for client c.
func alertMatches(a *Alert, c *ClientInfo) bool {
	if a.Spec.Target.All {
		return true
	}
	return rulesMatch(a.Rules, c)
}

// flattenTarget resolves saved groups into plain rules and builds the
// human-readable summary stored with the alert. Caller holds the store lock.
func (s *Store) flattenTarget(t Target) (rules []Rule, summary string) {
	if t.All {
		return nil, "Every computer"
	}
	var parts []string
	for _, id := range t.GroupIDs {
		if g := s.groupByID(id); g != nil {
			rules = append(rules, g.Rules...)
			parts = append(parts, g.Name)
		}
	}
	for _, r := range t.Rules {
		rules = append(rules, r)
		parts = append(parts, validFields[r.Field]+" "+r.Pattern)
	}
	summary = strings.Join(parts, ", ")
	if len(summary) > 200 {
		summary = summary[:197] + "..."
	}
	return rules, summary
}

func cleanRules(in []Rule) []Rule {
	out := make([]Rule, 0, len(in))
	for _, r := range in {
		r.Field = strings.ToLower(strings.TrimSpace(r.Field))
		r.Pattern = cleanText(r.Pattern, 256)
		if _, ok := validFields[r.Field]; ok && r.Pattern != "" {
			out = append(out, r)
		}
	}
	return out
}

// cleanText strips control characters (keeping newlines and tabs out too when
// single is true) and caps the length in runes.
func cleanText(s string, max int) string { return cleanTextOpt(s, max, false) }

func cleanTextOpt(s string, max int, keepNewlines bool) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var b strings.Builder
	n := 0
	for _, r := range s {
		if r == '\n' && keepNewlines {
			// allowed
		} else if unicode.IsControl(r) {
			continue
		}
		if n >= max {
			break
		}
		b.WriteRune(r)
		n++
	}
	return strings.TrimSpace(b.String())
}
