package incident

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cbroglie/mustache"
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

// CreateEvent creates an incident event
func CreateEvent(
	cfg *config.Config,
	mustacheValues map[string]string,
	incidentKey string,
	summary string,
	details string,
	eventType string,
	priority string,
) *int64 {
	if ilertClient == nil {
		ilertClient = ilert.NewClient(ilert.WithUserAgent(fmt.Sprintf("ilert-kube-agent/%s", shared.Version)))
	}

	event := &ilert.Event{
		IncidentKey: incidentKey,
		Summary:     summary,
		Details:     details,
		EventType:   eventType,
		APIKey:      cfg.Settings.APIKey,
		Priority:    priority,
	}
	links := make([]ilert.IncidentLink, 0)
	if cfg.Links.Metrics != "" {
		url, err := mustache.Render(cfg.Links.Metrics, mustacheValues)
		if err == nil && url != "" {
			links = append(links, ilert.IncidentLink{
				Href: url,
				Text: "Metrics",
			})
		}
	}
	if cfg.Links.Logs != "" {
		url, err := mustache.Render(cfg.Links.Logs, mustacheValues)
		if err == nil && url != "" {
			links = append(links, ilert.IncidentLink{
				Href: url,
				Text: "Logs",
			})
		}
	}
	event.Links = links
	log.Debug().Interface("event", event).Msg("Creating incident event")

	output, err := ilertClient.CreateEvent(&ilert.CreateEventInput{
		Event: event,
		// URL:   utils.String(fmt.Sprintf("https://api.ilert.com/api/v1/events/kubernetes/%s", ilertAPIKey)),
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to create incident event")
		return nil
	}

	incidentIDStr := strings.ReplaceAll(output.EventResponse.IncidentURL, "https://api.ilert.com/api/v1/incidents/", "")
	incidentID, err := strconv.ParseInt(incidentIDStr, 10, 64)
	if err != nil {
		log.Error().Err(err).Msg("Failed to convert incident id to int64")
		return nil
	}

	log.Info().Int64("incident_id", incidentID).Msg("Incident event created")

	return utils.Int64(incidentID)
}

// CreateIncidentRef definition
func CreateIncidentRef(agentKubeClient *agentclientset.Clientset, name string, namespace string, incidentID *int64, summary string, details string) {
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
	log.Debug().Str("name", name).Str("namespace", namespace).Msg("Deleting incident ref")
	err := agentKubeClient.IlertV1().Incidents(namespace).Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		log.Debug().Err(err).Msg("Failed to create incident ref")
	}
}
