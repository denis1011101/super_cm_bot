package app

import (
	"math/rand"
	"time"
	"log"
)

type Pen struct {
	Size           int
	LastUpdateTime time.Time
}

type SpinpenResult struct {
	ResultType string
	Size       int
}

func SpinPenSize(pen Pen) SpinpenResult {
    log.Printf("SpinPenSize called with pen: %+v", pen)
    result := calculateResult(pen, false, false)
    log.Printf("SpinPenSize result: %+v", result)
	return result
}

func SpinAddPenSize(pen Pen) SpinpenResult {
    log.Printf("SpinAddPenSize called with pen: %+v", pen)
    result := calculateResult(pen, true, false)
    log.Printf("SpinAddPenSize result: %+v", result)
    return result
}

func SpinDiffPenSize(pen Pen) SpinpenResult {
    log.Printf("SpinDiffPenSize called with pen: %+v", pen)
    result := calculateResult(pen, false, true)
    log.Printf("SpinDiffPenSize result: %+v", result)
    return result
}

func calculateResult(pen Pen, needAdd bool, needDiff bool) SpinpenResult {
    log.Printf("calculateResult called with pen: %+v, needAdd: %v, needDiff: %v", pen, needAdd, needDiff)
    if !needAdd && needReset(pen) {
        log.Printf("Resetting pen: %+v", pen)
        return SpinpenResult{ResultType: "RESET", Size: -pen.Size}
    }

	min := -10000000
	if needAdd {
		min = 0
	}
	max := 50000000
	if needDiff {
		max = 0
	}

	randomInt := rand.Intn(max-min+1) + min
	log.Printf("Random integer generated: %d", randomInt)

	if randomInt > 40000000 {
		randomInt -= 40000000
	}
	if randomInt > 30000000 {
		randomInt -= 30000000
	}
	if randomInt > 20000000 {
		randomInt -= 20000000
	}
	if randomInt > 10000000 {
		randomInt -= 10000000
	}

	multiplicator := 1
	if randomInt < 0 {
		multiplicator = -1
	}

	size := calculateRandSize(randomInt * multiplicator)
	log.Printf("Calculated size: %d", size)

	if size > 0 {
		size *= multiplicator
	}

	switch {
    case size < 0:
        log.Printf("Result type: DIFF, size: %d", size)
        return SpinpenResult{ResultType: "DIFF", Size: size}
    case size == 0:
        log.Printf("Result type: ZERO, size: %d", size)
        return SpinpenResult{ResultType: "ZERO", Size: size}
    default:
        log.Printf("Result type: ADD, size: %d", size)
        return SpinpenResult{ResultType: "ADD", Size: size}
	}
}

func needReset(pen Pen) bool {
    min := pen.Size * 10000
    max := 10000000
    randomNumber := rand.Intn(max-min+1) + min
    log.Printf("Random number for reset check: %d", randomNumber)
    needReset := randomNumber > 9900000
    log.Printf("Need reset: %v", needReset)
    return needReset
}

func calculateRandSize(randomInt int) int {
    log.Printf("calculateRandSize called with randomInt: %d", randomInt)
    var size int
    switch {
    case randomInt > 500000 && randomInt <= 4000000:
        size = 1
    case randomInt > 4000000 && randomInt <= 6500000:
        size = 2
    case randomInt > 6500000 && randomInt <= 8000000:
        size = 3
    case randomInt > 8000000 && randomInt <= 9500000:
        size = 4
    case randomInt > 9500000 && randomInt <= 10000000:
        size = 5
    default:
        size = 0
    }
    log.Printf("Calculated size: %d", size)
    return size
}

type Member struct {
	ID   int64
	Name string
}

type SpinAction struct{}

type SpinMemberResult struct {
	ResultType   string
	AnotherField int
}

// Выбирает случайного участника из списка
func SelectRandomMember(members []Member) Member {
	if len(members) == 0 {
		return Member{}
	}
	randomInt := rand.Intn((len(members) * 1000000) - 1)
	selectedMember := members[randomInt/1000000]
	log.Printf("Selected member: %+v", selectedMember)
	return selectedMember
}

// Возвращает true, если действие должно быть пропущено (1% шанс)
func SpinSkipAction() bool {
	randomInt := rand.Intn(100)
	return randomInt > 98
}
