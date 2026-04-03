package unicmd

import (
	"testing"

	"github.com/spf13/pflag"
)

// func (f Flags) String(name string) string
func TestFlagsString(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("config", "default.json", "path to a config file")
	fs.Parse([]string{"--config=myconfig.json"})
	flags := Flags{fs}
	val := flags.String("config")
	expected := "myconfig.json"
	if val != expected {
		t.Errorf("expected %q but got %q", expected, val)
	}
}
