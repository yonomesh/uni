package uni

import (
	"reflect"
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
			name: "duplicate registration",
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
