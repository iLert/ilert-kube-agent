package config

// GetDefaultConfig returns default config
func GetDefaultConfig() *Config {
	return &Config{
		Settings: ConfigSettings{
			ElectionID:    "ilert-kube-agent",
			Namespace:     "kube-system",
			Port:          9092,
			CheckInterval: "15s",
			Log: ConfigSettingsLog{
				JSON:  false,
				Level: "info",
			},
		},
		Alarms: ConfigAlarms{
			Cluster: ConfigAlarmSetting{
				Enabled:  true,
				Priority: "HIGH",
			},
			Pods: ConfigAlarmsPods{
				Enabled: true,
				Terminate: ConfigAlarmSetting{
					Enabled:  true,
					Priority: "HIGH",
				},
				Waiting: ConfigAlarmSetting{
					Enabled:  true,
					Priority: "LOW",
				},
				Restarts: ConfigAlarmSettingWithThreshold{
					Enabled:   true,
					Priority:  "LOW",
					Threshold: 10,
				},
				Resources: ConfigAlarmSettingResources{
					Enabled: true,
					CPU: ConfigAlarmSettingWithThreshold{
						Enabled:   true,
						Priority:  "LOW",
						Threshold: 90,
					},
					Memory: ConfigAlarmSettingWithThreshold{
						Enabled:   true,
						Priority:  "LOW",
						Threshold: 90,
					},
				},
			},
			Nodes: ConfigAlarmsNodes{
				Enabled: true,
				Terminate: ConfigAlarmSetting{
					Enabled:  true,
					Priority: "HIGH",
				},
				Resources: ConfigAlarmSettingResources{
					Enabled: true,
					CPU: ConfigAlarmSettingWithThreshold{
						Enabled:   true,
						Priority:  "LOW",
						Threshold: 90,
					},
					Memory: ConfigAlarmSettingWithThreshold{
						Enabled:   true,
						Priority:  "LOW",
						Threshold: 90,
					},
				},
			},
		},
	}
}
