package uni

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

type testMod struct {
	info ModuleInfo
}

func (m testMod) UniModule() ModuleInfo {
	return m.info
}

func TestModuleInfo_String(t *testing.T) {
	tests := []struct {
		name string
		mi   ModuleInfo
		want string
	}{
		{
			name: "normal ID",
			mi:   ModuleInfo{ID: "a.b.c"},
			want: "a.b.c",
		},
		{
			name: "single segment",
			mi:   ModuleInfo{ID: "foo"},
			want: "foo",
		},
		{
			name: "empty ID",
			mi:   ModuleInfo{ID: ""},
			want: "",
		},
		{
			name: "unicode ID",
			mi:   ModuleInfo{ID: "模块.名字"},
			want: "模块.名字",
		},
		{
			name: "ID with trailing dot",
			mi:   ModuleInfo{ID: "a.b."},
			want: "a.b.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.mi.String()
			if got != tt.want {
				t.Fatalf("ModuleInfo.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestModuleID_Namespace(t *testing.T) {
	tests := []struct {
		name string
		id   ModuleID
		want string
	}{
		{
			name: "empty string",
			id:   "",
			want: "",
		},
		{
			name: "no namespace single label",
			id:   "endpoint",
			want: "",
		},
		{
			name: "simple namespace",
			id:   "endpoint.socks",
			want: "endpoint",
		},
		{
			name: "multi level namespace",
			id:   "logging.encoders.json",
			want: "logging.encoders",
		},
		{
			name: "two dots",
			id:   "a.b.c",
			want: "a.b",
		},
		{
			name: "trailing dot",
			id:   "a.b.",
			want: "a.b",
		},
		{
			name: "leading dot",
			id:   ".hidden",
			want: "",
		},
		{
			name: "only dot",
			id:   ".",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.id.Namespace()
			if got != tt.want {
				t.Fatalf("Namespace() = %q, want %q (id=%q)", got, tt.want, tt.id)
			}
		})
	}
}

func TestModuleID_Name(t *testing.T) {
	tests := []struct {
		name string
		id   ModuleID
		want string
	}{
		{
			name: "empty",
			id:   ModuleID(""),
			want: "",
		},
		{
			name: "single segment",
			id:   ModuleID("module"),
			want: "module",
		},
		{
			name: "two segments",
			id:   ModuleID("a.b"),
			want: "b",
		},
		{
			name: "multiple segments",
			id:   ModuleID("a.b.c.d"),
			want: "d",
		},
		{
			name: "leading dot",
			id:   ModuleID(".a"),
			want: "a",
		},
		{
			name: "trailing dot",
			id:   ModuleID("a."),
			want: "",
		},
		{
			name: "multiple dots tail",
			id:   ModuleID("a.b."),
			want: "",
		},
		{
			name: "only dot",
			id:   ModuleID("."),
			want: "",
		},
		{
			name: "double dot",
			id:   ModuleID("a..b"),
			want: "b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.id.Name()
			if got != tt.want {
				t.Fatalf("Name() = %q, want %q (id=%q)", got, tt.want, tt.id)
			}
		})
	}
}

// func RegisterModule(instance Module)
func TestRegisterModule(t *testing.T) {
	modulesMu.Lock()
	modules = map[string]ModuleInfo{
		"a.b.c": {ID: "a.b.c"},
	}
	modulesMu.Unlock()

	tests := []struct {
		name             string
		mod              testMod
		shouldPanic      bool
		panicMsgContains string
	}{
		{
			name: "normal registration",
			mod: testMod{
				info: ModuleInfo{
					ID:  "foo",
					New: func() Module { return testMod{} },
				},
			},
			shouldPanic: false,
		},
		{
			name: "empty ID",
			mod: testMod{
				info: ModuleInfo{
					ID:  "",
					New: func() Module { return testMod{} },
				},
			},
			shouldPanic:      true,
			panicMsgContains: "module ID missing",
		},
		{
			name: "reserved ID uni",
			mod: testMod{
				info: ModuleInfo{
					ID:  "uni",
					New: func() Module { return testMod{} },
				},
			},
			shouldPanic:      true,
			panicMsgContains: "reserved",
		},
		{
			name: "New is nil",
			mod: testMod{
				info: ModuleInfo{
					ID:  "nilmod",
					New: nil,
				},
			},
			shouldPanic:      true,
			panicMsgContains: "ModuleInfo.New",
		},
		{
			name: "New returns nil",
			mod: testMod{
				info: ModuleInfo{
					ID:  "nilmod",
					New: func() Module { return nil },
				},
			},
			shouldPanic:      true,
			panicMsgContains: "must return a non-nil",
		},
		{
			name: "duplicate registration",
			mod: testMod{
				info: ModuleInfo{
					ID:  "a.b.c", // 已经预先注册
					New: func() Module { return testMod{} },
				},
			},
			shouldPanic:      true,
			panicMsgContains: "already registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.shouldPanic {
					if r == nil {
						t.Fatalf("expected panic but did not get one")
					} else if msg, ok := r.(string); ok && !strings.Contains(msg, tt.panicMsgContains) {
						t.Fatalf("panic message = %q, want contains %q", msg, tt.panicMsgContains)
					}
				} else {
					if r != nil {
						t.Fatalf("unexpected panic: %v", r)
					}
				}
			}()

			RegisterModule(tt.mod)
		})
	}
}

