package scan

// State represents the lifecycle state of a scan (enforced by persistence-service).
const (
	StatePENDING     = "PENDING"
	StateRUNNING     = "RUNNING"
	StateSUCCESS     = "SUCCESS"
	StateFAILED      = "FAILED"
	StateTIMEOUT     = "TIMEOUT"
	StateUNREACHABLE = "UNREACHABLE"
)

// TerminalStates are states that must not be overwritten by RUNNING or other non-terminal states.
var TerminalStates = map[string]bool{
	StateSUCCESS: true, StateFAILED: true, StateTIMEOUT: true, StateUNREACHABLE: true,
}

// IsTerminal returns true if status is a terminal state (must not be downgraded).
func IsTerminal(status string) bool {
	return TerminalStates[status]
}

// IsSuccess returns true when status is a completed-success scan (G4 / P1 CBOM gate).
func IsSuccess(status string) bool {
	return status == StateSUCCESS
}
