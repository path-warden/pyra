// Package identity mints and resolves opaque Canon artifact IDs.
//
// An ID has the form "<repository-key>-<12-char Crockford base32>", e.g.
// "OKF-KTQ63DPSMF19". Minting takes injected entropy so it is a pure, testable
// function and never depends on a global clock or RNG; entropy is generated only
// at artifact-creation time (canon/identity.NewEntropy), never on read paths.
package identity

import (
	"crypto/rand"
	"encoding/binary"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/chasedputnam/pyra/internal/canon/model"
)

// crockford is the Crockford base32 alphabet (no I, L, O, U).
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// idLen is the number of Crockford characters in the random suffix.
const idLen = 12

var (
	idRe          = regexp.MustCompile(`^[A-Z0-9]+-[0-9A-HJKMNP-TV-Z]{12}$`)
	repoKeyRe     = regexp.MustCompile(`^[A-Z0-9]+$`)
	filenamePfxRe = regexp.MustCompile(`^([A-Za-z]+-\d+)`)
)

// NewEntropy returns 8 fresh random bytes for minting. This is the only source
// of randomness in the package and is used only by create-time commands.
func NewEntropy() [8]byte {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return b
}

// Mint produces an ID for the given repository key and entropy. The repository
// key is upper-cased; entropy is interpreted big-endian.
func Mint(repoKey string, entropy [8]byte) string {
	key := strings.ToUpper(strings.TrimSpace(repoKey))
	v := binary.BigEndian.Uint64(entropy[:])
	return key + "-" + encodeCrockford(v, idLen)
}

func encodeCrockford(v uint64, n int) string {
	b := make([]byte, n)
	for i := n - 1; i >= 0; i-- {
		b[i] = crockford[v&0x1f]
		v >>= 5
	}
	return string(b)
}

// ValidID reports whether s is a well-formed artifact ID.
func ValidID(s string) bool {
	return idRe.MatchString(s)
}

// ValidRepositoryKey reports whether a repository key is usable for minting.
func ValidRepositoryKey(key string) bool {
	return repoKeyRe.MatchString(strings.ToUpper(strings.TrimSpace(key)))
}

// Aliases returns the non-opaque identifiers by which an artifact may be
// referenced from other artifacts: its filename stem and any "<letters>-<digits>"
// prefix (e.g. "adr-002"). These let cross-references like "ADR-002" resolve to
// the artifact whose file is adr-002-*.md regardless of its opaque frontmatter id.
func Aliases(filename string) []string {
	base := filepath.Base(filename)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	out := []string{stem}
	if m := filenamePfxRe.FindStringSubmatch(stem); m != nil && m[1] != stem {
		out = append(out, m[1])
	}
	return out
}

// Resolve determines an artifact's identifier using the precedence:
//  1. explicit frontmatter id
//  2. a "<letters>-<digits>" prefix of the filename (e.g. adr-001)
//  3. the filename stem
//
// The returned bool reports whether the result came from an explicit/structured
// source (1 or 2) rather than the bare stem fallback (3).
func Resolve(p *model.Product, filename string) (string, bool) {
	if p != nil && strings.TrimSpace(p.Metadata.ID) != "" {
		return strings.TrimSpace(p.Metadata.ID), true
	}
	base := filepath.Base(filename)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	if m := filenamePfxRe.FindStringSubmatch(stem); m != nil {
		return m[1], true
	}
	return stem, false
}
