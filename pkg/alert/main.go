package alert

import (
	"errors"
	"fmt"
	"strings"
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
	alertKey string,
	summary string,
	details string,
	eventType string,
	priority string,
	labels map[string]string,
	links []ilert.AlertLink,
	logs []ilert.EventLog,
) error {
	if ilertClient == nil {
		ilertClient = ilert.NewClient(ilert.WithUserAgent(fmt.Sprintf("ilert-kube-agent/%s", shared.Version)))
	}

	if cfg.Settings.APIKey == "" {
		log.Error().Msg("Failed to create an alert event. API key is required")
		return errors.New("Failed to create an alert event. API key is required")
	}

	// Split API keys by comma and trim whitespace
	apiKeys := strings.Split(cfg.Settings.APIKey, ",")
	for i, key := range apiKeys {
		apiKeys[i] = strings.TrimSpace(key)
	}

	var lastError error
	successCount := 0

	// Process each API key
	for _, apiKey := range apiKeys {
		if apiKey == "" {
			log.Warn().Msg("Skipping empty API key")
			continue
		}

		// Create rate limiting key per API key
		limitKey := fmt.Sprintf("%s:%s:%s", alertKey, eventType, apiKey)
		resolveLimitKey := fmt.Sprintf("%s:%s:%s", alertKey, ilert.EventTypes.Resolve, apiKey)

		currentRate, err := cache.Cache.Events.GetInt64Item(limitKey)
		if err != nil {
			log.Debug().Err(err).Str("limit_key", limitKey).Str("api_key", apiKey).Msg("Failed to get current rate for alert key")
			currentRate = 0
		}

		if eventType == ilert.EventTypes.Resolve && currentRate >= resolveEventRateLimitPer30Minute {
			log.Debug().
				Int64("current_rate", currentRate).
				Str("limit_key", limitKey).
				Str("api_key", apiKey).
				Msg("Current rate is greater than the resolve event rate limit, skipping resolve event")
			continue
		} else if currentRate >= alertEventRateLimitPerMinute {
			log.Debug().
				Int64("current_rate", currentRate).
				Str("limit_key", limitKey).
				Str("api_key", apiKey).
				Msg("Current rate is greater than the alert event rate limit, skipping alert event")
			continue
		}

		event := &ilert.Event{
			AlertKey:  alertKey,
			Summary:   summary,
			Details:   details,
			EventType: eventType,
			APIKey:    apiKey,
			Priority:  priority,
			Links:     links,
			Labels:    labels,
			Logs:      logs,
		}

		log.Debug().Interface("event", event).Str("api_key", apiKey).Msg("Creating alert event")

		_, err = ilertClient.CreateEvent(&ilert.CreateEventInput{
			Event: event,
			URL:   utils.String(fmt.Sprintf("https://api.ilert.com/api/v1/events/kubernetes/%s", apiKey)),
		})

		if err != nil {
			log.Error().Err(err).Str("api_key", apiKey).Msg("Failed to create alert event")
			lastError = err
			continue
		}

		// Update rate limiting per API key
		if eventType == ilert.EventTypes.Alert {
			cache.Cache.Events.IncrementItemBy(limitKey, 1, time.Minute*1)
			cache.Cache.Events.SetInt64Item(resolveLimitKey, 0, time.Minute*30)
		} else if eventType == ilert.EventTypes.Resolve {
			cache.Cache.Events.IncrementItemBy(limitKey, 1, time.Minute*30)
		}

		successCount++
		log.Info().Str("summary", summary).Str("alert_key", alertKey).Str("api_key", apiKey).Msg("Alert event created")
	}

	// Return error only if all API keys failed
	if successCount == 0 && lastError != nil {
		return lastError
	}

	// Log summary of results
	if successCount > 0 {
		log.Info().
			Int("success_count", successCount).
			Int("total_api_keys", len(apiKeys)).
			Str("alert_key", alertKey).
			Msg("Alert events processed")
	}

	return nil
}
