package agent

import (
	"bufio"
	"crypto/tls"
	"net"
	"net/http"
)

// TLSRoundTripper is a roundtripper used for http client connection hijack
// Usually, http client will automatically close connection after handling response but SPDY needs to hold that
// connection and expose it to outer scope for later use. This roundtripper is designed to do it.
type TLSRoundTripper struct {
	Conn net.Conn
}

func (tlsRoundTripper *TLSRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	err := r.Write(tlsRoundTripper.Conn)
	if err != nil {
		return nil, err
	}
	return http.ReadResponse(bufio.NewReader(tlsRoundTripper.Conn), r)
}

func NewTLSRoundTripper(config *tls.Config, address string) (*TLSRoundTripper, error) {
	conn, err := tls.Dial("tcp", address, config)
	if err != nil {
		return nil, err
	}
	return &TLSRoundTripper{Conn: conn}, nil
}