// func GetModule(name string) (ModuleInfo, error)
func TestGetModule(t *testing.T) {
	modulesMu.Lock()
	modules = map[string]ModuleInfo{
		"a":      {ID: "a"},
		"a.b":    {ID: "a.b"},
		"a.b.c":  {ID: "a.b.c"},
		"a.b.cd": {ID: "a.b.cd"},
		"a.c":    {ID: "a.c"},
		"a.d":    {ID: "a.d"},
		"b":      {ID: "b"},
		"b.a":    {ID: "b.a"},
		"b.b":    {ID: "b.b"},
		"b.a.c":  {ID: "b.a.c"},
		"c":      {ID: "c"},
	}
	modulesMu.Unlock()

	tests := []struct {
		name        string
		moduleName  string
		wantID      string
		expectError bool
	}{
		{
			name:        "get no existing module foo",
			moduleName:  "foo",
			wantID:      "foo",
			expectError: true,
		},
		{
			name:        "get existing module bar",
			moduleName:  "b.a",
			wantID:      "b.a",
			expectError: false,
		},
		{
			name:        "get none",
			moduleName:  "",
			wantID:      "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetModule(tt.moduleName)

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if string(got.ID) != tt.wantID {
				t.Fatalf("ID mismatch: got=%s want=%s", got.ID, tt.wantID)
			}
		})
	}

}

// func GetModules(scope string) []ModuleInfo
func TestGetModules(t *testing.T) {
	modulesMu.Lock()
	modules = map[string]ModuleInfo{
		"a":      {ID: "a"},
		"a.b":    {ID: "a.b"},
		"a.b.c":  {ID: "a.b.c"},
		"a.b.cd": {ID: "a.b.cd"},
		"a.c":    {ID: "a.c"},
		"a.d":    {ID: "a.d"},
		"b":      {ID: "b"},
		"b.a":    {ID: "b.a"},
		"b.b":    {ID: "b.b"},
		"b.a.c":  {ID: "b.a.c"},
		"c":      {ID: "c"},
	}
	modulesMu.Unlock()

	for i, tc := range []struct {
		input  string
		expect []ModuleInfo
	}{
		{
			input: "",
			expect: []ModuleInfo{
				{ID: "a"},
				{ID: "b"},
				{ID: "c"},
			},
		},
		{
			input: "a",
			expect: []ModuleInfo{
				{ID: "a.b"},
				{ID: "a.c"},
				{ID: "a.d"},
			},
		},
		{
			input: "a.b",
			expect: []ModuleInfo{
				{ID: "a.b.c"},
				{ID: "a.b.cd"},
			},
		},
		{
			input: "a.b.c",
		},
		{
			input: "b",
			expect: []ModuleInfo{
				{ID: "b.a"},
				{ID: "b.b"},
			},
		},
		{
			input: "asdf",
		},
	} {
		actual := GetModules(tc.input)
		if !reflect.DeepEqual(actual, tc.expect) {
			t.Errorf("Test %d: Expected %v but got %v", i, tc.expect, actual)
		}
	}
}

