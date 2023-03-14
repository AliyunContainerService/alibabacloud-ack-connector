package tcp_tunnel

import (
	"bufio"
	"context"
	"crypto/tls"
	"github.com/alibaba/alibabacloud-ack-connector/pkg/tcp_tunnel/agent"
	"github.com/alibaba/alibabacloud-ack-connector/pkg/tcp_tunnel/base"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
)

type AgentClient struct {
	base.TunnelEndpoint
	kubernetesClientManager agent.KubernetesClientManager
	stubConnector           agent.StubConnector
}

// The creation of AgentClient will block until connection is gone or context is done.
func RunAgent(ctx context.Context, logger *logrus.Logger, urlStr string, targetURL *url.URL, cfg *rest.Config, tunnelTLSConfig *tls.Config, tunnelSPerAgent int) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	client := &AgentClient{
		TunnelEndpoint:          base.NewTunnelEndpoint(ctx, logger),
		kubernetesClientManager: agent.NewKubernetesClientManager(ctx, logger, cfg, targetURL),
		stubConnector:           agent.NewStubConnector(ctx, logger, urlStr, tunnelTLSConfig),
	}
	client.Logger.Infof("proxy to %s", targetURL)
	client.Logger.Infof("waiting for meta connection established")

	var reconnect = make(chan struct{}, 2)
	for i := 0; i < tunnelSPerAgent; i++ {
		conn, err := client.stubConnector.Connect(0, 0)
		if err != nil {
			return err
		}
		defer conn.Close()

		//logger.Infof("tunnelsPerAgent, desired: %v", tunnelSPerAgent)
		go base.Heartbeat(ctx, logger, conn, cfg)
		go func() {
			var lock sync.Mutex
			for {
				select {
				case <-ctx.Done():
					logger.Info("context done")
					return
				default:
					lock.Lock()
					request, err := http.ReadRequest(bufio.NewReader(conn))
					if err != nil {
						lock.Unlock()
						client.Logger.Error("read request failed: ", err)
						// skip reconnect when read request failed from current connection
						continue
					}
					sessionID, err := strconv.ParseUint(request.Header.Get(base.SessionIDHeaderKey), 10, 16)
					if err != nil {
						client.Logger.Error("read tunnel session id failed: ", err)
						lock.Unlock()
						continue
					}
					go client.newSession(uint16(sessionID), request, &lock)
				}
			}
		}()
	}

	//the channel will be closed by gc after all goroutines were closed by context
	// var reconnect = make(chan struct{}, 2)
	go func() {
		logger.Info("meta connection establishing")
		metaConn, err := client.stubConnector.Connect(2, 0)
		if err != nil {
			logger.Errorf("meta connection connect failed: %s", err)
			reconnect <- struct{}{}
			return
		}
		defer metaConn.Close()
		logger.Info("meta connection established")
		go base.StartHealthzServer(":10254", logger)
		for {
			select {
			case <-ctx.Done():
				logger.Info("meta connection exit normally")
				return
			default:
				if err = agent.MetaMessenger(ctx, logger, metaConn); err != nil {
					logger.Errorf("meta connection failed: %s", err)
				}
				reconnect <- struct{}{}
				return
			}
		}
	}()
	<-reconnect
	logger.Info("reconnect signal, try to reconnect")
	return nil
}

func (client *AgentClient) newSession(sessionID uint16, request *http.Request, lock *sync.Mutex) {
	var err error

	k8sConn, response, err := client.kubernetesClientManager.Do(sessionID, request)
	lock.Unlock()
	if err != nil {
		client.Logger.Error("connect session with K8s err: ", err)
		return
	}

	var agentConn net.Conn
	if agentConn, err = client.stubConnector.Connect(1, sessionID); err != nil {
		client.Logger.Error("connect stub err: ", err)
		return
	}
	defer agentConn.Close()

	if err = response.Write(agentConn); err != nil {
		client.Logger.Error("write HTTP response failed: ", err)
		return
	}

	client.Logger.Trace("HTTP response returned")

	client.CheckAndStartPipe(request, response, agentConn, k8sConn)
}
