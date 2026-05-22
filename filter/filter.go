package filter

import (
	"strings"

	"github.com/leehoawki/pika/model"
)

type Opts struct {
	Process string
}

func Apply(conns []model.Connection, opts Opts) []model.Connection {
	if opts.Process == "" {
		return conns
	}

	var result []model.Connection
	for _, c := range conns {
		if !matchProcess(c, opts.Process) {
			continue
		}
		result = append(result, c)
	}
	return result
}

func matchProcess(c model.Connection, process string) bool {
	return strings.Contains(
		strings.ToLower(c.ProcessName),
		strings.ToLower(process),
	)
}
