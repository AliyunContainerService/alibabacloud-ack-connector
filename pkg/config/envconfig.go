package config

import (
	"errors"
	"fmt"
	"github.com/alibaba/alibabacloud-ack-connector/pkg/vars"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	kubernetesServiceHostKey = "KUBERNETES_SERVICE_HOST"
	kubernetesServicePortKey = "KUBERNETES_SERVICE_PORT"
	kubernetesProto          = "https"
	tunnelsPerAgentKey       = "TUNNELS_PER_AGENT"
)

func LoadClientConfigFromEnv() (*ClientConfig, error) {
	c := ClientConfig{
		Backoff: BackoffConfig{
			Interval:    DefaultBackoffInterval,
			Multiplier:  DefaultBackoffMultiplier,
			MaxInterval: DefaultBackoffMaxInterval,
			MaxTime:     DefaultBackoffMaxTime,
		},
	}
	stubserver, err := getEnv(vars.EnvStubServer)
	if err != nil {
		return nil, err
	}
	c.ServerAddr = stubserver
	if c.ServerAddr, err = getAddress(c.ServerAddr); err != nil {
		return nil, fmt.Errorf("server_addr: %s", err)
	}
	clusterID, err := getEnv(vars.ClusterID)
	if err != nil {
		return nil, err
	}

	k8stun, err := GetK8sTunnelFromENV()
	if err != nil {
		return nil, err
	}
	k8stun.Name = clusterID

	c.ClusterID = clusterID
	c.TLSCrt = getPath(vars.TlsCrt)
	c.TLSKey = getPath(vars.TlsKey)
	c.RootCA = getPath(vars.RootCa)
	c.Tunnel = k8stun
	c.ConvertUrl = getPath(vars.ConvertUrl)
	c.Token = getPath(vars.ConnectToken)
	tunnelsPerAgentStr, err := getEnv(tunnelsPerAgentKey)
	if err != nil {
		c.TunnelsPerAgent = 1
	}
	tunnelsPerAgent, err := strconv.Atoi(tunnelsPerAgentStr)
	if err != nil {
		c.TunnelsPerAgent = 1
	} else {
		c.TunnelsPerAgent = tunnelsPerAgent
	}
	return &c, nil
}

func getEnv(env string) (string, error) {
	value := os.Getenv(env)
	if value == "" {
		return "", fmt.Errorf("%s is empty", env)
	}
	return value, nil
}

func getPath(key string) string {
	return path.Join(vars.AliyunCredentialsFolder, key)
}

func populateCAData(cfg *rest.Config) error {
	if len(cfg.CAData) == 0 {
		bytes, err := ioutil.ReadFile(cfg.CAFile)
		if err != nil {
			return err
		}
		cfg.CAData = bytes
	}
	return nil
}

func GetK8sTunnelFromENV() (*Tunnel, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	if err := populateCAData(cfg); err != nil {
		return nil, err
	}

	kubernetesServiceHost, err := getEnv(kubernetesServiceHostKey)
	if err != nil {
		return nil, err
	}
	kubernetesServicePort, err := getEnv(kubernetesServicePortKey)
	if err != nil {
		return nil, err
	}

	return &Tunnel{
		Protocol: kubernetesProto,
		Addr:     "https://" + fmt.Sprintf("%s:%s", kubernetesServiceHost, kubernetesServicePort),
		Cfg:      cfg,
	}, nil
}

func GetK8sTunnelFromFile(kubeconfig string) (*Tunnel, error) {
	config, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("load %s err %v", kubeconfig, err)
	}

	cluster, ok := config.Clusters[config.CurrentContext]
	if !ok {
		return nil, errors.New("kubeconfig is invalid with no cluster info of  " + config.CurrentContext)
	}

	user, ok := config.AuthInfos[config.CurrentContext]
	if !ok {
		return nil, errors.New("kubeconfig is invalid with no user info of  " + config.CurrentContext)
	}

	cfg := &rest.Config{
		Host: cluster.Server,
		TLSClientConfig: rest.TLSClientConfig{
			CAFile: cluster.CertificateAuthority,
			CAData: cluster.CertificateAuthorityData,

			CertFile: user.ClientCertificate,
			CertData: user.ClientCertificateData,

			KeyFile: user.ClientKey,
			KeyData: user.ClientKeyData,
		},
	}

	if !strings.HasPrefix(cfg.Host, "https") {
		cfg.Host = "https://" + cfg.Host
	}

	return &Tunnel{
		Protocol: kubernetesProto,
		Addr:     cfg.Host,
		Cfg:      cfg,
	}, nil
}
