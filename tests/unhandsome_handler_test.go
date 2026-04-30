package tests

import (
	"testing"
	"time"
	_ "unsafe"

	"github.com/denis1011101/super_cm_bot/app"
	_ "github.com/denis1011101/super_cm_bot/app/handlers"
)

//go:linkname handlersNormalizeUnhandsomeDiffSize github.com/denis1011101/super_cm_bot/app/handlers.normalizeUnhandsomeDiffSize
func handlersNormalizeUnhandsomeDiffSize(diffSize int) int

func TestNormalizeUnhandsomeDiffSize_ClampsToAtMostMinusOne(t *testing.T) {
	cases := []struct {
		input int
		want  int
	}{
		{input: 5, want: -1},
		{input: 1, want: -1},
		{input: 0, want: -1},
		{input: -1, want: -1},
		{input: -2, want: -2},
		{input: -5, want: -5},
		{input: -50, want: -50},
	}

	for _, tc := range cases {
		got := handlersNormalizeUnhandsomeDiffSize(tc.input)
		if got != tc.want {
			t.Errorf("normalizeUnhandsomeDiffSize(%d) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestUnhandsomeDerivedDiffSize_ExpectedRangeOrReset(t *testing.T) {
	penSizes := []int{50, 0, -5}

	for _, size := range penSizes {
		pen := app.Pen{Size: size, LastUpdateTime: time.Now()}

		for i := 0; i < 500; i++ {
			result := app.SpinDiffPenSize(pen)

			if result.ResultType == "RESET" {
				newSize := pen.Size + result.Size
				if newSize != 0 {
					t.Fatalf("RESET must bring size to 0: pen.Size=%d, result.Size=%d, newSize=%d", pen.Size, result.Size, newSize)
				}
				continue
			}

			diffSize := handlersNormalizeUnhandsomeDiffSize(result.Size)
			if diffSize > -1 || diffSize < -5 {
				t.Fatalf("unhandsome diffSize out of range: got %d, resultType=%s", diffSize, result.ResultType)
			}
		}
	}
}
