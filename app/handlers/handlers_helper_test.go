package handlers

import (
	"testing"
	"time"
)

func TestCheckIsSpinNotLegal(t *testing.T) {
	// Test case: Last update is today and duration is less than 4 hours
	lastUpdate := time.Now().Add(-1 * time.Hour)
	if checkIsSpinNotLegal(lastUpdate) != true {
		t.Errorf("Expected true, but got false")
	}

	// Test case: Last update is today and equeals
	lastUpdate = time.Now().Add(time.Hour)
	if checkIsSpinNotLegal(lastUpdate) != true {
		t.Errorf("Expected true, but got false")
	}

	// Test case: Last update is not today
	lastUpdate = time.Now().Add(-24 * time.Hour)
	if checkIsSpinNotLegal(lastUpdate) != false {
		t.Errorf("Expected false, but got true")
	}

	// Test case: Last update is zero
	lastUpdate = time.Time{}
	if checkIsSpinNotLegal(lastUpdate) != false {
		t.Errorf("Expected false, but got true")
	}
}