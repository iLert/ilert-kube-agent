package config

// Config definition
type Config struct {
	Settings ConfigSettings `yaml:"settings"`
	Alarms   ConfigAlarms   `yaml:"alarms"`
	Links    ConfigLinks    `yaml:"links"`
}

// ConfigSettings definition
type ConfigSettings struct {
	APIKey        string            `yaml:"apiKey"`
	KubeConfig    string            `yaml:"kubeconfig"`
	Master        string            `yaml:"master"`
	Namespace     string            `yaml:"namespace"`
	Port          int               `yaml:"port"`
	Log           ConfigSettingsLog `yaml:"log"`
	ElectionID    string            `yaml:"electionID"`
	CheckInterval string            `yaml:"checkInterval"`
}

// ConfigSettingsLog definition
type ConfigSettingsLog struct {
	Level string `yaml:"log.level"`
	JSON  bool   `yaml:"log.json"`
}

// ConfigAlarms definition
type ConfigAlarms struct {
	Pods  ConfigAlarmsPods  `yaml:"pods"`
	Nodes ConfigAlarmsNodes `yaml:"nodes"`
}

// ConfigAlarmsPods definition
type ConfigAlarmsPods struct {
	Enabled   bool                            `yaml:"enabled"`
	Terminate ConfigAlarmSetting              `yaml:"terminate"`
	Waiting   ConfigAlarmSetting              `yaml:"waiting"`
	Restarts  ConfigAlarmSettingWithThreshold `yaml:"restarts"`
	Resources ConfigAlarmSettingWithThreshold `yaml:"resources"`
}

// ConfigAlarmsNodes definition
type ConfigAlarmsNodes struct {
	Enabled   bool                            `yaml:"enabled"`
	Terminate ConfigAlarmSetting              `yaml:"terminate"`
	Resources ConfigAlarmSettingWithThreshold `yaml:"resources"`
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

// ConfigLinks definition
type ConfigLinks struct {
	Pods  ConfigLinksSetting `yaml:"pods"`
	Nodes ConfigLinksSetting `yaml:"nodes"`
}

// ConfigLinksSetting definition
type ConfigLinksSetting struct {
	Metrics string `yaml:"metrics"`
	Logs    string `yaml:"logs"`
}
