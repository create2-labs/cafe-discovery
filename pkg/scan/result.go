package scan

import "time"

// Finding represents a single discovery finding (e.g. one primitive, one cert).
type Finding struct {
	Type         string         `json:"type"`
	Name         string         `json:"name"`
	NISTLevel    int            `json:"nist_level"`
	QuantumVuln  bool            `json:"quantum_vulnerable"`
	Details      map[string]any  `json:"details,omitempty"`
}

// ScanResult is the contract for any discovery scan result.
// Domain types or adapters implement this; CBOM is generated via ToCBOM() on demand.
type ScanResult interface {
	ScanKind() string
	ScannedAt() time.Time
	Findings() []Finding
	Classification() string
	ToCBOM() (map[string]any, error)
}

// RawResult is optional: implement it to expose the domain result for persistence (scan.completed payload).
type RawResult interface {
	ScanResult
	Raw() interface{}
}
