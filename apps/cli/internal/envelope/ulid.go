package envelope

import (
	"crypto/rand"
	"io"
	"time"
)

// crockfordAlphabet is the 32-character set used by the ULID
// specification. No I/L/O/U so transcribed ULIDs are unambiguous.
const crockfordAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// NewULID returns a 26-character Crockford base32 ULID per
// https://github.com/ulid/spec — 48-bit unix-millisecond timestamp +
// 80 bits of crypto/rand. ULIDs are time-sortable: lexicographic
// order matches chronological order, which makes them safe to use
// as both run identifiers and log directory names without an
// extra timestamp column.
//
// crypto/rand failure is unrecoverable; the caller cannot produce a
// meaningful run_id without entropy, and continuing with a zeroed
// random tail would silently break uniqueness. We panic so the
// failure surfaces immediately rather than as a confusing collision
// later.
func NewULID() string {
	var b [16]byte
	ms := uint64(time.Now().UnixMilli())
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	if _, err := io.ReadFull(rand.Reader, b[6:]); err != nil {
		panic("envelope: crypto/rand unavailable: " + err.Error())
	}
	return crockfordEncode(b)
}

// crockfordEncode encodes a 16-byte ULID into 26 ASCII characters.
//
// 26 chars × 5 bits = 130 bits, but ULIDs only carry 128. The spec
// resolves this by using 3 bits in the first character (its value is
// always 0..7) and 5 bits in each of the remaining 25, summing to
// 128. The unrolled bit layout below tracks that exactly: byte
// boundaries don't align with 5-bit groups so each output index
// composes from one or two adjacent input bytes.
func crockfordEncode(b [16]byte) string {
	var out [26]byte
	out[0] = crockfordAlphabet[(b[0]&0xe0)>>5]
	out[1] = crockfordAlphabet[b[0]&0x1f]
	out[2] = crockfordAlphabet[(b[1]&0xf8)>>3]
	out[3] = crockfordAlphabet[((b[1]&0x07)<<2)|((b[2]&0xc0)>>6)]
	out[4] = crockfordAlphabet[(b[2]&0x3e)>>1]
	out[5] = crockfordAlphabet[((b[2]&0x01)<<4)|((b[3]&0xf0)>>4)]
	out[6] = crockfordAlphabet[((b[3]&0x0f)<<1)|((b[4]&0x80)>>7)]
	out[7] = crockfordAlphabet[(b[4]&0x7c)>>2]
	out[8] = crockfordAlphabet[((b[4]&0x03)<<3)|((b[5]&0xe0)>>5)]
	out[9] = crockfordAlphabet[b[5]&0x1f]
	out[10] = crockfordAlphabet[(b[6]&0xf8)>>3]
	out[11] = crockfordAlphabet[((b[6]&0x07)<<2)|((b[7]&0xc0)>>6)]
	out[12] = crockfordAlphabet[(b[7]&0x3e)>>1]
	out[13] = crockfordAlphabet[((b[7]&0x01)<<4)|((b[8]&0xf0)>>4)]
	out[14] = crockfordAlphabet[((b[8]&0x0f)<<1)|((b[9]&0x80)>>7)]
	out[15] = crockfordAlphabet[(b[9]&0x7c)>>2]
	out[16] = crockfordAlphabet[((b[9]&0x03)<<3)|((b[10]&0xe0)>>5)]
	out[17] = crockfordAlphabet[b[10]&0x1f]
	out[18] = crockfordAlphabet[(b[11]&0xf8)>>3]
	out[19] = crockfordAlphabet[((b[11]&0x07)<<2)|((b[12]&0xc0)>>6)]
	out[20] = crockfordAlphabet[(b[12]&0x3e)>>1]
	out[21] = crockfordAlphabet[((b[12]&0x01)<<4)|((b[13]&0xf0)>>4)]
	out[22] = crockfordAlphabet[((b[13]&0x0f)<<1)|((b[14]&0x80)>>7)]
	out[23] = crockfordAlphabet[(b[14]&0x7c)>>2]
	out[24] = crockfordAlphabet[((b[14]&0x03)<<3)|((b[15]&0xe0)>>5)]
	out[25] = crockfordAlphabet[b[15]&0x1f]
	return string(out[:])
}
