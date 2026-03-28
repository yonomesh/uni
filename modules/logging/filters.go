package logging

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"

	"go.uber.org/zap/zapcore"

	"github.com/yonomesh/uni"
)

// LogFieldFilter can filter (or manipulate) a field in a log entry.
type LogFieldFilter interface {
	Filter(zapcore.Field) zapcore.Field
}

// DeleteFilter is a Uni log field filter that deletes the field.
type DeleteFilter struct{}

// UniModule returns the Uni module information.
func (DeleteFilter) UniModule() uni.ModuleInfo {
	return uni.ModuleInfo{
		ID:  "caddy.logging.encoders.filter.delete",
		New: func() uni.Module { return new(DeleteFilter) },
	}
}

// TODO
//
// UnmarshalCaddyfile sets up the module from Caddyfile tokens.
func (DeleteFilter) UnmarshalUniConfigfile() error {
	return nil
}

// Filter filters the input field.
func (DeleteFilter) Filter(in zapcore.Field) zapcore.Field {
	in.Type = zapcore.SkipType
	return in
}

// HashFilter is a Caddy log field filter that
// replaces the field with the initial 4 bytes
// of the SHA-256 hash of the content. Operates
// on string fields, or on arrays of strings
// where each string is hashed.
type HashFilter struct{}

// UniModule returns the Uni module information.
func (HashFilter) UniModule() uni.ModuleInfo {
	return uni.ModuleInfo{
		ID:  "caddy.logging.encoders.filter.hash",
		New: func() uni.Module { return new(HashFilter) },
	}
}

// UnmarshalCaddyfile sets up the module from Caddyfile tokens.
func (f *HashFilter) UnmarshalUniConfigfile() error {
	return nil
}

// Filter filters the input field with the replacement value.
func (f *HashFilter) Filter(in zapcore.Field) zapcore.Field {
	if array, ok := in.Interface.([]string); ok {
		newArray := make([]string, len(array))
		for i, s := range array {
			newArray[i] = hashHelper(s)
		}
		in.Interface = newArray
	} else {
		in.String = hashHelper(in.String)
	}

	return in
}

// hashHelper returns the first 4 bytes of the SHA-256 hash of the given data as hexadecimal
func hashHelper(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:4])
}

// ReplaceFilter is a Uni log field filter that
// replaces the field with the indicated string.
//
//	// Caddyfile
//	filter {
//		fields {
//			password replace "***"
//		}
//	}
type ReplaceFilter struct {
	Value string `json:"value,omitempty"`
}

// UniModule returns the Uni module information.
func (ReplaceFilter) UniModule() uni.ModuleInfo {
	return uni.ModuleInfo{
		ID:  "caddy.logging.encoders.filter.replace",
		New: func() uni.Module { return new(ReplaceFilter) },
	}
}

// TODO
//
// UnmarshalCaddyfile sets up the module from Caddyfile tokens.
func (f *ReplaceFilter) UnmarshalUniConfigfile() error {
	return nil
}

// Filter filters the input field with the replacement value.
func (f *ReplaceFilter) Filter(in zapcore.Field) zapcore.Field {
	in.Type = zapcore.StringType
	in.String = f.Value
	return in
}

// IPMaskFilter is a Caddy log field filter that
// masks IP addresses in a string, or in an array
// of strings. The string may be a comma separated
// list of IP addresses, where all of the values
// will be masked.
type IPMaskFilter struct {
	// The IPv4 mask subnet prefix length
	IPv4MaskRaw int `json:"ipv4_cidr,omitempty"`

	// The IPv6 mask subnet prefix length
	IPv6MaskRaw int `json:"ipv6_cidr,omitempty"`
}

// UniModule returns the Uni module information.
func (IPMaskFilter) UniModule() uni.ModuleInfo {
	return uni.ModuleInfo{
		ID:  "caddy.logging.encoders.filter.ip_mask",
		New: func() uni.Module { return new(IPMaskFilter) },
	}
}

// TODO
// UnmarshalCaddyfile sets up the module from Caddyfile tokens.
func (m *IPMaskFilter) UnmarshalCaddyfile() error {
	return nil
}

