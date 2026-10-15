// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

// Package pyrandom is a byte-compatible port of the parts of CPython's
// random.Random that upstream pre-commit uses to order a hook's files:
// seeding from an int, getrandbits, _randbelow_with_getrandbits and shuffle.
//
// The generator is MT19937 exactly as CPython's _randommodule.c implements
// it, so a Random seeded with the same int produces the same stream and the
// same permutations as Python 3.
package pyrandom

const (
	n         = 624
	m         = 397
	matrixA   = 0x9908b0df
	upperMask = 0x80000000
	lowerMask = 0x7fffffff
)

// Random is a Mersenne Twister generator matching CPython's random.Random.
type Random struct {
	state [n]uint32
	index int
}

// New returns a Random seeded as random.Random().seed(seed) would seed it.
// For an int seed the seeding version is irrelevant: version only changes
// how str and bytes seeds are converted.
func New(seed int64) *Random {
	r := &Random{}
	r.Seed(seed)
	return r
}

// Seed seeds the generator from an int the way CPython's random_seed does:
// the absolute value is split into 32-bit words, least significant first,
// and fed to init_by_array. Zero becomes the single word 0.
func (r *Random) Seed(seed int64) {
	u := uint64(seed)
	if seed < 0 {
		u = uint64(-seed)
	}
	key := []uint32{uint32(u)}
	if hi := uint32(u >> 32); hi != 0 {
		key = append(key, hi)
	}
	r.initByArray(key)
}

func (r *Random) initGenrand(s uint32) {
	r.state[0] = s
	for i := 1; i < n; i++ {
		r.state[i] = 1812433253*(r.state[i-1]^(r.state[i-1]>>30)) + uint32(i)
	}
	r.index = n
}

func (r *Random) initByArray(key []uint32) {
	r.initGenrand(19650218)
	mt := &r.state
	i, j := 1, 0
	k := max(n, len(key))
	for ; k > 0; k-- {
		mt[i] = (mt[i] ^ ((mt[i-1] ^ (mt[i-1] >> 30)) * 1664525)) + key[j] + uint32(j)
		i++
		j++
		if i >= n {
			mt[0] = mt[n-1]
			i = 1
		}
		if j >= len(key) {
			j = 0
		}
	}
	for k = n - 1; k > 0; k-- {
		mt[i] = (mt[i] ^ ((mt[i-1] ^ (mt[i-1] >> 30)) * 1566083941)) - uint32(i)
		i++
		if i >= n {
			mt[0] = mt[n-1]
			i = 1
		}
	}
	mt[0] = 0x80000000
}

// Uint32 returns the next 32-bit output (CPython's genrand_uint32).
func (r *Random) Uint32() uint32 {
	mt := &r.state
	if r.index >= n {
		var kk int
		for kk = 0; kk < n-m; kk++ {
			y := (mt[kk] & upperMask) | (mt[kk+1] & lowerMask)
			mt[kk] = mt[kk+m] ^ (y >> 1) ^ mag01(y)
		}
		for ; kk < n-1; kk++ {
			y := (mt[kk] & upperMask) | (mt[kk+1] & lowerMask)
			mt[kk] = mt[kk+(m-n)] ^ (y >> 1) ^ mag01(y)
		}
		y := (mt[n-1] & upperMask) | (mt[0] & lowerMask)
		mt[n-1] = mt[m-1] ^ (y >> 1) ^ mag01(y)
		r.index = 0
	}
	y := mt[r.index]
	r.index++
	y ^= y >> 11
	y ^= (y << 7) & 0x9d2c5680
	y ^= (y << 15) & 0xefc60000
	y ^= y >> 18
	return y
}

func mag01(y uint32) uint32 {
	if y&1 != 0 {
		return matrixA
	}
	return 0
}

// GetRandBits returns a non-negative int with k random bits, as
// random.getrandbits(k) does, for 0 <= k <= 64. Wider requests take 32-bit
// words least significant first, and the last word keeps its top bits.
func (r *Random) GetRandBits(k int) uint64 {
	if k < 0 || k > 64 {
		panic("pyrandom: GetRandBits supports 0 <= k <= 64")
	}
	if k == 0 {
		return 0
	}
	if k <= 32 {
		return uint64(r.Uint32() >> (32 - k))
	}
	var v uint64
	for shift := 0; k > 0; shift += 32 {
		w := r.Uint32()
		if k < 32 {
			w >>= 32 - k
		}
		v |= uint64(w) << shift
		k -= 32
	}
	return v
}

// randBelow returns a uniform int in [0, bound), as
// _randbelow_with_getrandbits: draw bound.bit_length() bits and reject
// anything out of range.
func (r *Random) randBelow(bound int) int {
	k := bitLength(uint64(bound))
	v := r.GetRandBits(k)
	for v >= uint64(bound) {
		v = r.GetRandBits(k)
	}
	return int(v)
}

func bitLength(v uint64) int {
	l := 0
	for ; v != 0; v >>= 1 {
		l++
	}
	return l
}

// Shuffle permutes s in place exactly as random.shuffle(s) does: a
// Fisher-Yates walk from the end, swapping s[i] with s[_randbelow(i+1)].
func Shuffle[T any](r *Random, s []T) {
	for i := len(s) - 1; i > 0; i-- {
		j := r.randBelow(i + 1)
		s[i], s[j] = s[j], s[i]
	}
}