// func GetModuleName(instance any) string
func TestGetModuleName(t *testing.T) {
	tests := []struct {
		name     string
		mod      testMod
		wantName string
	}{
		{
			name: "normal registration",
			mod: testMod{
				info: ModuleInfo{
					ID:  "foo",
					New: func() Module { return testMod{} },
				},
			},
			wantName: "foo",
		},
		{
			name: "empty ID",
			mod: testMod{
				info: ModuleInfo{
					ID:  "",
					New: func() Module { return testMod{} },
				},
			},
			wantName: "",
		},
		{
			name: "normal",
			mod: testMod{
				info: ModuleInfo{
					ID:  "a.b.c",
					New: func() Module { return testMod{} },
				},
			},
			wantName: "c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetModuleName(tt.mod)
			if got != tt.wantName {
				t.Fatalf("want %s but got %s", tt.wantName, got)
			}
		})
	}
}

// func GetModuleID(instance any) string
func TestGetModuleID(t *testing.T) {
	tests := []struct {
		name   string
		mod    testMod
		wantID string
	}{
		{
			name: "normal ID",
			mod: testMod{
				info: ModuleInfo{
					ID:  "foo",
					New: func() Module { return testMod{} },
				},
			},
			wantID: "foo",
		},
		{
			name: "empty ID",
			mod: testMod{
				info: ModuleInfo{
					ID:  "",
					New: func() Module { return testMod{} },
				},
			},
			wantID: "",
		},
		{
			name: "normal ID",
			mod: testMod{
				info: ModuleInfo{
					ID:  "a.b.c",
					New: func() Module { return testMod{} },
				},
			},
			wantID: "a.b.c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetModuleID(tt.mod)
			if got != tt.wantID {
				t.Fatalf("want %s but got %s", tt.wantID, got)
			}
		})
	}
}

// func Modules() []string
func TestModules(t *testing.T) {
	clear(modules)
	modulesMu.Lock()
	modules = map[string]ModuleInfo{
		"a":      {ID: "a"},
		"a.b":    {ID: "a.b"},
		"a.b.c":  {ID: "a.b.c"},
		"a.b.cd": {ID: "a.b.cd"},
		"a.c":    {ID: "a.c"},
		"a.d":    {ID: "a.d"},
		"b":      {ID: "b"},
		"b.a":    {ID: "b.a"},
		"b.b":    {ID: "b.b"},
		"b.a.c":  {ID: "b.a.c"},
		"c":      {ID: "c"},
	}
	modulesMu.Unlock()

	want := []string{"a", "a.b", "a.b.c", "a.b.cd", "a.c", "a.d", "b", "b.a", "b.a.c", "b.b", "c"}

	got := Modules()
	fmt.Println(got)
	if !slices.Equal(want, got) {
		t.Fatalf("no")
	}
}

