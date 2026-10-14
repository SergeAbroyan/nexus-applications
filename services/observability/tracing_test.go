package observability

import "testing"

func TestTraceSampleRatio(t *testing.T) {
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "0.25")

	ratio, err := traceSampleRatio()
	if err != nil {
		t.Fatal(err)
	}
	if ratio != 0.25 {
		t.Fatalf("expected sample ratio 0.25, got %f", ratio)
	}
}

func TestRejectsInvalidTraceSampleRatio(t *testing.T) {
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "2")

	if _, err := traceSampleRatio(); err == nil {
		t.Fatal("expected an invalid sample ratio error")
	}
}
