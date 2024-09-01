package app

import (
	"math/rand"
	"time"
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
	return calculateResult(pen, false, false)
}

func SpinAddPenSize(pen Pen) SpinpenResult {
	return calculateResult(pen, true, false)
}

func SpinDiffPenSize(pen Pen) SpinpenResult {
	return calculateResult(pen, false, true)
}

func calculateResult(pen Pen, needAdd bool, needDiff bool) SpinpenResult {
	if !needAdd && needReset(pen) {
		return SpinpenResult{ResultType: "RESET", Size: 0}
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

	if size > 0 {
		size *= multiplicator
	}

	switch {
	case size < 0:
		return SpinpenResult{ResultType: "DIFF", Size: size}
	case size == 0:
		return SpinpenResult{ResultType: "ZERO", Size: size}
	default:
		return SpinpenResult{ResultType: "ADD", Size: size}
	}
}

func needReset(pen Pen) bool {
	return rand.Intn(pen.Size*10000+10000000) > 9900000
}

func calculateRandSize(randomInt int) int {
	switch {
	case randomInt > 500000 && randomInt <= 4000000:
		return 1
	case randomInt > 4000000 && randomInt <= 6500000:
		return 2
	case randomInt > 6500000 && randomInt <= 8000000:
		return 3
	case randomInt > 8000000 && randomInt <= 9500000:
		return 4
	case randomInt > 9500000 && randomInt <= 10000000:
		return 5
	default:
		return 0
	}
}

type Member struct {
	ID   int64
	Name string
}

type SpinAction struct{}

type SpinMemberResult struct {
	ResultType   string
	AnotherField int // Add any other fields here
}

// SpinunhandsomeOrGiga выбирает случайного члена из списка членов
func SpinunhandsomeOrGiga(members []Member) Member {
	if len(members) == 0 {
		return Member{}
	}
	randomInt := rand.Intn((len(members) * 1000000) - 1)
	selectedMember := members[randomInt/1000000]
	return selectedMember
}
