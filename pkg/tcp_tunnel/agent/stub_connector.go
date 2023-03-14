package agent

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"net"
	"time"

	"github.com/alibaba/alibabacloud-ack-connector/pkg/id"

	"github.com/alibaba/alibabacloud-ack-connector/pkg/tcp_tunnel/base"
	"github.com/cenkalti/backoff/v4"
	"github.com/sirupsen/logrus"
)

const (
	NonSessionBackoffInitialInterval     = 500 * time.Millisecond
	NonSessionBackoffMultiplier          = 1.2
	NonSessionBackoffRandomizationFactor = 0.05
	NonSessionBackoffMaxInterval         = 10 * time.Second
	NonSessionBackoffMaxElapsedTime      = 0
)

// StubConnector construct new connection to stub server
type StubConnector struct {
	base.Component
	urlStr    string
	tlsConfig *tls.Config
	clusterID id.ID
}

func NewStubConnector(ctx context.Context, logger *logrus.Logger, urlStr string, tlsConfig *tls.Config) StubConnector {
	bytes, err := ioutil.ReadFile("/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		logger.Fatal("cannot recognize running cluster: ", err)
	}
	return StubConnector{
		Component: base.NewComponent(ctx, logger),
		urlStr:    urlStr,
		tlsConfig: tlsConfig,
		clusterID: id.NewID(bytes),
	}
}

// isSession = 0 means this is the connection of request channel (first registration channel)
// isSession = 1 means this is a new connection for some http request
// sessionID should be 0 is isSession = 0, otherwise it represents the current session id received from request channel
func (sc *StubConnector) Connect(isSession byte, sessionID uint16) (conn net.Conn, err error) {
	logger := sc.Logger
	if isSession == 1 {
		logger = sc.Logger.WithField(base.SessionIDHeaderKey, sessionID).Logger
	}
	logger.Tracef("dialing %s", sc.urlStr)
	backoffConfig := backoff.NewExponentialBackOff()
	if isSession == 0 {
		backoffConfig.InitialInterval = NonSessionBackoffInitialInterval
		backoffConfig.Multiplier = NonSessionBackoffMultiplier
		backoffConfig.RandomizationFactor = NonSessionBackoffRandomizationFactor
		backoffConfig.MaxInterval = NonSessionBackoffMaxInterval
		backoffConfig.MaxElapsedTime = NonSessionBackoffMaxElapsedTime
		backoffConfig.Reset()
	}
	//backoffObj := backoff.WithMaxRetries(backoffConfig, 3)
	if err = backoff.Retry(func() error {
		var e error
		conn, e = tls.DialWithDialer(&net.Dialer{KeepAlive: 0, Timeout: 30 * time.Second}, "tcp", sc.urlStr, sc.tlsConfig)
		return e
	}, backoffConfig); err != nil {
		logger.Errorf("dialing error %v", err)
		return nil, err
	}
	logger.Trace("try to handshake")
	bs := []byte{isSession, 0, 0}
	binary.BigEndian.PutUint16(bs[1:3], sessionID)
	if n, err := conn.Write(bs); err != nil || n != 3 {
		conn.Close()
		err = fmt.Errorf("handshake failed <%d>: %s", n, err)
		logger.Trace(err)
		return nil, err
	}
	if isSession == 1 {
		logger.Trace("connected")
	} else {
		logger.Info("writing cluster id")
		if n, err := conn.Write(sc.clusterID[:]); err != nil {
			conn.Close()
			err = fmt.Errorf("send cluster id failed <%d>: %s", n, err)
			logger.Info(err)
			return nil, err
		}
		logger.Info("connected")
	}
	return conn, nil
}
