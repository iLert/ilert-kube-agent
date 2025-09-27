package alert

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/iLert/ilert-go/v3"
	shared "github.com/iLert/ilert-kube-agent"
	"github.com/iLert/ilert-kube-agent/pkg/cache"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/iLert/ilert-kube-agent/pkg/utils"
)

var ilertClient *ilert.Client

const alertEventRateLimitPerMinute = 1
const resolveEventRateLimitPer30Minute = 1

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

	limitKey := fmt.Sprintf("%s:%s", alertKey, eventType)
	currentRate, err := cache.Cache.Events.GetInt64Item(limitKey)
	if err != nil {
		log.Warn().Err(err).Str("limit_key", limitKey).Msg("Failed to get current rate for alert key")
		currentRate = 0
	}

	if eventType == ilert.EventTypes.Resolve && currentRate >= resolveEventRateLimitPer30Minute {
		log.Warn().
			Int64("current_rate", currentRate).
			Str("limit_key", limitKey).
			Msg("Current rate is greater than the resolve event rate limit, skipping resolve event")
		return nil
	} else if currentRate >= alertEventRateLimitPerMinute {
		log.Warn().
			Int64("current_rate", currentRate).
			Str("limit_key", limitKey).
			Msg("Current rate is greater than the alert event rate limit, skipping alert event")
		return nil
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

	_, err = ilertClient.CreateEvent(&ilert.CreateEventInput{
		Event: event,
		URL:   utils.String(fmt.Sprintf("https://api.ilert.com/api/v1/events/kubernetes/%s", cfg.Settings.APIKey)),
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to create alert event")
		return err
	}

	if eventType == ilert.EventTypes.Alert {
		cache.Cache.Events.IncrementItemBy(limitKey, 1, time.Minute*1)
	} else if eventType == ilert.EventTypes.Resolve {
		cache.Cache.Events.IncrementItemBy(limitKey, 1, time.Minute*30)
	}

	log.Info().Str("summary", summary).Str("alert_key", alertKey).Msg("Alert event created")

	return nil
}
