package agent

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/alibaba/alibabacloud-ack-connector/pkg/tcp_tunnel"
	"k8s.io/client-go/rest"

	log "github.com/sirupsen/logrus"
)

type ClientConfig struct {
	ServerAddr      string
	TLSClientConfig *tls.Config
	Logger          *log.Logger
}

type Client struct {
	config         *ClientConfig
	lastDisconnect time.Time
	logger         *log.Logger
}

func NewClient(config *ClientConfig) (*Client, error) {
	if config.ServerAddr == "" {
		return nil, errors.New("missing ServerAddr")
	}
	if config.TLSClientConfig == nil {
		return nil, errors.New("missing TLSClientConfig")
	}

	logger := config.Logger
	if logger == nil {
		logger = log.New()
	}

	c := &Client{
		config: config,
		logger: logger,
	}

	return c, nil
}

func (c *Client) Start(targetURLStr string, cfg *rest.Config, tunnelsPerAgent int) error {

	c.logger.Info("agent started")

	for {
		targetURL, err := url.Parse(targetURLStr)
		if err != nil {
			return err
		}
		err = tcp_tunnel.RunAgent(context.Background(), c.logger, c.config.ServerAddr, targetURL, cfg, c.config.TLSClientConfig, tunnelsPerAgent)
		if err != nil {
			c.logger.Error("agent client failed: ", err)
		}

		c.logger.Info("connection disconnected")

		now := time.Now()

		// detect disconnect hiccup
		if err == nil && now.Sub(c.lastDisconnect).Seconds() < 5 {
			err = fmt.Errorf("connection is being cut")
		}

		c.lastDisconnect = now

		if err != nil {
			return err
		}
	}
}
