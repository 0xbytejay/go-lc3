// Copyright 2022 Google LLC
// Copyright 2026 go-lc3 contributors
// SPDX-License-Identifier: Apache-2.0
// Modified from liblc3: translated scalar bitstream and arithmetic coding to Go.

package lc3

const (
	lc3_BITS_MODE_READ = iota
	lc3_BITS_MODE_WRITE
	lc3_ACCU_BITS = 32
	lc3_AC_BITS   = 24
)

type lc3_ac_symbol struct{ low, range_ int }
type lc3_ac_model struct{ s [17]lc3_ac_symbol }
type lc3_bits_accu struct {
	v        uint32
	n, nover int
}
type lc3_bits_ac struct {
	low, range_               uint32
	cache, carry, carry_count int
	error                     bool
}
type lc3_bits_buffer struct {
	data       []byte
	p_fw, p_bw int
}
type lc3_bits_t struct {
	mode   int
	ac     lc3_bits_ac
	accu   lc3_bits_accu
	buffer lc3_bits_buffer
}

func bits_ac_get_range_bits(ac *lc3_bits_ac) int {
	n := 0
	for r := ac.range_; r != 0; r >>= 1 {
		n++
	}
	return n
}
func bits_ac_get_pending_bits(ac *lc3_bits_ac) int {
	c := 0
	if ac.cache >= 0 {
		c = 1
	}
	return 26 - bits_ac_get_range_bits(ac) + (c+ac.carry_count)*8
}
func bits_get_bits_left(bits *lc3_bits_t) int {
	end, start := bits.buffer.p_bw, bits.buffer.p_fw
	if bits.mode == lc3_BITS_MODE_READ {
		end += lc3_ACCU_BITS / 8
		start -= lc3_AC_BITS / 8
	}
	return 8*(end-start) - (bits.accu.n + bits.accu.nover + bits_ac_get_pending_bits(&bits.ac))
}
func lc3_setup_bits(bits *lc3_bits_t, mode int, buffer []byte, n int) {
	*bits = lc3_bits_t{mode: mode, ac: lc3_bits_ac{range_: 0xffffff, cache: -1}, buffer: lc3_bits_buffer{data: buffer[:n], p_bw: n}}
	if mode == lc3_BITS_MODE_READ {
		bits.accu.n = lc3_ACCU_BITS
		bits.ac.low = uint32(bits_ac_get(&bits.buffer)) << 16
		bits.ac.low |= uint32(bits_ac_get(&bits.buffer)) << 8
		bits.ac.low |= uint32(bits_ac_get(&bits.buffer))
		bits_accu_load(&bits.accu, &bits.buffer)
	}
}
func lc3_get_bits_left(bits *lc3_bits_t) int { return max(bits_get_bits_left(bits), 0) }
func lc3_check_bits(bits *lc3_bits_t) int {
	if bits_get_bits_left(bits) < 0 || bits.ac.error {
		return -1
	}
	return 0
}
func bits_accu_flush(accu *lc3_bits_accu, buffer *lc3_bits_buffer) {
	n := min(accu.n>>3, max(buffer.p_bw-buffer.p_fw, 0))
	accu.n -= 8 * n
	for ; n > 0; n-- {
		buffer.p_bw--
		buffer.data[buffer.p_bw] = byte(accu.v)
		accu.v >>= 8
	}
	if accu.n >= 8 {
		accu.n = 0
	}
}
func bits_ac_put(buffer *lc3_bits_buffer, b int) {
	if buffer.p_fw < len(buffer.data) {
		buffer.data[buffer.p_fw] = byte(b)
		buffer.p_fw++
	}
}
func bits_ac_shift(ac *lc3_bits_ac, buffer *lc3_bits_buffer) {
	if ac.low < 0xff0000 || ac.carry != 0 {
		if ac.cache >= 0 {
			bits_ac_put(buffer, ac.cache+ac.carry)
		}
		for ; ac.carry_count > 0; ac.carry_count-- {
			b := 255
			if ac.carry != 0 {
				b = 0
			}
			bits_ac_put(buffer, b)
		}
		ac.cache = int(ac.low >> 16)
		ac.carry = 0
	} else {
		ac.carry_count++
	}
	ac.low = (ac.low << 8) & 0xffffff
}
func bits_ac_terminate(ac *lc3_bits_ac, buffer *lc3_bits_buffer) {
	nbits := 25 - bits_ac_get_range_bits(ac)
	mask := uint32(0xffffff) >> nbits
	val := ac.low + mask
	high := ac.low + ac.range_
	overVal, overHigh := val>>24 != 0, high>>24 != 0
	val = (val & 0xffffff) &^ mask
	high &= 0xffffff
	if overVal == overHigh {
		if val+mask >= high {
			nbits++
			mask >>= 1
			val = ((ac.low + mask) & 0xffffff) &^ mask
		}
		if val < ac.low {
			ac.carry |= 1
		}
	}
	ac.low = val
	for ; nbits > 8; nbits -= 8 {
		bits_ac_shift(ac, buffer)
	}
	bits_ac_shift(ac, buffer)
	endVal := ac.cache >> (8 - nbits)
	if ac.carry_count != 0 {
		bits_ac_put(buffer, ac.cache)
		for ; ac.carry_count > 1; ac.carry_count-- {
			bits_ac_put(buffer, 255)
		}
		endVal = 255
		if nbits < 8 {
			endVal = 0
		}
	}
	if buffer.p_fw < len(buffer.data) {
		buffer.data[buffer.p_fw] &= byte(255 >> nbits)
		buffer.data[buffer.p_fw] |= byte(endVal << (8 - nbits))
	}
}
func lc3_flush_bits(bits *lc3_bits_t) {
	nleft := bits.buffer.p_bw - bits.buffer.p_fw
	for n := 8*nleft - bits.accu.n; n > 0; n -= 32 {
		lc3_put_bits(bits, 0, min(n, 32))
	}
	bits_accu_flush(&bits.accu, &bits.buffer)
	bits_ac_terminate(&bits.ac, &bits.buffer)
}
func lc3_put_bit(bits *lc3_bits_t, v int) { lc3_put_bits(bits, v, 1) }
func lc3_put_bits(bits *lc3_bits_t, v, n int) {
	if bits.accu.n+n <= lc3_ACCU_BITS {
		bits.accu.v |= uint32(v) << bits.accu.n
		bits.accu.n += n
	} else {
		lc3_put_bits_generic(bits, v, n)
	}
}
func lc3_put_bits_generic(bits *lc3_bits_t, v, n int) {
	accu := &bits.accu
	n1 := min(lc3_ACCU_BITS-accu.n, n)
	if n1 != 0 {
		accu.v |= uint32(v) << accu.n
		accu.n = lc3_ACCU_BITS
	}
	bits_accu_flush(accu, &bits.buffer)
	accu.v = uint32(v) >> n1
	accu.n = n - n1
}
func lc3_ac_write_renorm(bits *lc3_bits_t) {
	for ; bits.ac.range_ < 0x10000; bits.ac.range_ <<= 8 {
		bits_ac_shift(&bits.ac, &bits.buffer)
	}
}
func lc3_put_symbol(bits *lc3_bits_t, model *lc3_ac_model, s int) {
	ac := &bits.ac
	r := ac.range_ >> 10
	ac.low += r * uint32(model.s[s].low)
	ac.range_ = r * uint32(model.s[s].range_)
	ac.carry |= int(ac.low >> 24)
	ac.low &= 0xffffff
	if ac.range_ < 0x10000 {
		lc3_ac_write_renorm(bits)
	}
}
func bits_ac_get(buffer *lc3_bits_buffer) int {
	if buffer.p_fw < len(buffer.data) {
		v := int(buffer.data[buffer.p_fw])
		buffer.p_fw++
		return v
	}
	return 0
}
func bits_accu_load(accu *lc3_bits_accu, buffer *lc3_bits_buffer) {
	n := min(accu.n>>3, buffer.p_bw)
	accu.n -= 8 * n
	for ; n > 0; n-- {
		accu.v >>= 8
		buffer.p_bw--
		accu.v |= uint32(buffer.data[buffer.p_bw]) << (lc3_ACCU_BITS - 8)
	}
	if accu.n >= 8 {
		accu.nover = min(accu.nover+accu.n, lc3_ACCU_BITS)
		accu.v >>= accu.n
		accu.n = 0
	}
}
func lc3_get_bit(bits *lc3_bits_t) int { return lc3_get_bits(bits, 1) }
func bits_mask(n int) uint32 {
	if n >= 32 {
		return 0xffffffff
	}
	return (uint32(1) << n) - 1
}
func lc3_get_bits(bits *lc3_bits_t, n int) int {
	accu := &bits.accu
	if accu.n+n <= lc3_ACCU_BITS {
		v := (accu.v >> accu.n) & bits_mask(n)
		accu.n += n
		return int(v)
	}
	return lc3_get_bits_generic(bits, n)
}
func lc3_get_bits_generic(bits *lc3_bits_t, n int) int {
	accu := &bits.accu
	bits_accu_load(accu, &bits.buffer)
	n1 := min(lc3_ACCU_BITS-accu.n, n)
	v := (accu.v >> accu.n) & bits_mask(n1)
	accu.n += n1
	n2 := n - n1
	if n2 != 0 {
		bits_accu_load(accu, &bits.buffer)
		v |= ((accu.v >> accu.n) & bits_mask(n2)) << n1
		accu.n += n2
	}
	return int(v)
}
func lc3_ac_read_renorm(bits *lc3_bits_t) {
	for ; bits.ac.range_ < 0x10000; bits.ac.range_ <<= 8 {
		bits.ac.low = ((bits.ac.low << 8) | uint32(bits_ac_get(&bits.buffer))) & 0xffffff
	}
}
func lc3_get_symbol(bits *lc3_bits_t, model *lc3_ac_model) int {
	ac := &bits.ac
	r := (ac.range_ >> 10) & 0xffff
	ac.error = ac.error || ac.low >= r<<10
	if ac.error {
		ac.low = 0
	}
	s := 16
	if ac.low < r*uint32(model.s[s].low) {
		s >>= 1
		if ac.low < r*uint32(model.s[s].low) {
			s -= 4
		} else {
			s += 4
		}
		if ac.low < r*uint32(model.s[s].low) {
			s -= 2
		} else {
			s += 2
		}
		if ac.low < r*uint32(model.s[s].low) {
			s--
		} else {
			s++
		}
		if ac.low < r*uint32(model.s[s].low) {
			s--
		}
	}
	ac.low -= r * uint32(model.s[s].low)
	ac.range_ = r * uint32(model.s[s].range_)
	if ac.range_ < 0x10000 {
		lc3_ac_read_renorm(bits)
	}
	return s
}
