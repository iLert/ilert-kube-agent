package config

// Config definition
type Config struct {
	APIKey                           string
	EnablePodAlarms                  bool
	EnablePodTerminateAlarms         bool
	EnablePodWaitingAlarms           bool
	EnablePodRestartsAlarms          bool
	EnableNodeAlarms                 bool
	EnableResourcesAlarms            bool
	PodRestartThreshold              int32
	PodAlarmIncidentPriority         string
	PodRestartsAlarmIncidentPriority string
	NodeAlarmIncidentPriority        string
	ResourcesAlarmIncidentPriority   string
	ResourcesCheckerInterval         int32
	ResourcesThreshold               int32
}
