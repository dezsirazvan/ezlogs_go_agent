package agent

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// UniversalEvent represents the event format sent by any EZLogs SDK (Ruby, Python, Java, etc.)
type UniversalEvent struct {
	EventType     string                 `json:"event_type"`
	Action        string                 `json:"action"`
	Actor         map[string]interface{} `json:"actor"`
	Subject       map[string]interface{} `json:"subject,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Timestamp     string                 `json:"timestamp,omitempty"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
	ServiceName   string                 `json:"service_name,omitempty"`
	Environment   string                 `json:"environment,omitempty"`
}

// EventCompatibility provides validation and transformation for UniversalEvent batches
// from any supported SDK.
type EventCompatibility struct {
	validate  bool
	transform bool
}

// NewEventCompatibility creates a new event compatibility layer
func NewEventCompatibility(validate, transform bool) *EventCompatibility {
	return &EventCompatibility{validate: validate, transform: transform}
}

// ValidateProtocol checks if the payload is a valid UniversalEvent batch or single event
func (ec *EventCompatibility) ValidateProtocol(data []byte) error {
	// Try to unmarshal as array first (batch)
	var arr []map[string]interface{}
	if err := json.Unmarshal(data, &arr); err == nil {
		if len(arr) == 0 {
			return fmt.Errorf("empty event batch")
		}
		return nil
	}

	// If array parsing fails, try single event
	var single map[string]interface{}
	if err := json.Unmarshal(data, &single); err != nil {
		return fmt.Errorf("invalid JSON format - not an array or single event: %w", err)
	}

	return nil
}

// ParseBatch parses a batch of UniversalEvents or a single event
func (ec *EventCompatibility) ParseBatch(data []byte) ([]UniversalEvent, error) {
	// Try to parse as array first (batch format)
	var arr []UniversalEvent
	if err := json.Unmarshal(data, &arr); err == nil {
		if ec.validate {
			for i, evt := range arr {
				if err := ec.validateEvent(evt); err != nil {
					return nil, fmt.Errorf("event %d validation failed: %w", i, err)
				}
			}
		}
		return arr, nil
	}

	// If array parsing fails, try single event
	var single UniversalEvent
	if err := json.Unmarshal(data, &single); err != nil {
		return nil, fmt.Errorf("invalid JSON format - not an array or single UniversalEvent: %w", err)
	}

	if ec.validate {
		if err := ec.validateEvent(single); err != nil {
			return nil, fmt.Errorf("event validation failed: %w", err)
		}
	}

	// Return single event as array with one element
	return []UniversalEvent{single}, nil
}

// validateEvent checks required fields and formats
func (ec *EventCompatibility) validateEvent(evt UniversalEvent) error {
	if evt.EventType == "" {
		return fmt.Errorf("event_type is required")
	}
	// Make event_type validation more flexible - allow simple names for Ruby agent compatibility
	if len(evt.EventType) < 1 {
		return fmt.Errorf("event_type cannot be empty")
	}
	if evt.Action == "" {
		return fmt.Errorf("action is required")
	}
	// Make actor optional for broader SDK compatibility
	if evt.Actor != nil {
		if _, ok := evt.Actor["type"]; !ok {
			return fmt.Errorf("actor.type is required when actor is present")
		}
		if _, ok := evt.Actor["id"]; !ok {
			return fmt.Errorf("actor.id is required when actor is present")
		}
	}
	if evt.Timestamp != "" {
		if _, err := time.Parse(time.RFC3339, evt.Timestamp); err != nil {
			// Try alternative timestamp formats for compatibility
			if _, err2 := time.Parse("2006-01-02T15:04:05Z07:00", evt.Timestamp); err2 != nil {
				if _, err3 := time.Parse("2006-01-02 15:04:05", evt.Timestamp); err3 != nil {
					return fmt.Errorf("invalid timestamp format (use RFC3339): %w", err)
				}
			}
		}
	}
	return nil
}

// isValidEventType checks for 'namespace.category' format
func (ec *EventCompatibility) isValidEventType(eventType string) bool {
	// Simple check: must contain one dot, no spaces
	if len(eventType) < 3 || eventType[0] == '.' || eventType[len(eventType)-1] == '.' {
		return false
	}
	count := 0
	for _, c := range eventType {
		if c == '.' {
			count++
		}
	}
	return count == 1
}

// TransformToCollectorFormat transforms UniversalEvents to the collector's expected format
func (ec *EventCompatibility) TransformToCollectorFormat(events []UniversalEvent) []json.RawMessage {
	var out []json.RawMessage
	for _, evt := range events {
		m := map[string]interface{}{
			"event_type":     evt.EventType,
			"action":         evt.Action,
			"actor":          evt.Actor,
			"subject":        evt.Subject,
			"metadata":       evt.Metadata,
			"timestamp":      evt.Timestamp,
			"correlation_id": evt.CorrelationID,
			"service_name":   evt.ServiceName,
			"environment":    evt.Environment,
		}
		if ec.transform {
			m["agent"] = map[string]interface{}{
				"type":    "go",
				"version": "1.0.0",
			}
		}
		data, err := json.Marshal(m)
		if err != nil {
			logrus.WithError(err).Warn("Failed to marshal event for collector")
			continue
		}
		out = append(out, data)
	}
	return out
}
