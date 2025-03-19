package task

import (
	"log"
	"os"
	"testing"
)

func TestValidStateTransition(t *testing.T) {
	// Redirect log output to /dev/null to avoid cluttering test output
	log.SetOutput(os.NewFile(0, os.DevNull))

	tests := []struct {
		name     string
		src      State
		dst      State
		expected bool
	}{
		{"Pending to Scheduled", Pending, Scheduled, true},
		{"Pending to Running", Pending, Running, false},
		{"Scheduled to Scheduled", Scheduled, Scheduled, true},
		{"Scheduled to Running", Scheduled, Running, true},
		{"Scheduled to Failed", Scheduled, Failed, true},
		{"Scheduled to Completed", Scheduled, Completed, false},
		{"Running to Running", Running, Running, true},
		{"Running to Completed", Running, Completed, true},
		{"Running to Failed", Running, Failed, true},
		{"Running to Scheduled", Running, Scheduled, true},
		{"Running to Pending", Running, Pending, false},
		{"Completed to Scheduled", Completed, Scheduled, false},
		{"Completed to Running", Completed, Running, false},
		{"Failed to Scheduled", Failed, Scheduled, true},
		{"Failed to Running", Failed, Running, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidStateTransition(tt.src, tt.dst)
			if result != tt.expected {
				t.Errorf("ValidStateTransition(%v, %v) = %v; want %v", tt.src, tt.dst, result, tt.expected)
			}
		})
	}
}