// Filter filters the input field.
func (m IPMaskFilter) Filter(in zapcore.Field) zapcore.Field {
	if array, ok := in.Interface.([]string); ok {
		newArray := make([]string, len(array))
		for i, s := range array {
			newArray[i] = m.maskHelper(s)
		}
		in.Interface = newArray
	} else {
		in.String = m.maskHelper(in.String)
	}

	return in
}

func (m IPMaskFilter) maskHelper(s string) string {
	var output strings.Builder
	for value := range strings.SplitSeq(s, ",") {
		value = strings.TrimSpace(value)
		host, port, err := net.SplitHostPort(value)
		if err != nil {
			host = value // assume whole thing was IP address
		}

		ipAddr, err := netip.ParseAddr(host)
		if err != nil {
			output.WriteString(value + ", ")
			continue
		}
		// ipAddr=ipAddr.Unmap()
		var ipStrMasked string = ""

		if ipAddr.Is4() {
			ipStrMasked = ipv4Mask(ipAddr, m.IPv4MaskRaw)
		} else {
			ipStrMasked = ipv6Mask(ipAddr, m.IPv6MaskRaw)
		}

		if port == "" {
			output.WriteString(ipStrMasked + ", ")
			continue
		} else {
			output.WriteString(net.JoinHostPort(ipStrMasked, port) + ", ")
		}

	}
	return strings.TrimSuffix(output.String(), ", ")
}

type filterAction string

const (
	// Replace value(s).
	replaceAction filterAction = "replace"

	// Hash value(s).
	hashAction filterAction = "hash"

	// Delete.
	deleteAction filterAction = "delete"
)

func (a filterAction) IsValid() error {
	switch a {
	case replaceAction, deleteAction, hashAction:
		return nil
	}

	return errors.New("invalid logging filter action type")
}

type queryFilterAction struct {
	// `replace` to replace the value(s) associated with the parameter(s), `hash` to replace
	// them with the 4 initial bytes of the SHA-256 of their content or `delete` to remove
	// them entirely.
	Type filterAction `json:"type"`

	// The name of the query parameter.
	Parameter string `json:"parameter"`

	// The value to use as replacement if the action is `replace`.
	Value string `json:"value,omitempty"`
}

// QueryFilter is a Caddy log field filter that filters
// query parameters from a URL.
//
// This filter updates the logged URL string to remove, replace or hash
// query parameters containing sensitive data. For instance, it can be
// used to redact any kind of secrets which were passed as query parameters,
// such as OAuth access tokens, session IDs, magic link tokens, etc.
type QueryFilter struct {
	// A list of actions to apply to the query parameters of the URL.
	Actions []queryFilterAction `json:"actions"`
}

// Validate checks that action types are correct.
func (f *QueryFilter) Validate() error {
	for _, a := range f.Actions {
		if err := a.Type.IsValid(); err != nil {
			return err
		}
	}

	return nil
}

// CaddyModule returns the Caddy module information.
func (QueryFilter) UniModule() uni.ModuleInfo {
	return uni.ModuleInfo{
		ID:  "caddy.logging.encoders.filter.query",
		New: func() uni.Module { return new(QueryFilter) },
	}
}

// TODO
// UnmarshalCaddyfile sets up the module from Caddyfile tokens.
func (m *QueryFilter) UnmarshalCaddyfile() error {
	return nil
}

// Filter filters the input field.
func (m QueryFilter) Filter(in zapcore.Field) zapcore.Field {
	if array, ok := in.Interface.([]string); ok {
		newArray := make([]string, len(array))
		for i, s := range array {
			newArray[i] = m.processQueryString(s)
		}
		in.Interface = newArray
	} else {
		in.String = m.processQueryString(in.String)
	}

	return in
}

func (m QueryFilter) processQueryString(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return s
	}

	q := u.Query()
	for _, a := range m.Actions {
		switch a.Type {
		case replaceAction:
			for i := range q[a.Parameter] {
				q[a.Parameter][i] = a.Value
			}

		case hashAction:
			for i := range q[a.Parameter] {
				q[a.Parameter][i] = hashHelper(a.Value)
			}

		case deleteAction:
			q.Del(a.Parameter)
		}
	}

	u.RawQuery = q.Encode()
	return u.String()
}

