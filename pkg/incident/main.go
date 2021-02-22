package incident

import (
	"github.com/rs/zerolog/log"

	"github.com/iLert/ilert-go"
)

var ilertClient *ilert.Client

// CreateEvent creates an incident event
func CreateEvent(ilertAPIKey string, incidentKey string, summary string, details string, eventType string, priority string) {
	if ilertClient == nil {
		ilertClient = ilert.NewClient()
	}

	event := &ilert.Event{
		IncidentKey: incidentKey,
		Summary:     summary,
		Details:     details,
		EventType:   eventType,
		APIKey:      ilertAPIKey,
		Priority:    priority,
	}
	log.Info().Interface("event", event).Msg("Creating incident event")

	output, err := ilertClient.CreateEvent(&ilert.CreateEventInput{
		Event: event,
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to create incident event")
		return
	}

	log.Info().Str("url", output.EventResponse.IncidentURL).Msg("Incident event created")
}
