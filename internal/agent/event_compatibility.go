package agent

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// UniversalEvent represents the event format sent by any EZLogs SDK (Ruby, Python, Java, etc.)
// Using a flexible map-based approach to handle any fields from the Ruby agent exactly as they come
// Expected structure based on Ruby agent:
//
//	{
//	  event_id: string,
//	  event_type: string,
//	  action: string,
//	  actor: object,
//	  subject: object,
//	  correlation: object,
//	  correlation_context: object,
//	  payload: object,
//	  metadata: object,
//	  timing: object,
//	  platform: object,
//	  environment: object,
//	  impact: object
//	}
type UniversalEvent map[string]interface{}

// Helper methods to safely access common fields
func (e UniversalEvent) GetEventID() string {
	if v, ok := e["event_id"].(string); ok {
		return v
	}
	return ""
}

func (e UniversalEvent) GetEventType() string {
	if v, ok := e["event_type"].(string); ok {
		return v
	}
	return ""
}

func (e UniversalEvent) GetAction() string {
	if v, ok := e["action"].(string); ok {
		return v
	}
	return ""
}

func (e UniversalEvent) GetActor() map[string]interface{} {
	if v, ok := e["actor"].(map[string]interface{}); ok {
		return v
	}
	return nil
}

func (e UniversalEvent) GetSubject() map[string]interface{} {
	if v, ok := e["subject"].(map[string]interface{}); ok {
		return v
	}
	return nil
}

func (e UniversalEvent) GetCorrelation() map[string]interface{} {
	if v, ok := e["correlation"].(map[string]interface{}); ok {
		return v
	}
	return nil
}

func (e UniversalEvent) GetCorrelationContext() map[string]interface{} {
	if v, ok := e["correlation_context"].(map[string]interface{}); ok {
		return v
	}
	return nil
}

func (e UniversalEvent) GetPayload() map[string]interface{} {
	if v, ok := e["payload"].(map[string]interface{}); ok {
		return v
	}
	return nil
}

func (e UniversalEvent) GetMetadata() map[string]interface{} {
	if v, ok := e["metadata"].(map[string]interface{}); ok {
		return v
	}
	return nil
}

func (e UniversalEvent) GetTiming() map[string]interface{} {
	if v, ok := e["timing"].(map[string]interface{}); ok {
		return v
	}
	return nil
}

func (e UniversalEvent) GetPlatform() map[string]interface{} {
	if v, ok := e["platform"].(map[string]interface{}); ok {
		return v
	}
	return nil
}

func (e UniversalEvent) GetEnvironment() map[string]interface{} {
	if v, ok := e["environment"].(map[string]interface{}); ok {
		return v
	}
	return nil
}

func (e UniversalEvent) GetImpact() map[string]interface{} {
	if v, ok := e["impact"].(map[string]interface{}); ok {
		return v
	}
	return nil
}

// Legacy helper for backward compatibility - checks both timestamp and timing fields
func (e UniversalEvent) GetTimestamp() string {
	// First check for direct timestamp field (legacy)
	if v, ok := e["timestamp"].(string); ok {
		return v
	}

	// Then check timing.timestamp (new structure)
	if timing := e.GetTiming(); timing != nil {
		if v, ok := timing["timestamp"].(string); ok {
			return v
		}
	}

	return ""
}

