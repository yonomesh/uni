// Copyright 2015 Matthew Holt and The Caddy Authors
// Copyright 2025 K2 <skrik2@outlook.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");

package internal

import (
	"strconv"
	"testing"
)

func genMockSubjects(size int) map[string]struct{} {
	m := make(map[string]struct{}, size)
	for i := range size {
		// 模拟真实的域名字符串长度
		key := "domain-" + strconv.Itoa(i) + "-long-suffix.com"
		m[key] = struct{}{}
	}
	return m
}

func BenchmarkMaxSizeSubjectsList(b *testing.B) {
	subjects := genMockSubjects(100000)
	maxToDisplay := 20

	for b.Loop() {
		_ = MaxSizeSubjectsListForLog(subjects, maxToDisplay)
	}
}