// func getModuleNameInline(moduleNameKey string, raw json.RawMessage) (string, json.RawMessage, error)
func TestGetModuleNameInline(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		raw      string
		wantName string
		wantJSON string
		wantErr  bool
	}{
		{
			name:     "normal case",
			key:      "handler",
			raw:      `{"handler":"http","timeout":5}`,
			wantName: "http",
			wantJSON: `{"timeout":5}`,
			wantErr:  false,
		},
		{
			name:    "missing key",
			key:     "handler",
			raw:     `{"timeout":5}`,
			wantErr: true,
		},
		{
			name:    "key not string",
			key:     "handler",
			raw:     `{"handler":123}`,
			wantErr: true,
		},
		{
			name:    "empty string module name",
			key:     "handler",
			raw:     `{"handler":""}`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			key:     "handler",
			raw:     `{"handler":`,
			wantErr: true,
		},
		{
			name:     "only module key",
			key:      "handler",
			raw:      `{"handler":"http"}`,
			wantName: "http",
			wantJSON: `{}`,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, out, err := getModuleNameInline(tt.key, []byte(tt.raw))

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if name != tt.wantName {
				t.Fatalf("want name %q but got %q", tt.wantName, name)
			}

			var gotMap map[string]any
			var wantMap map[string]any

			if err := json.Unmarshal(out, &gotMap); err != nil {
				t.Fatalf("unmarshal output json failed: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.wantJSON), &wantMap); err != nil {
				t.Fatalf("unmarshal want json failed: %v", err)
			}

			if !reflect.DeepEqual(gotMap, wantMap) {
				t.Fatalf("want json %v but got %v", wantMap, gotMap)
			}
		})
	}
}

// func ParseStructTag(tag string) (map[string]string, error)
func TestParseStructTag(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
		wantErr  bool
	}{
		{
			name:  "two k-v",
			input: "key1=val1 key2=val2",
			expected: map[string]string{
				"key1": "val1",
				"key2": "val2",
			},
			wantErr: false,
		},
		{
			name:  "multi-space",
			input: "  key1=val1   key2=val2  ",
			expected: map[string]string{
				"key1": "val1",
				"key2": "val2",
			},
			wantErr: false,
		},
		{
			name:  "single k-v",
			input: "mode=fast",
			expected: map[string]string{
				"mode": "fast",
			},
			wantErr: false,
		},
		{
			name:     "missing equals sign",
			input:    "key1=val1 invalidkey",
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "none",
			input:    "",
			expected: map[string]string{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStructTag(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStructTag() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ParseStructTag() got = %v, want %v", got, tt.expected)
			}
		})
	}
}

// func StrictUnmarshalJSON(data []byte, v any) error
func TestStrictUnmarshalJSON(t *testing.T) {
	type Config struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	tests := []struct {
		name    string
		data    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "normal",
			data:    `{"name": "kaze", "count": 10}`,
			wantErr: false,
		},
		{
			name:    "包含未知字段 (严格模式应报错)",
			data:    `{"name": "kaze", "unknown": "value"}`,
			wantErr: true,
			errMsg:  "unknown field",
		},
		{
			name:    "JSON syntax error",
			data:    `{"name": "kaze" "count": 10}`, // 缺少逗号
			wantErr: true,
			errMsg:  "at offset",
		},
		{
			name:    "JSON Type mismatch",
			data:    `{"name": "kaze", "count": "not-an-int"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg Config
			err := StrictUnmarshalJSON([]byte(tt.data), &cfg)

			if (err != nil) != tt.wantErr {
				t.Errorf("StrictUnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Error message '%v' does not contain '%v'", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

// func isJSONRawMessage(typ reflect.Type) bool
func TestIsJSONRawMessage(t *testing.T) {
	type CustomStruct struct{}

	tests := []struct {
		name     string
		input    reflect.Type
		expected bool
	}{
		{
			name:     "json.RawMessage",
			input:    reflect.TypeFor[json.RawMessage](),
			expected: true,
		},
		{
			name:     "[]byte",
			input:    reflect.TypeFor[[]byte](),
			expected: false,
		},
		{
			name:     "string",
			input:    reflect.TypeFor[string](),
			expected: false,
		},
		{
			name:     "struct",
			input:    reflect.TypeFor[CustomStruct](),
			expected: false,
		},
		{
			name:     "nil",
			input:    nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isJSONRawMessage(tt.input); got != tt.expected {
				t.Errorf("isJSONRawMessage() = %v, want %v", got, tt.expected)
			}
		})
	}
}
