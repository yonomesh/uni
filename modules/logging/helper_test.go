// Copyright 2015 Matthew Holt and The Caddy Authors
// Copyright 2025 k2 <skrik2@outlook.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logging

import (
	"net/netip"
	"testing"
)

func TestIPv6Mask_64(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"F2BE:8066:CA7E:24D9:1574:CDE0:FCE4:921A", "f2be:8066:ca7e:24d9::"},
		{"59BC:0B00:ACF0:9CFC:18A4:6B90:4CD3:A6B3", "59bc:b00:acf0:9cfc::"},
		{"B90E:4A15:D452:5395:C964:942E:5A55:10B4", "b90e:4a15:d452:5395::"},
		{"8D02:A353:EF64:0D63:A698:AC5D:F93D:C304", "8d02:a353:ef64:d63::"},
		{"0786:5919:BF5A:2912:7B08:945A:E292:1B7C", "786:5919:bf5a:2912::"},
		{"0F4B:C1C7:255E:804B:943B:A645:7D5A:5F47", "f4b:c1c7:255e:804b::"},
		{"7D20:6BB1:006B:2744:DF9B:E3EE:0528:D71B", "7d20:6bb1:6b:2744::"},
		{"A8A9:A683:6FD2:E813:1A54:0EEB:233A:8EF2", "a8a9:a683:6fd2:e813::"},
		{"4742:6E8D:C48E:E4B3:D1B0:890B:A403:9656", "4742:6e8d:c48e:e4b3::"},
		{"8A3F:3A8B:AAA8:D7F0:81B3:AF66:4827:42A6", "8a3f:3a8b:aaa8:d7f0::"},
	}

	prefixLen := 64

	for _, tc := range tests {
		addr, err := netip.ParseAddr(tc.input)
		if err != nil {
			t.Fatalf("Failed to parse address %s: %v", tc.input, err)
		}

		got := ipv6Mask(addr, prefixLen)
		if got != tc.want {
			t.Errorf("ipv6Mask(%s, %d) = %s; want %s", tc.input, prefixLen, got, tc.want)
		}
	}
}

func TestIPv4Mask(t *testing.T) {
	tests := []struct {
		ip   string
		mask int
		want string
	}{
		{"192.168.1.123", 24, "192.168.1.0"},
		{"192.168.1.123", 16, "192.168.0.0"},
		{"10.123.45.67", 8, "10.0.0.0"},
		{"172.16.5.4", 12, "172.16.0.0"},
	}

	for _, tt := range tests {
		addr := netip.MustParseAddr(tt.ip)
		got := ipv4Mask(addr, tt.mask)
		if got != tt.want {
			t.Fatalf("ipv4Mask(%s/%d) = %s, want %s", tt.ip, tt.mask, got, tt.want)
		}
	}
}
