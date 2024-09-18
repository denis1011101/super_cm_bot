package tests

import (
	"testing"
	"time"
	_ "unsafe"
	_ "github.com/denis1011101/super_cum_bot/app/handlers"
)

//go:linkname handlersCheckIsSpinNotLegal github.com/denis1011101/super_cum_bot/app/handlers.checkIsSpinNotLegal
func handlersCheckIsSpinNotLegal(lastUpdate time.Time) bool

func TestCheckIsSpinNotLegal(t *testing.T) {	
	// Test case: Last update is today and duration is less than 4 hours
	lastUpdate := time.Now().Add(-1 * time.Hour)
	if handlersCheckIsSpinNotLegal(lastUpdate) == false {
		t.Errorf("Expected true, but got false")
	}

	// Test case: Last update is today and equeals
	lastUpdate = time.Now().Add(time.Hour)
	if handlersCheckIsSpinNotLegal(lastUpdate) == false {
		t.Errorf("Expected true, but got false")
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
