package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseAgentResultWithCodeFence(t *testing.T) {
	raw := "analysis\n```json\n{\"target_url\":\"https://example.com\",\"overall_result\":\"failed\",\"summary\":\"1 failure\",\"executed_cases\":[],\"bugs\":[]}\n```"

	result, err := parseAgentResult(raw)
	if err != nil {
		t.Fatalf("parseAgentResult failed: %v", err)
	}

	if result.TargetURL != "https://example.com" {
		t.Fatalf("unexpected target_url: %s", result.TargetURL)
	}
	if result.OverallResult != "failed" {
		t.Fatalf("unexpected overall_result: %s", result.OverallResult)
	}
}

func TestNeedsBugReport(t *testing.T) {
	passed := testRunResult{
		OverallResult: "passed",
		ExecutedCases: []executedCaseResult{{
			ID: "TC-1", Title: "home", Result: "passed",
		}},
		Bugs: []bugReportItem{},
	}
	if needsBugReport(passed) {
		t.Fatal("expected no bug report for passed result")
	}

	failed := passed
	failed.ExecutedCases[0].Result = "failed"
	if !needsBugReport(failed) {
		t.Fatal("expected bug report for failed case")
	}
}

func TestRenderBugReport(t *testing.T) {
	result := testRunResult{
		TargetURL:     "https://example.com",
		OverallResult: "failed",
		Summary:       "checkout failed",
		ExecutedCases: []executedCaseResult{{
			ID:       "TC-2",
			Title:    "Checkout",
			Result:   "failed",
			Steps:    "open checkout and submit",
			Expected: "order succeeds",
			Actual:   "500 error",
			Evidence: []string{"console error: 500"},
		}},
		Bugs: []bugReportItem{{
			ID:               "BUG-1",
			Title:            "Checkout returns 500",
			Severity:         "high",
			RelatedCaseID:    "TC-2",
			StepsToReproduce: "submit checkout form",
			Expected:         "successful order",
			Actual:           "500 internal error",
			Evidence:         []string{"screenshot.png"},
		}},
	}

	report := renderBugReport(result, "testdoc.md", time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC))
	checks := []string{
		"# Bug Report",
		"`https://example.com`",
		"TC-2 Checkout",
		"BUG-1 Checkout returns 500",
		"screenshot.png",
	}
	for _, check := range checks {
		if !strings.Contains(report, check) {
			t.Fatalf("report missing: %s", check)
		}
	}
}

func TestBuildExecutionFailureResult(t *testing.T) {
	result := buildExecutionFailureResult(errors.New("context deadline exceeded"))
	if result.OverallResult != "blocked" {
		t.Fatalf("expected blocked result, got %s", result.OverallResult)
	}
	if len(result.Bugs) != 1 {
		t.Fatalf("expected 1 bug, got %d", len(result.Bugs))
	}
	if !strings.Contains(result.Bugs[0].Actual, "context deadline exceeded") {
		t.Fatalf("unexpected bug actual: %s", result.Bugs[0].Actual)
	}
}
