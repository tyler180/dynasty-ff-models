package usage_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/tyler180/dynasty-ff-models/usage"
)

func TestUsageExample(t *testing.T) {
	payload, err := os.ReadFile("../data/usage-example.json")
	if err != nil {
		t.Fatal(err)
	}
	var input usage.Input
	if err := json.Unmarshal(payload, &input); err != nil {
		t.Fatal(err)
	}
	report, err := usage.Analyze(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Trends) != 1 || report.Trends[0].Signal != usage.SignalRising {
		t.Fatalf("report = %+v", report)
	}
}
