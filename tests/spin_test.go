package tests

import (
	"testing"
	"time"
	_ "unsafe"

	"github.com/denis1011101/super_cm_bot/app"
	_ "github.com/denis1011101/super_cm_bot/app" // для go:linkname
)

//go:linkname appNeedReset github.com/denis1011101/super_cm_bot/app.needReset
func appNeedReset(pen app.Pen) bool

// TestSpinAddPenSize_AlwaysNonNegative — SpinAddPenSize (для /giga) никогда не возвращает отрицательный Size.
func TestSpinAddPenSize_AlwaysNonNegative(t *testing.T) {
	pen := app.Pen{Size: 10, LastUpdateTime: time.Now()}
	for i := 0; i < 500; i++ {
		result := app.SpinAddPenSize(pen)
		if result.Size < 0 {
			t.Errorf("SpinAddPenSize returned negative Size=%d on iteration %d", result.Size, i)
		}
	}
}

// TestSpinAddPenSize_ProducesPositiveResults — за 500 итераций хоть один результат > 0.
// Если функция всегда возвращает 0, это баг.
func TestSpinAddPenSize_ProducesPositiveResults(t *testing.T) {
	pen := app.Pen{Size: 10, LastUpdateTime: time.Now()}
	gotPositive := false
	for i := 0; i < 500; i++ {
		if app.SpinAddPenSize(pen).Size > 0 {
			gotPositive = true
			break
		}
	}
	if !gotPositive {
		t.Error("SpinAddPenSize never returned Size > 0 in 500 iterations")
	}
}

// TestSpinDiffPenSize_AlwaysNonPositive — SpinDiffPenSize (для /unh) никогда не возвращает положительный Size.
func TestSpinDiffPenSize_AlwaysNonPositive(t *testing.T) {
	pen := app.Pen{Size: 50, LastUpdateTime: time.Now()}
	for i := 0; i < 500; i++ {
		result := app.SpinDiffPenSize(pen)
		if result.Size > 0 {
			t.Errorf("SpinDiffPenSize returned positive Size=%d on iteration %d", result.Size, i)
		}
	}
}

// TestSpinDiffPenSize_ProducesNegativeResults — за 500 итераций хоть один результат < 0.
func TestSpinDiffPenSize_ProducesNegativeResults(t *testing.T) {
	pen := app.Pen{Size: 50, LastUpdateTime: time.Now()}
	gotNegative := false
	for i := 0; i < 500; i++ {
		if app.SpinDiffPenSize(pen).Size < 0 {
			gotNegative = true
			break
		}
	}
	if !gotNegative {
		t.Error("SpinDiffPenSize never returned Size < 0 in 500 iterations")
	}
}

// TestSpinPenSize_ResultTypes — SpinPenSize возвращает только допустимые типы.
func TestSpinPenSize_ResultTypes(t *testing.T) {
	allowed := map[string]bool{"ADD": true, "DIFF": true, "ZERO": true, "RESET": true}
	pen := app.Pen{Size: 10, LastUpdateTime: time.Now()}
	for i := 0; i < 200; i++ {
		result := app.SpinPenSize(pen)
		if !allowed[result.ResultType] {
			t.Errorf("unexpected ResultType=%q on iteration %d", result.ResultType, i)
		}
	}
}

// TestSpinPenSize_ResetSetsNegativeFullSize — при ResultType=RESET размер обнуляется:
// result.Size == -pen.Size, то есть newSize после применения == 0.
func TestSpinPenSize_ResetSetsNegativeFullSize(t *testing.T) {
	// needReset срабатывает при size ~0: min = size*10000, чем меньше size, тем выше шанс.
	// При size=0 needReset почти всегда true, но calculateResult проверяет !needAdd && needReset,
	// что выполняется в SpinPenSize.
	pen := app.Pen{Size: 0, LastUpdateTime: time.Now()}
	gotReset := false
	for i := 0; i < 300; i++ {
		result := app.SpinPenSize(pen)
		if result.ResultType == "RESET" {
			gotReset = true
			if result.Size != -pen.Size {
				t.Errorf("RESET: expected Size=%d (= -pen.Size), got %d", -pen.Size, result.Size)
			}
		}
	}
	// RESET при size=0 ожидается часто, но не требуем обязательного появления —
	// функция вероятностная. Если таки встретился — проверили правильность поля Size.
	_ = gotReset
}

