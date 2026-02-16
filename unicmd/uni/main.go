package main

import (
	"github.com/yonomesh/uni/unicmd"

	// plug in Uni modules here
	_ "github.com/yonomesh/uni/modules/standard"
)

func main() {
	unicmd.Main()
}
