package config

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)

// Config definition
type Config struct {
	KubeConfig    *rest.Config
	KubeClient    *kubernetes.Clientset
	MetricsClient *metrics.Clientset

	Settings ConfigSettings `yaml:"settings" json:"settings"`
	Alarms   ConfigAlarms   `yaml:"alarms" json:"alarms"`
	Links    ConfigLinks    `yaml:"links" json:"links"`
}

// ConfigSettings definition
type ConfigSettings struct {
	APIKey               string            `yaml:"apiKey" json:"apiKey"`
	HttpAuthorizationKey string            `yaml:"httpAuthorizationKey" json:"httpAuthorizationKey"`
	KubeConfig           string            `yaml:"kubeconfig" json:"kubeconfig"`
	Master               string            `yaml:"master" json:"master"`
	Insecure             bool              `yaml:"insecure" json:"insecure"`
	Namespace            string            `yaml:"namespace" json:"namespace"`
	Port                 int               `yaml:"port" json:"port"`
	Log                  ConfigSettingsLog `yaml:"log" json:"log"`
	ElectionID           string            `yaml:"electionID" json:"electionID"`
	CheckInterval        string            `yaml:"checkInterval" json:"checkInterval"`
}

// ConfigSettingsLog definition
type ConfigSettingsLog struct {
	Level string `yaml:"log.level" json:"level"`
	JSON  bool   `yaml:"log.json" json:"json"`
}

// ConfigAlarms definition
type ConfigAlarms struct {
	Cluster ConfigAlarmSetting `yaml:"cluster" json:"cluster"`
	Pods    ConfigAlarmsPods   `yaml:"pods" json:"pods"`
	Nodes   ConfigAlarmsNodes  `yaml:"nodes" json:"nodes"`
}

// ConfigAlarmsPods definition
type ConfigAlarmsPods struct {
	Enabled   bool                            `yaml:"enabled" json:"enabled"`
	Terminate ConfigAlarmSetting              `yaml:"terminate" json:"terminate"`
	Waiting   ConfigAlarmSetting              `yaml:"waiting" json:"waiting"`
	Restarts  ConfigAlarmSettingWithThreshold `yaml:"restarts" json:"restarts"`
	Resources ConfigAlarmSettingResources     `yaml:"resources" json:"resources"`
}

// ConfigAlarmsNodes definition
type ConfigAlarmsNodes struct {
	Enabled   bool                        `yaml:"enabled" json:"enabled"`
	Terminate ConfigAlarmSetting          `yaml:"terminate" json:"terminate"`
	Resources ConfigAlarmSettingResources `yaml:"resources" json:"resources"`
}

// ConfigAlarmSetting definition
type ConfigAlarmSetting struct {
	Enabled         bool     `yaml:"enabled" json:"enabled"`
	Priority        string   `yaml:"priority" json:"priority"`
	ExcludedReasons []string `yaml:"excludedReasons" json:"excludedReasons"`
}

// ConfigAlarmSettingWithThreshold definition
type ConfigAlarmSettingWithThreshold struct {
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	Priority  string `yaml:"priority" json:"priority"`
	Threshold int32  `yaml:"threshold" json:"threshold"`
}

// ConfigAlarmSettingResources definition
type ConfigAlarmSettingResources struct {
	Enabled bool                            `yaml:"enabled" json:"enabled"`
	CPU     ConfigAlarmSettingWithThreshold `yaml:"cpu" json:"cpu"`
	Memory  ConfigAlarmSettingWithThreshold `yaml:"memory" json:"memory"`
}

// ConfigLinks definition
type ConfigLinks struct {
	Pods  []ConfigLinksSetting `yaml:"pods" json:"pods"`
	Nodes []ConfigLinksSetting `yaml:"nodes" json:"nodes"`
}

// ConfigLinksSetting definition
type ConfigLinksSetting struct {
	Name string `yaml:"name" json:"name"`
	Href string `yaml:"href" json:"href"`
}
