package telemetry

import (
	"os"

	"github.com/grafana/pyroscope-go"
)

func StartProfiling(service string) (func() error, error) {
	address := os.Getenv("PYROSCOPE_SERVER_ADDRESS")
	if address == "" {
		return func() error { return nil }, nil
	}

	profiler, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: service,
		ServerAddress:   address,
		Tags:            map[string]string{"pod": os.Getenv("HOSTNAME")},
		ProfileTypes: []pyroscope.ProfileType{
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
			pyroscope.ProfileGoroutines,
		},
	})
	if err != nil {
		return nil, err
	}
	return profiler.Stop, nil
}
