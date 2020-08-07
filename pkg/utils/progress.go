package utils

import (
	"fmt"
	"time"

	"github.com/briandowns/spinner"
)

func NewSpinner(prefix string) *spinner.Spinner {
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Prefix = prefix
	s.FinalMSG = fmt.Sprintf("%s [Done]\n", prefix)
	return s
}
