package base

type TunnelState string

const (
	DefaultAgentNamespace    string = "kube-system"
	ConfigMapProviderName    string = "provider"
	ConfigMapProviderKey     string = "provider"
	ConfigMapProviderAutoKey string = "auto"
	ConfigMapAgentConfigName string = "ack-agent-config"
	ConfigMapScriptPathKey   string = "addNodeScriptPath"
)

// Note: Any change to this struct needs to update DeepCopy function as well.
type AgentMeta struct {
	Provider         string            `json:"provider"`
	K8sVersion       string            `json:"k8sversion"`
	IsIntranet       string            `json:"isintranet"`
	CustomizeCommand string            `json:"customizecommand"`
	Data             map[string]string `json:"data"`
}

func (src *AgentMeta) DeepCopy(dest *AgentMeta) {
	dest.Provider = src.Provider
	dest.K8sVersion = src.K8sVersion
	dest.IsIntranet = src.IsIntranet
	dest.CustomizeCommand = src.CustomizeCommand
	dest.Data = make(map[string]string)
	for key, value := range src.Data {
		dest.Data[key] = value
	}
}

type StubMeta struct {
	State         TunnelState          `json:"state"`
	ActiveCluster string               `json:"active_cluster"`
	AgentsMeta    map[string]AgentMeta `json:"agents_meta"`
	Version       string               `json:"version"`
}

type StubMetaProviderFunc func() StubMeta
