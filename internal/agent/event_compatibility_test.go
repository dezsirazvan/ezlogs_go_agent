package agent

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventCompatibility_ParseBatch(t *testing.T) {
	ec := NewEventCompatibility(true, true)

	// Valid UniversalEvent batch
	validBatch := `[
		{
			"event_type": "user.action",
			"action": "login",
			"actor": {"type": "user", "id": "123"},
			"subject": {"type": "session", "id": "456"},
			"metadata": {"ip": "192.168.1.1"},
			"timestamp": "2023-01-01T12:00:00Z",
			"correlation_id": "corr_abc123",
			"service_name": "my-app",
			"environment": "production"
		}
	]`

	events, err := ec.ParseBatch([]byte(validBatch))
	require.NoError(t, err)
	assert.Len(t, events, 1)

	event := events[0]
	assert.Equal(t, "user.action", event.EventType)
	assert.Equal(t, "login", event.Action)
	assert.Equal(t, "user", event.Actor["type"])
	assert.Equal(t, "123", event.Actor["id"])
	assert.Equal(t, "session", event.Subject["type"])
	assert.Equal(t, "456", event.Subject["id"])
	assert.Equal(t, "192.168.1.1", event.Metadata["ip"])
	assert.Equal(t, "2023-01-01T12:00:00Z", event.Timestamp)
	assert.Equal(t, "corr_abc123", event.CorrelationID)
	assert.Equal(t, "my-app", event.ServiceName)
	assert.Equal(t, "production", event.Environment)
}

