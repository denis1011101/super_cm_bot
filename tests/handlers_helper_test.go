package tests

import (
	"testing"
	"time"
	_ "unsafe"

	_ "github.com/denis1011101/super_cm_bot/app/handlers"
)

//go:linkname handlersCheckIsSpinNotLegal github.com/denis1011101/super_cm_bot/app/handlers.checkIsSpinNotLegal
func handlersCheckIsSpinNotLegal(lastUpdate time.Time) bool

func TestCheckIsSpinNotLegal(t *testing.T) {
	// Test case: Last update is today and duration is less than 4 hours
	lastUpdate := time.Now().Add(-3 * time.Hour)
	if handlersCheckIsSpinNotLegal(lastUpdate) == false {
		t.Errorf("Expected true, but got false")
	}

	// Test case: Last update is today and duration is exactly 4 hours
	lastUpdate = time.Now().Add(-4 * time.Hour)
	if handlersCheckIsSpinNotLegal(lastUpdate) == true {
		t.Errorf("Expected false, but got true")
	}

	// Test case: Last update is today and duration is more than 4 hours
	lastUpdate = time.Now().Add(-5 * time.Hour)
	if handlersCheckIsSpinNotLegal(lastUpdate) == true {
		t.Errorf("Expected false, but got true")
	}

	// Test case: Last update is today and duration is more than 4 hours but less than 24 hours
	lastUpdate = time.Now().Add(-23 * time.Hour)
	if handlersCheckIsSpinNotLegal(lastUpdate) == true {
		t.Errorf("Expected false, but got true")
	}

	// Test case: Last update is today and duration is less than 4 hours but on the next day
	lastUpdate = time.Now().AddDate(0, 0, -1).Add(3 * time.Hour)
	if handlersCheckIsSpinNotLegal(lastUpdate) == true {
		t.Errorf("Expected false, but got true")
	}

	// Test case: Last update is not today
	lastUpdate = time.Now().Add(-24 * time.Hour)
	if handlersCheckIsSpinNotLegal(lastUpdate) == true {
		t.Errorf("Expected false, but got true")
	}

	// Test case: Last update is zero
	lastUpdate = time.Time{}
	if handlersCheckIsSpinNotLegal(lastUpdate) == true {
		t.Errorf("Expected false, but got true")
	}
}