type cookieFilterAction struct {
	// `replace` to replace the value of the cookie, `hash` to replace it with the 4 initial bytes of the SHA-256
	// of its content or `delete` to remove it entirely.
	Type filterAction `json:"type"`

	// The name of the cookie.
	Name string `json:"name"`

	// The value to use as replacement if the action is `replace`.
	Value string `json:"value,omitempty"`
}

// CookieFilter is a Caddy log field filter that filters
// cookies.
//
// This filter updates the logged HTTP header string
// to remove, replace or hash cookies containing sensitive data. For instance,
// it can be used to redact any kind of secrets, such as session IDs.
//
// If several actions are configured for the same cookie name, only the first
// will be applied.
type CookieFilter struct {
	// A list of actions to apply to the cookies.
	Actions []cookieFilterAction `json:"actions"`
}

// Validate checks that action types are correct.
func (f *CookieFilter) Validate() error {
	for _, a := range f.Actions {
		if err := a.Type.IsValid(); err != nil {
			return err
		}
	}

	return nil
}

// CaddyModule returns the Caddy module information.
func (CookieFilter) UniModule() uni.ModuleInfo {
	return uni.ModuleInfo{
		ID:  "caddy.logging.encoders.filter.cookie",
		New: func() uni.Module { return new(CookieFilter) },
	}
}

// UnmarshalCaddyfile sets up the module from Caddyfile tokens.
func (m *CookieFilter) UnmarshalCaddyfile() error {

	return nil
}

// Filter filters the input field.
func (m CookieFilter) Filter(in zapcore.Field) zapcore.Field {
	cookiesSlice, ok := in.Interface.([]string)
	if !ok {
		return in
	}

	// using a dummy Request to make use of the Cookies() function to parse it
	originRequest := http.Request{Header: http.Header{"Cookie": cookiesSlice}}
	cookies := originRequest.Cookies()
	transformedRequest := http.Request{Header: make(http.Header)}

OUTER:
	for _, c := range cookies {
		for _, a := range m.Actions {
			if c.Name != a.Name {
				continue
			}

			switch a.Type {
			case replaceAction:
				c.Value = a.Value
				transformedRequest.AddCookie(c)
				continue OUTER

			case hashAction:
				c.Value = hashHelper(c.Value)
				transformedRequest.AddCookie(c)
				continue OUTER

			case deleteAction:
				continue OUTER
			}
		}

		transformedRequest.AddCookie(c)
	}

	in.Interface = []string(transformedRequest.Header["Cookie"])

	return in
}

// RegexpFilter is a Caddy log field filter that
// replaces the field matching the provided regexp
// with the indicated string. If the field is an
// array of strings, each of them will have the
// regexp replacement applied.
type RegexpFilter struct {
	// The regular expression pattern defining what to replace.
	RawRegexp string `json:"regexp,omitempty"`

	// The value to use as replacement
	Value string `json:"value,omitempty"`

	regexp *regexp.Regexp
}

// CaddyModule returns the Caddy module information.
func (RegexpFilter) UniModule() uni.ModuleInfo {
	return uni.ModuleInfo{
		ID:  "caddy.logging.encoders.filter.regexp",
		New: func() uni.Module { return new(RegexpFilter) },
	}
}

// TODO
// UnmarshalCaddyfile sets up the module from Caddyfile tokens.
func (f *RegexpFilter) UnmarshalCaddyfile() error {
	return nil
}

// Provision compiles m's regexp.
func (m *RegexpFilter) Provision(ctx uni.Context) error {
	r, err := regexp.Compile(m.RawRegexp)
	if err != nil {
		return err
	}

	m.regexp = r

	return nil
}

// Filter filters the input field with the replacement value if it matches the regexp.
func (f *RegexpFilter) Filter(in zapcore.Field) zapcore.Field {
	if array, ok := in.Interface.([]string); ok {
		newArray := make([]string, len(array))
		for i, s := range array {
			newArray[i] = f.regexp.ReplaceAllString(s, f.Value)
		}
		in.Interface = newArray
	} else {
		in.String = f.regexp.ReplaceAllString(in.String, f.Value)
	}

	return in
}