func TestEventCompatibility_ValidateEvent(t *testing.T) {
	ec := NewEventCompatibility(true, true)

	// Valid event
	validEvent := UniversalEvent{
		EventType: "user.action",
		Action:    "login",
		Actor:     map[string]interface{}{"type": "user", "id": "123"},
	}

	err := ec.validateEvent(validEvent)
	assert.NoError(t, err)

	// Invalid event - missing event_type
	invalidEvent := UniversalEvent{
		Action: "login",
		Actor:  map[string]interface{}{"type": "user", "id": "123"},
	}

	err = ec.validateEvent(invalidEvent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "event_type is required")

	// Invalid event - missing action
	invalidEvent = UniversalEvent{
		EventType: "user.action",
		Actor:     map[string]interface{}{"type": "user", "id": "123"},
	}

	err = ec.validateEvent(invalidEvent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "action is required")

	// Invalid event - missing actor
	invalidEvent = UniversalEvent{
		EventType: "user.action",
		Action:    "login",
	}

	err = ec.validateEvent(invalidEvent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "actor is required")

	// Invalid event - missing actor.type
	invalidEvent = UniversalEvent{
		EventType: "user.action",
		Action:    "login",
		Actor:     map[string]interface{}{"id": "123"},
	}

	err = ec.validateEvent(invalidEvent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "actor.type is required")

	// Invalid event - missing actor.id
	invalidEvent = UniversalEvent{
		EventType: "user.action",
		Action:    "login",
		Actor:     map[string]interface{}{"type": "user"},
	}

	err = ec.validateEvent(invalidEvent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "actor.id is required")

	// Invalid event - wrong event_type format
	invalidEvent = UniversalEvent{
		EventType: "invalid",
		Action:    "login",
		Actor:     map[string]interface{}{"type": "user", "id": "123"},
	}

	err = ec.validateEvent(invalidEvent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "event_type must be in format 'namespace.category'")

	// Invalid event - invalid timestamp
	invalidEvent = UniversalEvent{
		EventType: "user.action",
		Action:    "login",
		Actor:     map[string]interface{}{"type": "user", "id": "123"},
		Timestamp: "invalid-timestamp",
	}

	err = ec.validateEvent(invalidEvent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid timestamp format")
}

func TestEventCompatibility_TransformToCollectorFormat(t *testing.T) {
	ec := NewEventCompatibility(true, true)

	events := []UniversalEvent{
		{
			EventType:     "user.action",
			Action:        "login",
			Actor:         map[string]interface{}{"type": "user", "id": "123"},
			Subject:       map[string]interface{}{"type": "session", "id": "456"},
			Metadata:      map[string]interface{}{"ip": "192.168.1.1"},
			Timestamp:     "2023-01-01T12:00:00Z",
			CorrelationID: "corr_abc123",
			ServiceName:   "my-app",
			Environment:   "production",
		},
	}

	transformed := ec.TransformToCollectorFormat(events)
	assert.Len(t, transformed, 1)

	// Parse the transformed event
	var transformedEvent map[string]interface{}
	err := json.Unmarshal(transformed[0], &transformedEvent)
	require.NoError(t, err)

	// Check that all fields are present
	assert.Equal(t, "user.action", transformedEvent["event_type"])
	assert.Equal(t, "login", transformedEvent["action"])
	assert.Equal(t, "2023-01-01T12:00:00Z", transformedEvent["timestamp"])
	assert.Equal(t, "corr_abc123", transformedEvent["correlation_id"])
	assert.Equal(t, "my-app", transformedEvent["service_name"])
	assert.Equal(t, "production", transformedEvent["environment"])

	// Check that agent metadata was added
	agent, ok := transformedEvent["agent"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "go", agent["type"])
	assert.Equal(t, "1.0.0", agent["version"])

	// Check actor and subject
	actor, ok := transformedEvent["actor"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "user", actor["type"])
	assert.Equal(t, "123", actor["id"])

	subject, ok := transformedEvent["subject"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "session", subject["type"])
	assert.Equal(t, "456", subject["id"])
}

func TestEventCompatibility_ValidateProtocol(t *testing.T) {
	ec := NewEventCompatibility(true, true)

	// Valid protocol
	validData := `[{"event_type": "test.event", "action": "test", "actor": {"type": "user", "id": "123"}}]`
	err := ec.ValidateProtocol([]byte(validData))
	assert.NoError(t, err)

	// Invalid JSON
	invalidJSON := `[{"invalid": json}]`
	err = ec.ValidateProtocol([]byte(invalidJSON))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON array")

	// Empty array
	emptyArray := `[]`
	err = ec.ValidateProtocol([]byte(emptyArray))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty event batch")

	// Not an array
	notArray := `{"event_type": "test.event"}`
	err = ec.ValidateProtocol([]byte(notArray))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON array")

	// Array with invalid object
	invalidObject := `[{"event_type": "test.event"}]`
	err = ec.ValidateProtocol([]byte(invalidObject))
	assert.NoError(t, err) // This should pass protocol validation
}

func TestEventCompatibility_IsValidEventType(t *testing.T) {
	ec := NewEventCompatibility(true, true)

	// Valid event types
	validTypes := []string{
		"user.action",
		"http.request",
		"db.query",
		"job.process",
		"api.call",
	}

	for _, eventType := range validTypes {
		assert.True(t, ec.isValidEventType(eventType), "Event type should be valid: %s", eventType)
	}

	// Invalid event types
	invalidTypes := []string{
		"invalid",
		"user",
		"action",
		"user-action",
		"user_action",
		".action",
		"user.",
		"",
	}

	for _, eventType := range invalidTypes {
		assert.False(t, ec.isValidEventType(eventType), "Event type should be invalid: %s", eventType)
	}
}

func TestEventCompatibility_WithoutValidation(t *testing.T) {
	ec := NewEventCompatibility(false, false)

	// Should not validate events
	invalidEvent := UniversalEvent{
		EventType: "invalid",
		Action:    "login",
		Actor:     map[string]interface{}{"type": "user", "id": "123"},
	}

	events := []UniversalEvent{invalidEvent}
	_, err := ec.ParseBatch([]byte(`[{"event_type": "invalid", "action": "login", "actor": {"type": "user", "id": "123"}}]`))
	assert.NoError(t, err) // Should not validate when validation is disabled

	// Should not transform events
	transformed := ec.TransformToCollectorFormat(events)
	assert.Len(t, transformed, 1)

	var transformedEvent map[string]interface{}
	err = json.Unmarshal(transformed[0], &transformedEvent)
	require.NoError(t, err)

	// Should not have agent metadata when transformation is disabled
	_, hasAgent := transformedEvent["agent"]
	assert.False(t, hasAgent)
}
