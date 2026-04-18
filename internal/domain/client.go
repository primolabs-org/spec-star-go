package domain

import (
	"time"

	"github.com/google/uuid"
)

// Client represents a wallet client with an external custody identifier.
type Client struct {
	clientID   uuid.UUID
	externalID string
	createdAt  time.Time
}

// NewClient creates a Client with a generated UUID and current timestamp.
func NewClient(externalID string) (*Client, error) {
	if externalID == "" {
		return nil, &ValidationError{Message: "external_id is required"}
	}
	return &Client{
		clientID:   uuid.New(),
		externalID: externalID,
		createdAt:  time.Now(),
	}, nil
}

// ReconstructClient loads a Client from persisted fields without generating new values.
func ReconstructClient(clientID uuid.UUID, externalID string, createdAt time.Time) *Client {
	return &Client{
		clientID:   clientID,
		externalID: externalID,
		createdAt:  createdAt,
	}
}

func (c *Client) ClientID() uuid.UUID  { return c.clientID }
func (c *Client) ExternalID() string   { return c.externalID }
func (c *Client) CreatedAt() time.Time { return c.createdAt }