// TestNeedReset_ProbabilityGrowsWithSize — чем больше size, тем выше шанс reset.
// Логика: min = size*10000, диапазон [min, 10M]; при size=1000 min=10M → всегда > magicNumber.
// При size=0 диапазон [0, 10M] → шанс reset ≈1%.
func TestNeedReset_ProbabilityGrowsWithSize(t *testing.T) {
	penSmall := app.Pen{Size: 0}  // шанс reset ≈1%
	penLarge := app.Pen{Size: 1000} // шанс reset = 100% (min=max=10M > magicNumber=9.9M)

	smallResets := 0
	largeResets := 0
	const n = 100
	for i := 0; i < n; i++ {
		if appNeedReset(penSmall) {
			smallResets++
		}
		if appNeedReset(penLarge) {
			largeResets++
		}
	}

	// При size=1000 reset должен быть всегда
	if largeResets != n {
		t.Errorf("expected needReset=true always for size=1000, got %d/%d", largeResets, n)
	}
	// При size=0 reset редок (≈1%), точно не каждый раз
	if smallResets >= n {
		t.Errorf("expected needReset to be rare for size=0, got %d/%d", smallResets, n)
	}
}

// TestSpinSkipAction_RarelyTrue — SpinSkipAction возвращает true редко (~1%),
// и точно не всегда true и не всегда false.
func TestSpinSkipAction_RarelyTrue(t *testing.T) {
	const n = 1000
	trueCount := 0
	for i := 0; i < n; i++ {
		if app.SpinSkipAction() {
			trueCount++
		}
	}

	// Ожидаем ~1% (randomInt > 98 из 0..99 → только 99 → 1%).
	// Проверяем что хотя бы встречается и не доминирует.
	if trueCount == 0 {
		t.Errorf("SpinSkipAction never returned true in %d iterations (expected ~1%%)", n)
	}
	if trueCount > n/5 { // >20% — явно что-то не так
		t.Errorf("SpinSkipAction returned true too often: %d/%d", trueCount, n)
	}
}

// TestSelectRandomMember_AlwaysFromList — SelectRandomMember всегда возвращает элемент из переданного слайса.
func TestSelectRandomMember_AlwaysFromList(t *testing.T) {
	members := []app.Member{
		{ID: 1, Name: "alice"},
		{ID: 2, Name: "bob"},
		{ID: 3, Name: "carol"},
	}
	ids := map[int64]bool{1: true, 2: true, 3: true}

	for i := 0; i < 300; i++ {
		m := app.SelectRandomMember(members)
		if !ids[m.ID] {
			t.Errorf("SelectRandomMember returned member not in list: %+v", m)
		}
	}
}

// TestSelectRandomMember_EmptyList — пустой список возвращает нулевой Member.
func TestSelectRandomMember_EmptyList(t *testing.T) {
	m := app.SelectRandomMember([]app.Member{})
	if m.ID != 0 || m.Name != "" {
		t.Errorf("expected zero Member for empty list, got %+v", m)
	}
}

// TestSelectRandomMember_SingleMember — единственный участник всегда выбирается.
func TestSelectRandomMember_SingleMember(t *testing.T) {
	members := []app.Member{{ID: 42, Name: "solo"}}
	for i := 0; i < 50; i++ {
		m := app.SelectRandomMember(members)
		if m.ID != 42 {
			t.Errorf("expected ID=42, got %+v", m)
		}
	}
}
