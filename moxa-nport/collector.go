package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
)

type SlaveCollector struct {
	mu  sync.Mutex
	ids map[string]map[uint8]struct{}
}

func NewSlaveCollector() *SlaveCollector {
	return &SlaveCollector{ids: make(map[string]map[uint8]struct{})}
}

func (c *SlaveCollector) Record(port string, slave uint8) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ids[port] == nil {
		c.ids[port] = make(map[uint8]struct{})
	}
	c.ids[port][slave] = struct{}{}
}

func (c *SlaveCollector) Report() map[string][]uint8 {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string][]uint8, len(c.ids))
	for port, slaves := range c.ids {
		list := make([]uint8, 0, len(slaves))
		for s := range slaves {
			list = append(list, s)
		}
		sort.Slice(list, func(i, j int) bool { return list[i] < list[j] })
		out[port] = list
	}
	return out
}

func (c *SlaveCollector) WriteFile(path string) error {
	report := c.Report()
	var b strings.Builder
	for port, list := range report {
		fmt.Fprintf(&b, "%s: ", port)
		if len(list) == 0 {
			b.WriteString("none\n")
			continue
		}
		for i, s := range list {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%d", s)
		}
		b.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}
