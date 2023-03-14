package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	errorsv1 "errors"
	"github.com/alibaba/alibabacloud-ack-connector/pkg/vars"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"time"

	"github.com/alibaba/alibabacloud-ack-connector/pkg/tcp_tunnel/base"
	"github.com/banzaicloud/satellite/api"
	"github.com/banzaicloud/satellite/defaults"
	"github.com/banzaicloud/satellite/providers"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func getAgentMeta(logger *logrus.Logger) base.AgentMeta {
	agentMeta := base.AgentMeta{}
	customizeCommand := ""

	cfg, err := rest.InClusterConfig()
	if err != nil {
		logger.Errorf("Failed to init incluster client with error: %v", err)
		return agentMeta
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logger.Errorf("Failed to init incluster client with error: %v", err)
		return agentMeta
	}

	k8sVersion, err := getK8sVersion(client)
	if err != nil {
		logger.Errorf("failed to get cluster version: ", err)
		return agentMeta
	}

	provider, err := getOrUpdateProvider(client, k8sVersion, logger)
	if err != nil {
		logger.Errorf("Failed to get provider with err: %v", err)
		return agentMeta
	}

	isIntranet, err := getIsIntranetFromEnv()
	if err != nil {
		logger.Errorf("Failed to get INTERNAL_ENDPOINT with err: %v", err)
		return agentMeta
	}

	customizeCommand, err = getCustomizeCommandFromConfigmap(client, logger)
	if err != nil {
		logger.Errorf("Failed to get addNodeScriptPath with err: %v", err)
		return agentMeta
	}

	agentMeta.K8sVersion = k8sVersion
	agentMeta.Provider = provider
	agentMeta.IsIntranet = isIntranet
	agentMeta.CustomizeCommand = customizeCommand

	return agentMeta
}

func MetaMessenger(ctx context.Context, logger *logrus.Logger, metaConn net.Conn) error {
	var num = 0
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			logger.Trace("syncing meta")
			var agentMeta base.AgentMeta

			if num%20 == 0 {
				agentMeta = getAgentMeta(logger)
			}
			if bs, err := json.Marshal(&agentMeta); err != nil {
				logger.Errorf("json marshal failed: %s, will retry after one minute", err)
			} else {
				metaConn.SetWriteDeadline(time.Now().Add(3 * time.Second))
				_, err = metaConn.Write(bs)
				if err != nil {
					return err
				}
			}
			var ack = make([]byte, 3)
			metaConn.SetReadDeadline(time.Now().Add(3 * time.Second))
			_, err := io.ReadFull(metaConn, ack)
			if err != nil {
				return err
			}
			if string(ack) != "ack" {
				logger.Warn("returned message not ack but %s", string(ack))
			}
		}
		num = (num + 1) % 20
		time.Sleep(3 * time.Second)
	}
}

func Base64EncodeStr(info string) (str string) {
	str = base64.StdEncoding.EncodeToString([]byte(info))
	return
}

func getIsIntranetFromEnv() (string, error) {
	isIntranet := os.Getenv("INTERNAL_ENDPOINT")
	if isIntranet != "true" && isIntranet != "false" {
		return "", errorsv1.New("INTERNAL_ENDPOINT value invalid, should be true or false.")
	}
	return isIntranet, nil
}

func isCustomizeCommandValidate(customizeCommand string) (bool, error) {
	if len(customizeCommand) > 1024 {
		return false, errorsv1.New("Customize command length invalid, should be less than 100.")
	}
	if customizeCommand == "" || strings.HasPrefix(customizeCommand, "https://") || strings.HasPrefix(customizeCommand, "http://") {
		return true, nil
	}
	return false, errorsv1.New("Customize command length invalid, should start with https:// or http://.")
}

func getProvider(logger *logrus.Logger) string {
	identifiers := []api.Identifier{
		&providers.IdentifyAzure{Log: logger},
		&providers.IdentifyAmazon{Log: logger},
		&providers.IdentifyDigitalOcean{Log: logger},
		&providers.IdentifyOracle{Log: logger},
		&providers.IdentifyGoogle{Log: logger},
		&providers.IdentifyAlibaba{Log: logger},
	}
	identifiedProv := defaults.Unknown
	var err error
	for _, prov := range identifiers {
		identifiedProv, err = prov.Identify()
		if err != nil {
			logger.Tracef("Failed to get provider with err: %v", err)
			continue
		}
		if identifiedProv != defaults.Unknown {
			logger.Tracef("The provider is %s", identifiedProv)
			break
		}
	}
	logger.Tracef("The provider is %s", identifiedProv)
	return identifiedProv
}

