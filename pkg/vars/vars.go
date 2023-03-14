package vars

const (
	EnvStubServer = "ALI_STUB_REGISTER_ADDR"

	ClusterID               = "KUBERNETES_CLUSTER_ID"
	LOG_LEVEL               = "LOG_LEVEL"
	AliyunCredentialsFolder = "/ack-credentials"
	TlsCrt                  = "cert"
	TlsKey                  = "key"
	RootCa                  = "ca"
	ConnectToken            = "token"
	ConvertUrl              = "url"
	SECRET_NAME             = "SECRET_NAME"
	PayloadLength           = 8
	AlibabacloudNodeLabel   = "alibabacloud.com/external=true"

	Amazon       = "amazon"
	Alibaba      = "alibaba"
	Azure        = "azure"
	Google       = "google"
	Oracle       = "oracle"
	DigitalOcean = "digitalocean"
	Idc          = "idc"
	Unknown      = ""
	EKS          = "eks"
	TKE          = "tke"
	CCE          = "cce"
)
