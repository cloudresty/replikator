package valueobject

import (
	"fmt"
	"regexp"
	"strings"
)

type SourceID struct {
	namespace string
	name      string
}

func NewSourceID(namespace, name string) SourceID {
	return SourceID{
		namespace: namespace,
		name:      name,
	}
}

func (s SourceID) Namespace() string {
	return s.namespace
}

func (s SourceID) Name() string {
	return s.name
}

func (s SourceID) String() string {
	return fmt.Sprintf("%s/%s", s.namespace, s.name)
}

func (s SourceID) Equals(other SourceID) bool {
	return s.namespace == other.namespace && s.name == other.name
}

type MirrorID struct {
	namespace string
	name      string
}

func NewMirrorID(namespace, name string) MirrorID {
	return MirrorID{
		namespace: namespace,
		name:      name,
	}
}

func (m MirrorID) Namespace() string {
	return m.namespace
}

func (m MirrorID) Name() string {
	return m.name
}

func (m MirrorID) String() string {
	return fmt.Sprintf("%s/%s", m.namespace, m.name)
}

func (m MirrorID) Equals(other MirrorID) bool {
	return m.namespace == other.namespace && m.name == other.name
}

type AllowedNamespaces struct {
	patterns []string
	regexes  []*regexp.Regexp
	isAll    bool
}

func NewAllowedNamespaces(patterns []string) (AllowedNamespaces, error) {
	an := AllowedNamespaces{
		patterns: make([]string, 0, len(patterns)),
		regexes:  make([]*regexp.Regexp, 0, len(patterns)),
	}

	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if pattern == "*" {
			an.isAll = true
			return an, nil
		}

		an.patterns = append(an.patterns, pattern)

		regexStr := strings.ReplaceAll(pattern, ".", "\\.")
		regexStr = strings.ReplaceAll(regexStr, "*", ".*")
		regexStr = strings.ReplaceAll(regexStr, "[", "(")
		regexStr = strings.ReplaceAll(regexStr, "]", ")")

		re, err := regexp.Compile("^" + regexStr + "$")
		if err != nil {
			return AllowedNamespaces{}, fmt.Errorf("invalid namespace pattern %q: %w", pattern, err)
		}
		an.regexes = append(an.regexes, re)
	}

	return an, nil
}

func (an AllowedNamespaces) Matches(namespace string) bool {
	if an.isAll {
		return true
	}
	for _, re := range an.regexes {
		if re.MatchString(namespace) {
			return true
		}
	}
	return false
}

func (an AllowedNamespaces) IsEmpty() bool {
	return len(an.patterns) == 0 && !an.isAll
}

func (an AllowedNamespaces) IsAll() bool {
	return an.isAll
}

func (an AllowedNamespaces) Patterns() []string {
	return an.patterns
}

func ParseAllowedNamespacesAnnotation(value string) ([]string, error) {
	if value == "" || value == "*" {
		return []string{"*"}, nil
	}

	patterns := strings.Split(value, ",")
	result := make([]string, 0, len(patterns))
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result, nil
}
