package logging

import (
	"encoding/binary"
	"net/netip"
)

var (
	v4Masks [33][4]byte
	v6Masks [129][2]uint64
)

func init() {
	// v4Masks[33][4]
	for i := 0; i <= 32; i++ {
		mask := uint32(0)
		if i > 0 {
			mask = 0xFFFFFFFF << (32 - i)
		}
		v4Masks[i] = [4]byte{
			byte(mask >> 24),
			byte(mask >> 16),
			byte(mask >> 8),
			byte(mask),
		}
	}

	// v6Masks [129][16]byte
	for i := 0; i <= 128; i++ {
		var mask [16]byte
		fullBytes := i / 8
		remainingBits := i % 8
		for j := range fullBytes {
			mask[j] = 0xFF
		}
		if remainingBits > 0 && fullBytes < 16 {
			mask[fullBytes] = byte(0xFF << (8 - remainingBits))
		}

		v6Masks[i][0] = binary.BigEndian.Uint64(mask[0:8])
		v6Masks[i][1] = binary.BigEndian.Uint64(mask[8:16])
	}
}

func ipv4Mask(addr netip.Addr, prefixLen int) string {
	b4 := addr.As4()
	mask := v4Masks[prefixLen]
	b4[0] &= mask[0]
	b4[1] &= mask[1]
	b4[2] &= mask[2]
	b4[3] &= mask[3]
	return netip.AddrFrom4(b4).String()
}

func ipv6Mask(addr netip.Addr, prefixLen int) string {
	mask := v6Masks[prefixLen]

	b16 := addr.As16()

	high := binary.BigEndian.Uint64(b16[0:8])
	low := binary.BigEndian.Uint64(b16[8:16])

	high &= mask[0]
	low &= mask[1]

	binary.BigEndian.PutUint64(b16[0:8], high)
	binary.BigEndian.PutUint64(b16[8:16], low)

	return netip.AddrFrom16(b16).String()
}
