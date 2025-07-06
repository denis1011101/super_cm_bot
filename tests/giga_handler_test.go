package tests

import (
    "testing"
    _ "unsafe"
    _ "github.com/denis1011101/super_cm_bot/app/handlers"
)

//go:linkname handlersAveragePenMass github.com/denis1011101/super_cm_bot/app/handlers.averagePenMass
func handlersAveragePenMass() (int, error)

//go:linkname handlersKineticEnergy github.com/denis1011101/super_cm_bot/app/handlers.kineticEnergy
func handlersKineticEnergy(velocity int) (int, error)

//go:linkname handlersPotentialEnergy github.com/denis1011101/super_cm_bot/app/handlers.potentialEnergy
func handlersPotentialEnergy(height int) (int, error)

//go:linkname handlersAverageVoltage github.com/denis1011101/super_cm_bot/app/handlers.averageVoltage
func handlersAverageVoltage() (int, error)

//go:linkname handlersOhmLaw github.com/denis1011101/super_cm_bot/app/handlers.ohmLaw
func handlersOhmLaw(resistance int) (int, error)

func TestAveragePenMass(t *testing.T) {
    for i := 0; i < 100; i++ {
        mass, err := handlersAveragePenMass()
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        if mass < 130 || mass > 140 {
            t.Errorf("mass out of range: got %d", mass)
        }
    }
}

func TestKineticEnergy(t *testing.T) {
    for v := 0; v <= 5; v++ {
        energy, err := handlersKineticEnergy(v)
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        if energy < 0 {
            t.Errorf("energy should not be negative: got %d", energy)
        }
    }
}

func TestPotentialEnergy(t *testing.T) {
    for h := 0; h <= 5; h++ {
        energy, err := handlersPotentialEnergy(h)
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        if energy < 0 {
            t.Errorf("energy should not be negative: got %d", energy)
        }
    }
}

func TestAverageVoltage(t *testing.T) {
    for i := 0; i < 100; i++ {
        voltage, err := handlersAverageVoltage()
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        if voltage < 1 || voltage > 100 {
            t.Errorf("voltage out of range: got %d", voltage)
        }
    }
}

func TestOhmLaw(t *testing.T) {
    for r := 1; r <= 5; r++ {
        val, err := handlersOhmLaw(r)
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        if val < 0 {
            t.Errorf("ohmLaw result should not be negative: got %d", val)
        }
    }
    _, err := handlersOhmLaw(0)
    if err == nil {
        t.Error("expected error for resistance=0, got nil")
    }
}
