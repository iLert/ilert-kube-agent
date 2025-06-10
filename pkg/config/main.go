package config

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"

	agentclientset "github.com/iLert/ilert-kube-agent/pkg/client/clientset/versioned"
)

// Config definition
type Config struct {
	KubeConfig      *rest.Config
	KubeClient      *kubernetes.Clientset
	AgentKubeClient *agentclientset.Clientset
	MetricsClient   *metrics.Clientset

	Settings ConfigSettings `yaml:"settings"`
	Alarms   ConfigAlarms   `yaml:"alarms"`
	Links    ConfigLinks    `yaml:"links"`
}

// ConfigSettings definition
type ConfigSettings struct {
	APIKey           string            `yaml:"apiKey"`
	AuthorizationKey string            `yaml:"authorizationKey"`
	KubeConfig       string            `yaml:"kubeconfig"`
	Master           string            `yaml:"master"`
	Insecure         bool              `yaml:"insecure"`
	Namespace        string            `yaml:"namespace"`
	Port             int               `yaml:"port"`
	Log              ConfigSettingsLog `yaml:"log"`
	ElectionID       string            `yaml:"electionID"`
	CheckInterval    string            `yaml:"checkInterval"`
}

// ConfigSettingsLog definition
type ConfigSettingsLog struct {
	Level string `yaml:"log.level"`
	JSON  bool   `yaml:"log.json"`
}

// ConfigAlarms definition
type ConfigAlarms struct {
	Cluster ConfigAlarmSetting `yaml:"cluster"`
	Pods    ConfigAlarmsPods   `yaml:"pods"`
	Nodes   ConfigAlarmsNodes  `yaml:"nodes"`
}

// ConfigAlarmsPods definition
type ConfigAlarmsPods struct {
	Enabled   bool                            `yaml:"enabled"`
	Terminate ConfigAlarmSetting              `yaml:"terminate"`
	Waiting   ConfigAlarmSetting              `yaml:"waiting"`
	Restarts  ConfigAlarmSettingWithThreshold `yaml:"restarts"`
	Resources ConfigAlarmSettingResources     `yaml:"resources"`
}

// ConfigAlarmsNodes definition
type ConfigAlarmsNodes struct {
	Enabled   bool                        `yaml:"enabled"`
	Terminate ConfigAlarmSetting          `yaml:"terminate"`
	Resources ConfigAlarmSettingResources `yaml:"resources"`
}

// ConfigAlarmSetting definition
type ConfigAlarmSetting struct {
	Enabled  bool   `yaml:"enabled"`
	Priority string `yaml:"priority"`
}

// ConfigAlarmSettingWithThreshold definition
type ConfigAlarmSettingWithThreshold struct {
	Enabled   bool   `yaml:"enabled"`
	Priority  string `yaml:"priority"`
	Threshold int32  `yaml:"priority"`
}

// ConfigAlarmSettingResources definition
type ConfigAlarmSettingResources struct {
	Enabled bool                            `yaml:"enabled"`
	CPU     ConfigAlarmSettingWithThreshold `yaml:"cpu"`
	Memory  ConfigAlarmSettingWithThreshold `yaml:"memory"`
}

// ConfigLinks definition
type ConfigLinks struct {
	Pods  []ConfigLinksSetting `yaml:"pods"`
	Nodes []ConfigLinksSetting `yaml:"nodes"`
}

// ConfigLinksSetting definition
type ConfigLinksSetting struct {
	Name string `yaml:"name"`
	Href string `yaml:"href"`
}
