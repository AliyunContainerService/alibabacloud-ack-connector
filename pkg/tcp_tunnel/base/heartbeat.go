package base

import (
	"context"
	"github.com/alibaba/alibabacloud-ack-connector/pkg/vars"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	HeartbeatInterval = time.Second
	CheckInterval     = time.Second * 5
)

type Heart struct {
	//0 represent ok
	//1 represent cluster is sick
	status int32
	ctx    context.Context
	logger *logrus.Logger
	cfg    *rest.Config
	conn   net.Conn
}

func (h *Heart) Beat() {
	t := time.NewTicker(15 * HeartbeatInterval)
	defer t.Stop()
	//close to connection to notify read goroutine and reconnect
	defer h.conn.Close()
	var cnt = 0
	for {
		select {
		case <-h.ctx.Done():
			h.logger.Trace("heartbeat goroutine finished by context done")
			return
		case <-t.C:
			status := atomic.LoadInt32(&h.status)
			h.conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
			beat := make([]byte, vars.PayloadLength)
			for i := 0; i < len(beat); i++ {
				beat[i] = uint8(status)
			}
			n, e := h.conn.Write(beat)
			if e != nil {
				h.logger.Error("heartbeat to stub server failed:", e)
				return
			}
			if cnt == 0 {
				h.logger.Trace("heartbeat to stub server success with length ", n)
			}
			cnt = (cnt + 1) % 10
		}
	}
}

func (h *Heart) CheckCluster() {
	client, err := kubernetes.NewForConfig(h.cfg)
	if err != nil {
		h.logger.Errorf("build client for kubernetes err %v", err)
		return
	}

	_, err = client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: vars.AlibabacloudNodeLabel})
	if err != nil {
		h.logger.Errorf("health check failed with err %v", err)
		atomic.StoreInt32(&h.status, 1)
	} else {
		atomic.StoreInt32(&h.status, 0)
	}
	t := time.NewTicker(15 * CheckInterval)
	var cnt = 0
	defer t.Stop()
	for {
		select {
		case <-h.ctx.Done():
			h.logger.Trace("check K8s cluster goroutine finished by context done")
			return
		case <-t.C:
			_, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: "alibabacloud.com/external=true"})
			if err != nil {
				atomic.StoreInt32(&h.status, 1)
				h.logger.Errorf("heart beat to cluster err %v", err)
				continue
			}
			atomic.StoreInt32(&h.status, 0)
			if cnt == 0 {
				h.logger.Trace("check K8s cluster success..")
			}
			cnt = (cnt + 1) % 10
		}
	}
}

// Heartbeat send heartbeat byte to stub requester each HeartbeatInterval in order to maintain connection.
// Otherwise, requester will decrease the health of this connection and in the end kick it off.
func Heartbeat(ctx context.Context, logger *logrus.Logger, conn net.Conn, cfg *rest.Config) {
	h := &Heart{
		status: 1,
		ctx:    ctx,
		cfg:    cfg,
		logger: logger,
		conn:   conn,
	}
	go h.CheckCluster()
	h.Beat()
}

func StartHealthzServer(addr string, logger *logrus.Logger) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
	})
	s := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	logger.Infof("start healthz server and listen on addr: %s", addr)
	s.ListenAndServe()
}
