package incident

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/iLert/ilert-go"
	shared "github.com/iLert/ilert-kube-agent"
	v1 "github.com/iLert/ilert-kube-agent/pkg/apis/incident/v1"
	agentclientset "github.com/iLert/ilert-kube-agent/pkg/client/clientset/versioned"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/iLert/ilert-kube-agent/pkg/utils"
)

var ilertClient *ilert.Client

// CreateEvent creates an alert event
func CreateEvent(
	cfg *config.Config,
	links []ilert.IncidentLink,
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
		IncidentKey: alertKey,
		Summary:     summary,
		Details:     details,
		EventType:   eventType,
		APIKey:      cfg.Settings.APIKey,
		Priority:    priority,
		Links:       links,
		CustomDetails: map[string]interface{}{
			"labels": labels,
		},
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

// CreateIncidentRef definition
func CreateIncidentRef(agentKubeClient *agentclientset.Clientset, name string, namespace string, incidentID *int64, summary string, details string, incidentType string) {
	if agentKubeClient == nil {
		return
	}
	if incidentID != nil && *incidentID > 0 {
		log.Debug().Int64("incident_id", *incidentID).Str("name", name).Str("namespace", namespace).Msg("Creating incident ref")
		incident := &v1.Incident{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: v1.IncidentSpec{
				ID:      *incidentID,
				Summary: summary,
				Details: details,
				Type:    incidentType,
			},
		}
		_, err := agentKubeClient.IlertV1().Incidents(namespace).Create(incident)
		if err != nil {
			log.Debug().Err(err).Msg("Failed to create incident ref")
		}
	}
}

// GetIncidentRef definition
func GetIncidentRef(agentKubeClient *agentclientset.Clientset, name string, namespace string) *v1.Incident {
	if agentKubeClient == nil {
		return nil
	}
	incident, err := agentKubeClient.IlertV1().Incidents(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		// log.Debug().Err(err).Msg("Failed to get incident ref")
		return nil
	}

	log.Debug().Str("name", name).Str("namespace", namespace).Msg("Got incident ref")

	return incident
}

// DeleteIncidentRef definition
func DeleteIncidentRef(agentKubeClient *agentclientset.Clientset, name string, namespace string) {
	if agentKubeClient == nil {
		return
	}
	log.Debug().Str("name", name).Str("namespace", namespace).Msg("Deleting incident ref")
	err := agentKubeClient.IlertV1().Incidents(namespace).Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		log.Debug().Err(err).Msg("Failed to create incident ref")
	}
}
