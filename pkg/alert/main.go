package alert

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/iLert/ilert-go/v3"
	shared "github.com/iLert/ilert-kube-agent"
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
