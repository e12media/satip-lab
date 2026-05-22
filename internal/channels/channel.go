package channels

import (
	"fmt"
	"strings"
)

type Channel struct {
	ID           string
	Number       int
	Name         string
	Group        string
	TvgID        string
	Frequency    int
	Polarization string
	SymbolRate   int
	Delivery     string
	Src          int
	Pids         []int
}

func (c Channel) TuningQuery() string {
	pids := make([]string, len(c.Pids))
	for i, pid := range c.Pids {
		pids[i] = fmt.Sprintf("%d", pid)
	}
	parts := []string{
		fmt.Sprintf("src=%d", c.Src),
		fmt.Sprintf("freq=%d", c.Frequency),
		fmt.Sprintf("pol=%s", c.Polarization),
		fmt.Sprintf("msys=%s", c.Delivery),
		fmt.Sprintf("sr=%d", c.SymbolRate),
		fmt.Sprintf("pids=%s", strings.Join(pids, ",")),
	}
	return strings.Join(parts, "&")
}
