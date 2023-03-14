package agent

import (
	"context"
	"net"
	"net/http"
	"net/url"

	"github.com/alibaba/alibabacloud-ack-connector/pkg/tcp_tunnel/base"

	"github.com/alibaba/alibabacloud-ack-connector/pkg/utils"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
)

// KubernetesClientManager manages http clients with connection to kubernetes api server.
type KubernetesClientManager struct {
	base.Component
	config *rest.Config
	target *url.URL
}

func NewKubernetesClientManager(ctx context.Context, logger *logrus.Logger, config *rest.Config, target *url.URL) KubernetesClientManager {
	return KubernetesClientManager{
		Component: base.NewComponent(ctx, logger),
		config:    config,
		target:    target,
	}
}

// Do function will automatically create new http client and do http request. The connection is hijacked and hold when
// response is successfully returned. If any error exists, no connection or response returned. The close of hijacked
// connection should be processed by function return value receiver.
func (kcm *KubernetesClientManager) Do(sessionID uint16, r *http.Request) (net.Conn, *http.Response, error) {
	logger := kcm.Logger.WithField(base.SessionIDHeaderKey, sessionID)
	transportConfig, err := kcm.config.TransportConfig()
	if err != nil {
		logger.Debug("cannot create transport config: ", err)
		return nil, nil, err
	}
	tlsConfig, err := transport.TLSConfigFor(transportConfig)
	if err != nil {
		logger.Debug("cannot create tls config: ", err)
		return nil, nil, err
	}
	tlsRoundTripper, err := NewTLSRoundTripper(tlsConfig, kcm.target.Host)
	if err != nil {
		logger.Debug("cannot create tlsRoundTripper: ", err)
		return nil, nil, err
	}
	_transport, err := transport.HTTPWrappersForConfig(transportConfig, tlsRoundTripper)
	if err != nil {
		logger.Debug("cannot create transport: ", err)
		return nil, nil, err
	}
	client := http.Client{Transport: _transport}
	conn := tlsRoundTripper.Conn
	utils.RedirectRequest(r, kcm.target)
	logger.Trace("sending redirected request to api server: ", r.URL.String())
	resp, err := client.Do(r)
	if err != nil {
		conn.Close()
		logger.Debug("do request failed: ", err)
		return nil, nil, err
	}
	return conn, resp, nil
}