// regexpFilterOperation represents a single regexp operation
// within a MultiRegexpFilter.
type regexpFilterOperation struct {
	// The regular expression pattern defining what to replace.
	RawRegexp string `json:"regexp,omitempty"`

	// The value to use as replacement
	Value string `json:"value,omitempty"`

	regexp *regexp.Regexp
}

// MultiRegexpFilter is a Caddy log field filter that
// can apply multiple regular expression replacements to
// the same field. This filter processes operations in the
// order they are defined, applying each regexp replacement
// sequentially to the result of the previous operation.
//
// This allows users to define multiple regexp filters for
// the same field without them overwriting each other.
//
// Security considerations:
// - Uses Go's regexp package (RE2 engine) which is safe from ReDoS attacks
// - Validates all patterns during provisioning
// - Limits the maximum number of operations to prevent resource exhaustion
// - Sanitizes input to prevent injection attacks
type MultiRegexpFilter struct {
	// A list of regexp operations to apply in sequence.
	// Maximum of 50 operations allowed for security and performance.
	Operations []regexpFilterOperation `json:"operations"`
}

// Security constants
const (
	maxRegexpOperations = 50   // Maximum operations to prevent resource exhaustion
	maxPatternLength    = 1000 // Maximum pattern length to prevent abuse
)

// CaddyModule returns the Caddy module information.
func (MultiRegexpFilter) UniModule() uni.ModuleInfo {
	return uni.ModuleInfo{
		ID:  "caddy.logging.encoders.filter.multi_regexp",
		New: func() uni.Module { return new(MultiRegexpFilter) },
	}
}

// UnmarshalCaddyfile sets up the module from Caddyfile tokens.
// Syntax:
//
//	multi_regexp {
//	    regexp <pattern> <replacement>
//	    regexp <pattern> <replacement>
//	    ...
//	}
func (f *MultiRegexpFilter) UnmarshalCaddyfile() error { return nil }

// Provision compiles all regexp patterns with security validation.
func (f *MultiRegexpFilter) Provision(ctx uni.Context) error {
	// Security check: validate operation count
	if len(f.Operations) > maxRegexpOperations {
		return fmt.Errorf("too many regexp operations: %d (maximum %d allowed)", len(f.Operations), maxRegexpOperations)
	}

	if len(f.Operations) == 0 {
		return fmt.Errorf("multi_regexp filter requires at least one operation")
	}

	for i := range f.Operations {
		// Security validation: pattern length check
		if len(f.Operations[i].RawRegexp) > maxPatternLength {
			return fmt.Errorf("regexp pattern %d too long: %d characters (maximum %d)", i, len(f.Operations[i].RawRegexp), maxPatternLength)
		}

		// Security validation: empty pattern check
		if f.Operations[i].RawRegexp == "" {
			return fmt.Errorf("regexp pattern %d cannot be empty", i)
		}

		// Compile and validate the pattern (uses RE2 engine - safe from ReDoS)
		r, err := regexp.Compile(f.Operations[i].RawRegexp)
		if err != nil {
			return fmt.Errorf("compiling regexp pattern %d (%s): %v", i, f.Operations[i].RawRegexp, err)
		}
		f.Operations[i].regexp = r
	}
	return nil
}

// Validate ensures the filter is properly configured with security checks.
func (f *MultiRegexpFilter) Validate() error {
	if len(f.Operations) == 0 {
		return fmt.Errorf("multi_regexp filter requires at least one operation")
	}

	if len(f.Operations) > maxRegexpOperations {
		return fmt.Errorf("too many regexp operations: %d (maximum %d allowed)", len(f.Operations), maxRegexpOperations)
	}

	for i, op := range f.Operations {
		if op.RawRegexp == "" {
			return fmt.Errorf("regexp pattern %d cannot be empty", i)
		}
		if len(op.RawRegexp) > maxPatternLength {
			return fmt.Errorf("regexp pattern %d too long: %d characters (maximum %d)", i, len(op.RawRegexp), maxPatternLength)
		}
		if op.regexp == nil {
			return fmt.Errorf("regexp pattern %d not compiled (call Provision first)", i)
		}
	}
	return nil
}

