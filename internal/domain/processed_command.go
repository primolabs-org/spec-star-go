package domain

import (
	"time"

	"github.com/google/uuid"
)

// ProcessedCommand records an idempotent command execution with its response snapshot.
type ProcessedCommand struct {
	commandID        uuid.UUID
	commandType      string
	orderID          string
	clientID         uuid.UUID
	responseSnapshot []byte
	createdAt        time.Time
}

// NewProcessedCommand creates a ProcessedCommand with a generated UUID and current timestamp.
func NewProcessedCommand(
	commandType string,
	orderID string,
	clientID uuid.UUID,
	responseSnapshot []byte,
) (*ProcessedCommand, error) {
	if commandType == "" {
		return nil, &ValidationError{Message: "command_type is required"}
	}
	if orderID == "" {
		return nil, &ValidationError{Message: "order_id is required"}
	}
	if clientID == uuid.Nil {
		return nil, &ValidationError{Message: "client_id is required"}
	}
	if len(responseSnapshot) == 0 {
		return nil, &ValidationError{Message: "response_snapshot is required"}
	}
	return &ProcessedCommand{
		commandID:        uuid.New(),
		commandType:      commandType,
		orderID:          orderID,
		clientID:         clientID,
		responseSnapshot: responseSnapshot,
		createdAt:        time.Now(),
	}, nil
}

// ReconstructProcessedCommand loads a ProcessedCommand from persisted fields without generating new values.
func ReconstructProcessedCommand(
	commandID uuid.UUID,
	commandType string,
	orderID string,
	clientID uuid.UUID,
	responseSnapshot []byte,
	createdAt time.Time,
) *ProcessedCommand {
	return &ProcessedCommand{
		commandID:        commandID,
		commandType:      commandType,
		orderID:          orderID,
		clientID:         clientID,
		responseSnapshot: responseSnapshot,
		createdAt:        createdAt,
	}
}

func (pc *ProcessedCommand) CommandID() uuid.UUID        { return pc.commandID }
func (pc *ProcessedCommand) CommandType() string          { return pc.commandType }
func (pc *ProcessedCommand) OrderID() string              { return pc.orderID }
func (pc *ProcessedCommand) ClientID() uuid.UUID          { return pc.clientID }
func (pc *ProcessedCommand) ResponseSnapshot() []byte     { return pc.responseSnapshot }
func (pc *ProcessedCommand) CreatedAt() time.Time         { return pc.createdAt }