// Legacy helper for backward compatibility - checks both correlation_id and correlation fields
func (e UniversalEvent) GetCorrelationID() string {
	// First check for direct correlation_id field (legacy)
	if v, ok := e["correlation_id"].(string); ok {
		return v
	}

	// Then check correlation.id (new structure)
	if correlation := e.GetCorrelation(); correlation != nil {
		if v, ok := correlation["id"].(string); ok {
			return v
		}
	}

	return ""
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

// ValidateProtocol checks if the payload is a valid UniversalEvent batch
func (ec *EventCompatibility) ValidateProtocol(data []byte) error {
	// Try to parse as array first
	var arr []map[string]interface{}
	if err := json.Unmarshal(data, &arr); err != nil {
		// If array parsing fails, try single object
		var single map[string]interface{}
		if err2 := json.Unmarshal(data, &single); err2 != nil {
			return fmt.Errorf("invalid JSON (neither array nor object): array_error=%v, object_error=%v", err, err2)
		}
		// Single object is valid, we'll wrap it in an array during parsing
	}

	// Additional validation can be added here
	return nil
}

// ParseBatch parses a batch of UniversalEvents
func (ec *EventCompatibility) ParseBatch(data []byte) ([]UniversalEvent, error) {
	// Try to parse as array first
	var arr []UniversalEvent
	if err := json.Unmarshal(data, &arr); err != nil {
		// If array parsing fails, try single object
		var single UniversalEvent
		if err2 := json.Unmarshal(data, &single); err2 != nil {
			return nil, fmt.Errorf("invalid UniversalEvent batch: array_error=%v, object_error=%v", err, err2)
		}
		// Wrap single object in array
		arr = []UniversalEvent{single}
	}

	if len(arr) == 0 {
		return nil, fmt.Errorf("empty event batch")
	}

	if ec.validate {
		for i, evt := range arr {
			if err := ec.validateEvent(evt); err != nil {
				logrus.WithFields(logrus.Fields{
					"event_index": i,
					"event":       evt,
					"error":       err,
				}).Warn("Event validation failed, but continuing processing")
				// Don't fail the entire batch for validation errors
				// Just log and continue to be more resilient
			}
		}
	}

	return arr, nil
}

// validateEvent checks required fields and formats (more lenient now)
func (ec *EventCompatibility) validateEvent(evt UniversalEvent) error {
	// Only require event_type as the absolute minimum
	eventType := evt.GetEventType()
	if eventType == "" {
		return fmt.Errorf("event_type is required")
	}

	// Validate event_type format if present
	if !ec.isValidEventType(eventType) {
		return fmt.Errorf("event_type should be in format 'namespace.category', got: %s", eventType)
	}

	// Action is highly recommended but not strictly required for flexibility
	if evt.GetAction() == "" {
		logrus.WithField("event_type", eventType).Debug("Event missing action field (recommended)")
	}

	// Actor is recommended but not required for maximum flexibility
	actor := evt.GetActor()
	if actor == nil {
		logrus.WithField("event_type", eventType).Debug("Event missing actor field (recommended)")
	} else {
		// If actor is present, validate its structure
		if _, ok := actor["type"]; !ok {
			logrus.WithField("event_type", eventType).Debug("Actor missing type field (recommended)")
		}
		if _, ok := actor["id"]; !ok {
			logrus.WithField("event_type", eventType).Debug("Actor missing id field (recommended)")
		}
	}

	// Validate timestamp format if present
	timestamp := evt.GetTimestamp()
	if timestamp != "" {
		if _, err := time.Parse(time.RFC3339, timestamp); err != nil {
			return fmt.Errorf("invalid timestamp format (should be RFC3339): %w", err)
		}
	}

	return nil
}

// isValidEventType checks for 'namespace.category' format
func (ec *EventCompatibility) isValidEventType(eventType string) bool {
	// Simple check: must contain one dot, no spaces, minimum length
	if len(eventType) < 3 || eventType[0] == '.' || eventType[len(eventType)-1] == '.' {
		return false
	}

	// Check for spaces (not allowed)
	for _, c := range eventType {
		if c == ' ' {
			return false
		}
	}

	// Count dots - should have exactly one
	count := 0
	for _, c := range eventType {
		if c == '.' {
			count++
		}
	}
	return count == 1
}

// TransformToCollectorFormat transforms UniversalEvents to the collector's expected format
// This is a pass-through approach that preserves ALL fields exactly as received
func (ec *EventCompatibility) TransformToCollectorFormat(events []UniversalEvent) []json.RawMessage {
	var out []json.RawMessage
	for _, evt := range events {
		// Start with a copy of the original event to preserve all fields exactly as they came
		m := make(map[string]interface{})
		for k, v := range evt {
			m[k] = v
		}

		// Add agent information if transformation is enabled
		if ec.transform {
			// Add agent metadata without overriding existing fields
			if _, exists := m["agent"]; !exists {
				m["agent"] = map[string]interface{}{
					"type":    "go",
					"version": "1.0.0",
					"name":    "ezlogs-go-agent",
				}
			}

			// Add processing timestamp if not present
			if _, exists := m["processed_at"]; !exists {
				m["processed_at"] = time.Now().UTC().Format(time.RFC3339)
			}
		}

		data, err := json.Marshal(m)
		if err != nil {
			logrus.WithError(err).WithField("event", evt).Warn("Failed to marshal event for collector")
			continue
		}
		out = append(out, data)
	}

	logrus.WithFields(logrus.Fields{
		"input_events":  len(events),
		"output_events": len(out),
		"transform":     ec.transform,
	}).Debug("Transformed events for collector")

	return out
}