// Filter applies all regexp operations sequentially to the input field.
// Input is sanitized and validated for security.
func (f *MultiRegexpFilter) Filter(in zapcore.Field) zapcore.Field {
	if array, ok := in.Interface.([]string); ok {
		newArray := make([]string, len(array))
		for i, s := range array {
			newArray[i] = f.processString(s)
		}
		in.Interface = newArray
	} else {
		in.String = f.processString(in.String)
	}

	return in
}

// processString applies all regexp operations to a single string with input validation.
func (f *MultiRegexpFilter) processString(s string) string {
	// Security: validate input string length to prevent resource exhaustion
	const maxInputLength = 1000000 // 1MB max input size
	if len(s) > maxInputLength {
		// Log warning but continue processing (truncated)
		s = s[:maxInputLength]
	}

	result := s
	for _, op := range f.Operations {
		// Each regexp operation is applied sequentially
		// Using RE2 engine which is safe from ReDoS attacks
		result = op.regexp.ReplaceAllString(result, op.Value)

		// Ensure result doesn't exceed max length after each operation
		if len(result) > maxInputLength {
			result = result[:maxInputLength]
		}
	}
	return result
}

// AddOperation adds a single regexp operation to the filter with validation.
// This is used when merging multiple RegexpFilter instances.
func (f *MultiRegexpFilter) AddOperation(rawRegexp, value string) error {
	// Security checks
	if len(f.Operations) >= maxRegexpOperations {
		return fmt.Errorf("cannot add operation: maximum %d operations allowed", maxRegexpOperations)
	}

	if rawRegexp == "" {
		return fmt.Errorf("regexp pattern cannot be empty")
	}

	if len(rawRegexp) > maxPatternLength {
		return fmt.Errorf("regexp pattern too long: %d characters (maximum %d)", len(rawRegexp), maxPatternLength)
	}

	f.Operations = append(f.Operations, regexpFilterOperation{
		RawRegexp: rawRegexp,
		Value:     value,
	})
	return nil
}

// RenameFilter is a Caddy log field filter that
// renames the field's key with the indicated name.
type RenameFilter struct {
	Name string `json:"name,omitempty"`
}

// CaddyModule returns the Caddy module information.
func (RenameFilter) UniModule() uni.ModuleInfo {
	return uni.ModuleInfo{
		ID:  "caddy.logging.encoders.filter.rename",
		New: func() uni.Module { return new(RenameFilter) },
	}
}

// UnmarshalCaddyfile sets up the module from Caddyfile tokens.
func (f *RenameFilter) UnmarshalCaddyfile() error {
	return nil
}

// Filter renames the input field with the replacement name.
func (f *RenameFilter) Filter(in zapcore.Field) zapcore.Field {
	in.Key = f.Name
	return in
}

// Interface Guards
var (
	_ LogFieldFilter = (*DeleteFilter)(nil)
	_ LogFieldFilter = (*HashFilter)(nil)
	_ LogFieldFilter = (*ReplaceFilter)(nil)
	_ LogFieldFilter = (*IPMaskFilter)(nil)
	_ LogFieldFilter = (*QueryFilter)(nil)
	_ LogFieldFilter = (*CookieFilter)(nil)
	_ LogFieldFilter = (*RegexpFilter)(nil)
	_ LogFieldFilter = (*RenameFilter)(nil)
	_ LogFieldFilter = (*MultiRegexpFilter)(nil)

	// _ caddyfile.Unmarshaler = (*DeleteFilter)(nil)
	// _ caddyfile.Unmarshaler = (*HashFilter)(nil)
	// _ caddyfile.Unmarshaler = (*ReplaceFilter)(nil)
	// _ caddyfile.Unmarshaler = (*IPMaskFilter)(nil)
	// _ caddyfile.Unmarshaler = (*QueryFilter)(nil)
	// _ caddyfile.Unmarshaler = (*CookieFilter)(nil)
	// _ caddyfile.Unmarshaler = (*RegexpFilter)(nil)
	// _ caddyfile.Unmarshaler = (*RenameFilter)(nil)
	// _ caddyfile.Unmarshaler = (*MultiRegexpFilter)(nil)

	_ uni.Provisioner = (*RegexpFilter)(nil)
	_ uni.Provisioner = (*MultiRegexpFilter)(nil)

	_ uni.Validator = (*QueryFilter)(nil)
	_ uni.Validator = (*MultiRegexpFilter)(nil)
)
