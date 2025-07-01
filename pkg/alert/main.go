package alert

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/iLert/ilert-go/v3"
	shared "github.com/iLert/ilert-kube-agent"
	v1 "github.com/iLert/ilert-kube-agent/pkg/apis/alert/v1"
	agentclientset "github.com/iLert/ilert-kube-agent/pkg/client/clientset/versioned"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/iLert/ilert-kube-agent/pkg/utils"
)

var ilertClient *ilert.Client

// CreateEvent creates an alert event
func CreateEvent(
	cfg *config.Config,
	links []ilert.AlertLink,
	alertKey string,
	summary string,
	details string,
	eventType string,
	priority string,
	labels map[string]string,
) error {
	if ilertClient == nil {
		ilertClient = ilert.NewClient(ilert.WithUserAgent(fmt.Sprintf("ilert-kube-agent/%s", shared.Version)))
	}

	if cfg.Settings.APIKey == "" {
		log.Error().Msg("Failed to create an alert event. API key is required")
		return errors.New("Failed to create an alert event. API key is required")
	}

	event := &ilert.Event{
		AlertKey:  alertKey,
		Summary:   summary,
		Details:   details,
		EventType: eventType,
		APIKey:    cfg.Settings.APIKey,
		Priority:  priority,
		Links:     links,
		Labels:    labels,
	}

	log.Debug().Interface("event", event).Msg("Creating alert event")

	_, err := ilertClient.CreateEvent(&ilert.CreateEventInput{
		Event: event,
		URL:   utils.String(fmt.Sprintf("https://api.ilert.com/api/v1/events/kubernetes/%s", cfg.Settings.APIKey)),
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to create alert event")
		return err
	}

	log.Info().Str("summary", summary).Str("alert_key", alertKey).Msg("Alert event created")

	return nil
}

// CreateAlertRef definition
func CreateAlertRef(agentKubeClient *agentclientset.Clientset, name string, namespace string, alertID *int64, summary string, details string, alertType string) {
	if agentKubeClient == nil {
		return
	}
	if alertID != nil && *alertID > 0 {
		log.Debug().Int64("alert_id", *alertID).Str("name", name).Str("namespace", namespace).Msg("Creating alert ref")
		alert := &v1.Alert{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: v1.AlertSpec{
				ID:      *alertID,
				Summary: summary,
				Details: details,
				Type:    alertType,
			},
		}
		_, err := agentKubeClient.IlertV1().Alerts(namespace).Create(alert)
		if err != nil {
			log.Debug().Err(err).Msg("Failed to create alert ref")
		}
	}
}

// GetAlertRef definition
func GetAlertRef(agentKubeClient *agentclientset.Clientset, name string, namespace string) *v1.Alert {
	if agentKubeClient == nil {
		return nil
	}
	alert, err := agentKubeClient.IlertV1().Alerts(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		// log.Debug().Err(err).Msg("Failed to get alert ref")
		return nil
	}

	log.Debug().Str("name", name).Str("namespace", namespace).Msg("Got alert ref")

	return alert
}

// DeleteAlertRef definition
func DeleteAlertRef(agentKubeClient *agentclientset.Clientset, name string, namespace string) {
	if agentKubeClient == nil {
		return
	}
	log.Debug().Str("name", name).Str("namespace", namespace).Msg("Deleting alert ref")
	err := agentKubeClient.IlertV1().Alerts(namespace).Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		log.Debug().Err(err).Msg("Failed to create alert ref")
	}
}
