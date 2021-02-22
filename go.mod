module github.com/iLert/ilert-kube-agent

go 1.15

require (
	github.com/dustin/go-humanize v1.0.0
	github.com/gin-contrib/logger v0.0.2
	github.com/gin-contrib/size v0.0.0-20200916080119-37b334d93b20
	github.com/gin-gonic/gin v1.6.3
	github.com/iLert/ilert-go v1.2.0
	github.com/prometheus/client_golang v1.9.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/rs/zerolog v1.20.0
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.17.7
	k8s.io/apimachinery v0.17.7
	k8s.io/client-go v0.17.7
	k8s.io/klog/v2 v2.5.0
	k8s.io/metrics v0.17.7
)
