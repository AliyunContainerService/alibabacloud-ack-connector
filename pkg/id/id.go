package id

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/calmh/luhn"
)

type ID [32]byte

func NewID(data []byte) ID {
	var id ID

	hash := sha256.New()
	hash.Write(data)
	hash.Sum(id[:0])

	return id
}

func (i ID) String() string {
	ss := base32.StdEncoding.EncodeToString(i[:])
	ss = strings.Trim(ss, "=")

	ss, err := luhnify(ss)
	if err != nil {
		// Should never happen
		panic(err)
	}

	ss = chunkify(ss)

	return ss
}

func (i ID) Compare(other ID) int {
	return bytes.Compare(i[:], other[:])
}

func (i ID) Equals(other ID) bool {
	return subtle.ConstantTimeCompare(i[:], other[:]) == 1
}

func (i *ID) MarshalText() ([]byte, error) {
	return []byte(i.String()), nil
}

func (i *ID) UnmarshalText(bs []byte) (err error) {
	id := string(bs)
	id = strings.Trim(id, "=")
	id = strings.ToUpper(id)
	id = untypeoify(id)
	id = unchunkify(id)

	if len(id) != 56 {
		return errors.New("device ID invalid: incorrect length")
	}

	id, err = unluhnify(id)
	if err != nil {
		return err
	}

	dec, err := base32.StdEncoding.DecodeString(id + "====")
	if err != nil {
		return err
	}

	copy(i[:], dec)
	return nil
}

func luhnify(s string) (string, error) {
	if len(s) != 52 {
		panic("unsupported string length")
	}

	res := make([]string, 0, 4)
	for i := 0; i < 4; i++ {
		chunk := s[i*13 : (i+1)*13]

		l, err := luhn.Base32.Generate(chunk)
		if err != nil {
			return "", err
		}

		res = append(res, fmt.Sprintf("%s%c", chunk, l))
	}

	return res[0] + res[1] + res[2] + res[3], nil
}

func unluhnify(s string) (string, error) {
	if len(s) != 56 {
		return "", fmt.Errorf("unsupported string length %d", len(s))
	}

	res := make([]string, 0, 4)
	for i := 0; i < 4; i++ {
		chunk := s[i*14 : (i+1)*14]

		l, err := luhn.Base32.Generate(chunk[0:13])
		if err != nil {
			return "", err
		}

		if fmt.Sprintf("%c", l) != chunk[13:] {
			return "", errors.New("check digit incorrect")
		}

		res = append(res, chunk[0:13])
	}

	return res[0] + res[1] + res[2] + res[3], nil
}

func chunkify(s string) string {
	s = regexp.MustCompile("(.{7})").ReplaceAllString(s, "$1-")
	s = strings.Trim(s, "-")
	return s
}

func unchunkify(s string) string {
	s = strings.Replace(s, "-", "", -1)
	s = strings.Replace(s, " ", "", -1)
	return s
}

func untypeoify(s string) string {
	s = strings.Replace(s, "0", "O", -1)
	s = strings.Replace(s, "1", "I", -1)
	s = strings.Replace(s, "8", "B", -1)
	return s
}
