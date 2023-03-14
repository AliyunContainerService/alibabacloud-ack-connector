package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/alibaba/alibabacloud-ack-connector/pkg/vars"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/alibaba/alibabacloud-ack-connector/pkg/tcp_tunnel/base"

	v12 "k8s.io/api/core/v1"

	"github.com/alibaba/alibabacloud-ack-connector/pkg/config"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type CERT struct {
	CA  string `json:"ca"`
	CRT string `json:"cert"`
	KEY string `json:"key"`
}

func IsNotExist(crt, key string) bool {
	_, err := os.Stat(crt)
	if os.IsNotExist(err) {
		return true
	}
	_, err = os.Stat(key)
	if os.IsNotExist(err) {
		return true
	}
	return false
}

func GetCert(clusterid, urlpath, tokenpath string) (ca, crt, key []byte, err error) {
	rawurl, err := ioutil.ReadFile(urlpath)
	if err != nil {
		err = errors.New("url not exist in Secret")
		return
	}

	token, err := ioutil.ReadFile(tokenpath)
	if err != nil {
		err = errors.New("")
		return []byte{}, []byte{}, []byte{}, fmt.Errorf("read %s in Secret err %v", tokenpath, err)
	}
	//http://cs-anony.aliyuncs.com/clusters/{{.ClusterID}}/agent/certs?Version=2015-12-15

	url := strings.Replace(string(rawurl), "{{.ClusterID}}", clusterid, 1) + "&token=" + string(token)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		err = fmt.Errorf("get certificate form request err: %v", err)
		return
	}
	req.Header.Set("Date", time.Now().Format(time.RFC1123Z))
	logrus.Printf("[%s] request for token by url %s  HEADER Date %s", clusterid, url, req.Header.Get("Date"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	datas, err := ioutil.ReadAll(resp.Body)
	if err != nil {

		err = fmt.Errorf("get certificate read body err: %v", err)
		return
	}

	if resp.StatusCode != 200 {
		err = fmt.Errorf("get certificate info err %d, body is  %s", resp.StatusCode, string(datas))
		return
	}
	var tokenresp CERT
	err = json.Unmarshal(datas, &tokenresp)
	if err != nil {
		fmt.Println(string(datas))
		err = fmt.Errorf("get certificate unmrashal result err: %v", err)
		return
	}

	ca, err = base64.StdEncoding.DecodeString(tokenresp.CA)
	if err != nil {
		err = fmt.Errorf("base64 decode ca err: %v", err)
		return
	}
	crt, err = base64.StdEncoding.DecodeString(tokenresp.CRT)
	if err != nil {
		err = fmt.Errorf("base64 decode crt err: %v", err)
		return
	}
	if tokenresp.KEY != "" {
		key, err = base64.StdEncoding.DecodeString(tokenresp.KEY)
		if err != nil {
			err = fmt.Errorf("base64 decode key err: %v", err)
			return
		}
	}

	return ca, crt, key, err
}

func PutToSecrets(config *config.ClientConfig) error {
	ca, crt, key, err := GetCert(config.ClusterID, config.ConvertUrl, config.Token)
	if err != nil {
		return fmt.Errorf("get tls config error: %s", err)
	}
	client, err := kubernetes.NewForConfig(config.Tunnel.Cfg)
	if err != nil {
		return fmt.Errorf("put tls config to apiserver error: %s", err)
	}
	var namespace string
	bytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		namespace = base.DefaultAgentNamespace
	} else {
		namespace = string(bytes)
	}

	secretName := os.Getenv(vars.SECRET_NAME)
	if secretName == "" {
		// handle all the secrets which begin with ack-credentials
		secrets, err := client.CoreV1().Secrets(namespace).List(context.TODO(), v1.ListOptions{
			TypeMeta: v1.TypeMeta{},
		})
		if err != nil {
			return err
		}
		for _, v := range secrets.Items {
			if v.Type == v12.SecretTypeOpaque && strings.HasPrefix(v.Name, "ack-credentials") {
				if len(string(v.Data[vars.TlsCrt])) > 0 && len(string(v.Data[vars.TlsKey])) > 0 {
					// already update, so no need
					continue
				}
				v.Data[vars.TlsCrt] = []byte(crt)
				v.Data[vars.TlsKey] = []byte(key)
				if len(ca) > 0 {
					// tentatively do not use ca
				}

				_, err = client.CoreV1().Secrets(namespace).Update(context.TODO(), &v, v1.UpdateOptions{})
				if err != nil {
					logrus.Infof("put tls config to apiserver error: %s", err)
				} else {
					logrus.Infof("secret %s is appended with cert and key", v.Name)
				}
			}
		}
	} else {
		// only handle this specific secret
		secret, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, v1.GetOptions{
			TypeMeta: v1.TypeMeta{},
		})
		if err != nil {
			return err
		}

		if len(ca) > 0 {
			// tentatively do not use ca
		}
		if len(string(secret.Data[vars.TlsCrt])) > 0 && len(string(secret.Data[vars.TlsKey])) > 0 {
			// already update, so no need
			logrus.Infof("secret %s is already updated with cert and key", secret.Name)
		} else {
			secret.Data[vars.TlsCrt] = []byte(crt)
			secret.Data[vars.TlsKey] = []byte(key)
			_, err = client.CoreV1().Secrets(namespace).Update(context.TODO(), secret, v1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("put tls config to apiserver error: %s", err)
			} else {
				logrus.Infof("secret %s is appended with cert and key", secret.Name)
			}
		}

	}

	config.TLSCrt = "/tls.crt"
	config.TLSKey = "/tls.key"
	if err = ioutil.WriteFile(config.TLSCrt, crt, 0600); err != nil {
		return fmt.Errorf("rewrite tls crt file failed: %s", err)
	}
	if err = ioutil.WriteFile(config.TLSKey, key, 0600); err != nil {
		return fmt.Errorf("rewrite tls key file failed: %s", err)
	}
	return nil
}
