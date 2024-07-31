package app

import (
	"math/rand"
	"time"
)

type pen struct {
	Size int
}

type SpinpenResult struct {
	ResultType string
	Size       int
}

type SpinpenSizeAction struct{}

func (s *SpinpenSizeAction) SpinpenSize(pen pen) SpinpenResult {
	return s.calculateResult(pen, false, false)
}

func (s *SpinpenSizeAction) SpinAddpenSize(pen pen) SpinpenResult {
	return s.calculateResult(pen, true, false)
}

func (s *SpinpenSizeAction) SpinDiffpenSize(pen pen) SpinpenResult {
	return s.calculateResult(pen, false, true)
}

func (s *SpinpenSizeAction) calculateResult(pen pen, needAdd bool, needDiff bool) SpinpenResult {
	if !needAdd && s.needReset(pen) {
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

	size := s.calculateRandSize(randomInt * multiplicator)

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

func (s *SpinpenSizeAction) needReset(pen pen) bool {
	return rand.Intn(pen.Size*10000+10000000) > 9900000
}

func (s *SpinpenSizeAction) calculateRandSize(randomInt int) int {
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
	ID	 int
	Name string
	Result string 
}

type SpinAction struct{}

type SpinMemberResult struct {
    ResultType   string
    AnotherField int // Add any other fields here
}

// SpinunhandsomeOrGiga выбирает случайного члена из списка членов
func SpinunhandsomeOrGiga(members []Member) Member {
    if len(members) == 0 || len(members) == 1 {
        return Member{}
    }
    randomInt := rand.Intn((len(members) * 1000000) - 1)
    selectedMember := members[randomInt/1000000]
    result := GenerateSpinMemberResult(selectedMember)
    selectedMember.Result = result.ResultType
    return selectedMember
}

const (
    SECRET = "SECRET"
    FIRST  = "FIRST"
    SECOND = "SECOND"
    THIRD  = "THIRD"
)

// GenerateSpinMemberResult генерирует случайный результат для спина
func GenerateSpinMemberResult(selectedMember Member) SpinMemberResult {
    rand.Seed(time.Now().UnixNano())
    randInt := rand.Intn(10000001) // Генерирует число от 0 до 10,000,000

    if randInt <= 300000 {
        return SpinMemberResult{ResultType: SECRET}
    }
    if randInt > 300000 && randInt <= 3300000 {
        return SpinMemberResult{ResultType: FIRST}
    }
    if randInt > 3300000 && randInt <= 6600000 {
        return SpinMemberResult{ResultType: SECOND}
    }

    return SpinMemberResult{ResultType: THIRD}
}