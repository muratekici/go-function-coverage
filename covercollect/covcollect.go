package covcollect

import (
	"bufio"
	"fmt"
	"os"
	"time"
)

type Cover struct {
	Len    int
	Names  []string
	Lines  []uint32
	Counts []bool
}

func (c Cover) PeriodicalCollect(period string, args ...string) {

	duration, err := time.ParseDuration(period)
	if err != nil || duration <= 0 {
		return
	}

	ticker := time.NewTicker(duration)

	for _ = range ticker.C {
		c.Collect(args...)
	}
}

func (c Cover) Collect(args ...string) {

	fd, err := os.Create(args[0])
	if err != nil {
		panic(err)
	}

	w := bufio.NewWriter(fd)

	defer func() {
		w.Flush()
		fd.Close()
	}()

	for i, count := range c.Counts {
		fmt.Fprintf(w, "%s:%d:%d\n", c.Names[i], c.Lines[i], count)
	}
}
