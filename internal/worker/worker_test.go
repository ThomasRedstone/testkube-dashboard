package worker

import (
	"encoding/json"
	"testing"
)

func TestPlaywrightTestCases(t *testing.T) {
	report := `{
		"suites": [{
			"title": "login.spec.ts",
			"file": "login.spec.ts",
			"specs": [{
				"title": "loads the page",
				"file": "login.spec.ts",
				"tests": [{
					"projectName": "chromium",
					"results": [
						{"status": "failed", "duration": 900, "error": {"message": "timeout"}},
						{"status": "passed", "duration": 400}
					]
				}]
			}],
			"suites": [{
				"title": "form",
				"file": "login.spec.ts",
				"specs": [{
					"title": "submits",
					"file": "login.spec.ts",
					"tests": [{"projectName": "", "results": [{"status": "failed", "duration": 50, "error": {"message": "boom"}}]}]
				}]
			}]
		}]
	}`

	var results PlaywrightResults
	if err := json.Unmarshal([]byte(report), &results); err != nil {
		t.Fatal(err)
	}
	cases := playwrightTestCases("exec-1", results)
	if len(cases) != 2 {
		t.Fatalf("got %d cases, want 2: %+v", len(cases), cases)
	}

	first := cases[0]
	if first.TestName != "loads the page [chromium]" || first.Status != "passed" || first.DurationMs != 400 || first.RetryCount != 1 || first.ErrorMessage != "" {
		t.Errorf("unexpected first case: %+v", first)
	}
	nested := cases[1]
	if nested.TestName != "submits" || nested.Status != "failed" || nested.ErrorMessage != "boom" || nested.RetryCount != 0 || nested.ExecutionID != "exec-1" {
		t.Errorf("unexpected nested case: %+v", nested)
	}
}
