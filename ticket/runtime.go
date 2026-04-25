package ticket

import "time"

// ClaimsDir is the subdirectory within a ticket store that holds runtime sidecar files.
const ClaimsDir = ".claims"

// ClaimsSuffix is the file extension for claim sidecar files.
const ClaimsSuffix = ".json"

// DefaultLeaseDuration is the default duration for a ticket lease.
// After this duration without a heartbeat renewal the lease is considered expired.
const DefaultLeaseDuration = 15 * time.Minute

// RuntimeState is the sidecar runtime state for a ticket.
// It lives in .tickets/.claims/<ticket-id>.json, not in the ticket frontmatter.
// This enforces D6: claims/leases are stored in sidecars, never in frontmatter.
type RuntimeState struct {
	// TicketID is the canonical ID of the ticket this state belongs to.
	TicketID string `json:"ticket_id"`

	// Status is the last-known lifecycle status of the ticket.
	// Written by Release so Claim can enforce eligibility on subsequent claim attempts.
	// An empty value is treated as StatusPending (eligible for claiming).
	Status Status `json:"status,omitempty"`

	// Claim records the agent that has claimed the ticket, if any.
	Claim *Claim `json:"claim,omitempty"`

	// Lease records when the current claim expires.
	Lease *Lease `json:"lease,omitempty"`

	// Heartbeat records the last keepalive from the claiming agent.
	Heartbeat *Heartbeat `json:"heartbeat,omitempty"`

	// Phase is the current execution phase within the implementing lifecycle.
	// Known values: "implementing", "verifying", "reviewing", "repairing".
	Phase string `json:"phase,omitempty"`

	// Attempt is the number of implementation attempts made for this ticket.
	Attempt int `json:"attempt,omitempty"`

	// Artifacts holds references to review, repair, or closeout artifacts.
	Artifacts []string `json:"artifacts,omitempty"`
}

// Claim records which agent has claimed a ticket and when.
type Claim struct {
	// ClaimedBy is the identifier of the agent or run that holds the claim.
	ClaimedBy string `json:"claimed_by"`

	// ClaimBackend identifies the execution backend (e.g. "verk", "codex").
	ClaimBackend string `json:"claim_backend"`

	// ClaimedAt is the wall-clock time when the claim was established.
	ClaimedAt time.Time `json:"claimed_at"`
}

// Lease records when a claim expires and must be renewed.
type Lease struct {
	// LeaseID is a unique identifier for this lease instance.
	LeaseID string `json:"lease_id"`

	// ExpiresAt is the wall-clock time after which the lease is considered expired.
	ExpiresAt time.Time `json:"expires_at"`
}

// Heartbeat records the last keepalive signal from the claiming agent.
type Heartbeat struct {
	// LastBeat is the wall-clock time of the most recent heartbeat.
	LastBeat time.Time `json:"last_beat"`
}
