package scan

import "testing"

func TestIsTerminal(t *testing.T) {
	if !IsTerminal(StateSUCCESS) || !IsTerminal(StateFAILED) || !IsTerminal(StateTIMEOUT) || !IsTerminal(StateUNREACHABLE) {
		t.Error("terminal states should be terminal")
	}
	if IsTerminal(StatePENDING) || IsTerminal(StateRUNNING) || IsTerminal("") {
		t.Error("non-terminal states should not be terminal")
	}
}

func TestIsSuccess(t *testing.T) {
	if !IsSuccess(StateSUCCESS) {
		t.Error("SUCCESS should be success")
	}
	for _, st := range []string{StatePENDING, StateRUNNING, StateFAILED, StateTIMEOUT, StateUNREACHABLE, ""} {
		if IsSuccess(st) {
			t.Errorf("IsSuccess(%q) should be false", st)
		}
	}
}