func getOrUpdateProvider(client *kubernetes.Clientset, k8sVersion string, logger *logrus.Logger) (string, error) {
	namespace := getNamespace(logger)
	cm, err := client.CoreV1().ConfigMaps(namespace).Get(context.TODO(), base.ConfigMapProviderName, metav1.GetOptions{})
	if err != nil {
		logger.Tracef("Missing configmap [%s], try to create it", base.ConfigMapProviderName)
		provider := getProvider(logger)
		if provider == defaults.Unknown {
			provider = getProviderFromK8sVersion(k8sVersion)
		}
		newcm := corev1.ConfigMap{}
		newcm.Name = base.ConfigMapProviderName
		newcm.Data = map[string]string{
			base.ConfigMapProviderKey:     provider,
			base.ConfigMapProviderAutoKey: "true",
		}
		if errors.IsNotFound(err) {
			_, err = client.CoreV1().ConfigMaps(namespace).Create(context.TODO(), &newcm, metav1.CreateOptions{})
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
		return provider, nil
	} else {
		p, a := "", ""
		if cm.Data != nil {
			p = cm.Data[base.ConfigMapProviderKey]
			a = cm.Data[base.ConfigMapProviderAutoKey]
			if a == "true" && p != "" {
				return p, nil
			}
		}

		if a != "true" || p == "" {
			p = getProvider(logger)
			if p == defaults.Unknown {
				p = getProviderFromK8sVersion(k8sVersion)
			}
			a = "true"
		}

		cm.Data = map[string]string{
			base.ConfigMapProviderKey:     p,
			base.ConfigMapProviderAutoKey: a,
		}
		_, err = client.CoreV1().ConfigMaps(namespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
		if err != nil {
			return "", err
		}
		return p, nil
	}
}

func getProviderFromK8sVersion(k8sVersion string) string {
	if strings.Contains(strings.ToLower(k8sVersion), vars.EKS) {
		return vars.Amazon
	}
	if strings.Contains(strings.ToLower(k8sVersion), vars.TKE) {
		return vars.TKE
	}
	if strings.Contains(strings.ToLower(k8sVersion), vars.CCE) {
		return vars.CCE
	}
	return vars.Idc
}

func getCustomizeCommandFromConfigmap(client *kubernetes.Clientset, logger *logrus.Logger) (string, error) {
	namespace := getNamespace(logger)
	cm, err := client.CoreV1().ConfigMaps(namespace).Get(context.TODO(), base.ConfigMapAgentConfigName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			newcm := corev1.ConfigMap{}
			newcm.Name = base.ConfigMapAgentConfigName
			newcm.Data = map[string]string{
				base.ConfigMapScriptPathKey: "",
			}
			if _, err := client.CoreV1().ConfigMaps(namespace).Create(context.TODO(), &newcm, metav1.CreateOptions{}); err != nil {
				return "", err
			}
			return "", nil
		}
		return "", err
	} else {
		if cm.Data != nil {
			v := cm.Data[base.ConfigMapScriptPathKey]
			if _, err := isCustomizeCommandValidate(v); err != nil {
				return "", err
			}
			return v, nil
		}

		cm.Data = map[string]string{
			base.ConfigMapScriptPathKey: "",
		}
		client.CoreV1().ConfigMaps(namespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
		return "", nil
	}
}

func getK8sVersion(client *kubernetes.Clientset) (string, error) {
	versionInfo, err := client.Discovery().ServerVersion()
	if err != nil {

		return "", err
	}
	version := versionInfo.GitVersion
	if strings.HasPrefix(version, "v") {
		version = strings.TrimPrefix(version, "v")
	}
	return version, nil
}

func getNamespace(logger *logrus.Logger) string {
	var namespace string
	bytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		namespace = base.DefaultAgentNamespace
		logger.Tracef("Using default namespace [%s]", namespace)
	} else {
		namespace = string(bytes)
		logger.Tracef("Load namespace [%s] from pod", namespace)
	}
	return namespace
}
