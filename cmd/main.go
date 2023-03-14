package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/alibaba/alibabacloud-ack-connector/common"
	"github.com/alibaba/alibabacloud-ack-connector/pkg/logging"
	"github.com/alibaba/alibabacloud-ack-connector/pkg/utils"
	"github.com/alibaba/alibabacloud-ack-connector/pkg/vars"
	"net"
	"os"
	"strconv"

	"github.com/alibaba/alibabacloud-ack-connector/pkg/agent"
	"github.com/alibaba/alibabacloud-ack-connector/pkg/config"
	conf "github.com/alibaba/alibabacloud-ack-connector/pkg/config"
	log "github.com/sirupsen/logrus"
)

func main() {
	opts, err := parseArgs()
	if err != nil {
		log.Fatal(err)
	}
	logLevel := os.Getenv(vars.LOG_LEVEL)
	if logLevel != "" {
		level, err := strconv.Atoi(logLevel)
		if err == nil {
			opts.logLevel = level
		}
	}
	v := common.GetVersion()
	fmt.Println("version:", v.Version)
	fmt.Println("git commit id:", v.GitCommit)
	fmt.Println("git tag:", v.GitTag)
	fmt.Println("build date:", v.BuildDate)
	fmt.Println("gitTreeState:", v.GitTreeState)
	fmt.Println("log level(-1:trace,0:debug,1:info,2:warn,3:error):", opts.logLevel)
	logger := logging.NewLogger(opts.logLevel)

	switch opts.logLevel {
	case -1:
		logger.SetLevel(log.TraceLevel)
	case 0:
		logger.SetLevel(log.DebugLevel)
	case 1:
		logger.SetLevel(log.InfoLevel)
	case 2:
		logger.SetLevel(log.WarnLevel)
	default:
		logger.SetLevel(log.ErrorLevel)
	}

	var clientConfig *config.ClientConfig
	clientConfig, err = conf.LoadClientConfigFromEnv()
	if err != nil {
		logger.Fatalf("configuration error: %s", err)
	}
	if agent.IsNotExist(clientConfig.TLSCrt, clientConfig.TLSKey) {
		err = agent.PutToSecrets(clientConfig)
		if err != nil {
			logger.Fatalf("%v", err)
		}
		log.Infof("store client crt and key success, continue")
	}

	if clientConfig.Tunnel == nil {
		logger.Fatal("no tunnels")
	}

	tlsconf, err := tlsConfig(clientConfig)
	if err != nil {
		logger.Fatalf("failed to configure tls: %s", err)
	}

	client, err := agent.NewClient(&agent.ClientConfig{
		ServerAddr:      clientConfig.ServerAddr,
		TLSClientConfig: tlsconf,
		Logger:          logger,
	})
	if err != nil {
		logger.Fatalf("failed to create client: %s", err)
	}

	if err := client.Start(clientConfig.Tunnel.Addr, clientConfig.Tunnel.Cfg, clientConfig.TunnelsPerAgent); err != nil {
		logger.Fatalf("failed to start tunnels: %s", err)
	}

}

func tlsConfig(config *config.ClientConfig) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(config.TLSCrt, config.TLSKey)
	if err != nil {
		return nil, err
	}

	var roots *x509.CertPool
	if config.RootCA != "" && utils.IsExist(config.RootCA) {
		fmt.Println("[WARN]: rootCA: ", config.RootCA, " is exist: but not used now")
	}

	host, _, err := net.SplitHostPort(config.ServerAddr)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		ServerName:         host,
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: roots == nil,
		RootCAs:            roots,
	}, nil
}
