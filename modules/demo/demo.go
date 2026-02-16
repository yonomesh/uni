package demo

import "github.com/yonomesh/uni"

func init() {
	uni.RegisterModule(Demo{})
}

type Demo struct {
	name string
}

func (Demo) UniModule() uni.ModuleInfo {
	return uni.ModuleInfo{
		ID:  "uni.demo",
		New: func() uni.Module { return new(Demo) },
	}
}
