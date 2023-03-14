package id

import (
	"fmt"
)

// ImproperCertsNumberError returned error when peer certificate is not 1
type ImproperCertsNumberError struct {
	N int
}

func (e ImproperCertsNumberError) Error() string {
	return fmt.Sprintf("tls: expecting more than 1 peer certificate, got %d", e.N)
}
