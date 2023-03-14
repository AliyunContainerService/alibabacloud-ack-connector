package config

import (
	"time"

	"k8s.io/client-go/rest"
)

const (
	DefaultBackoffInterval    = 500 * time.Millisecond
	DefaultBackoffMultiplier  = 1.5
	DefaultBackoffMaxInterval = 10 * time.Second
	DefaultBackoffMaxTime     = 0
)

const (
	HTTP  = "http"
	HTTPS = "https"
)

type BackoffConfig struct {
	Interval    time.Duration
	Multiplier  float64
	MaxInterval time.Duration
	MaxTime     time.Duration
}

type Tunnel struct {
	Name       string
	Protocol   string
	Addr       string
	RemoteAddr string

	Cfg *rest.Config
}

type ClientConfig struct {
	ServerAddr      string
	TLSCrt          string
	TLSKey          string
	RootCA          string
	Backoff         BackoffConfig
	ClusterID       string
	KubeConfig      string
	Tunnel          *Tunnel
	ConvertUrl      string
	Token           string
	TunnelsPerAgent int
}
