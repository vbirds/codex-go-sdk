package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fanwenlin/codex-go-sdk/codex"
)

const (
	exitCodeSuccess       = 0
	exitCodeExecutionFail = 1
	exitCodeBugFound      = 2
)

type cliConfig struct {
	InputPath        string
	ReportPath       string
	Model            string
	WorkingDirectory string
	Transport        string
	SandboxMode      string
	SkillsCSV        string
	CodexPath        string
	BaseURL          string
	APIKey           string
	Timeout          time.Duration
	JSONOutput       bool
	Verbose          bool
}

type testRunResult struct {
	TargetURL     string               `json:"target_url"`
	OverallResult string               `json:"overall_result"`
	Summary       string               `json:"summary"`
	ExecutedCases []executedCaseResult `json:"executed_cases"`
	Bugs          []bugReportItem      `json:"bugs"`
}

type executedCaseResult struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Result   string   `json:"result"`
	Steps    string   `json:"steps"`
	Expected string   `json:"expected"`
	Actual   string   `json:"actual"`
	Evidence []string `json:"evidence"`
}

type bugReportItem struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	Severity         string   `json:"severity"`
	RelatedCaseID    string   `json:"related_case_id"`
	StepsToReproduce string   `json:"steps_to_reproduce"`
	Expected         string   `json:"expected"`
	Actual           string   `json:"actual"`
	Evidence         []string `json:"evidence"`
}

func main() {
	code, err := run(os.Args[1:], os.Stdout, os.Stderr, time.Now)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}

func run(args []string, stdout io.Writer, stderr io.Writer, now func() time.Time) (int, error) {
	cfg, err := parseFlags(args)
	if err != nil {
		return exitCodeExecutionFail, err
	}

	docBytes, err := os.ReadFile(cfg.InputPath)
	if err != nil {
		return exitCodeExecutionFail, fmt.Errorf("failed to read input document: %w", err)
	}

	workingDirectory, err := resolveWorkingDirectory(cfg.WorkingDirectory)
	if err != nil {
		return exitCodeExecutionFail, err
	}
	cfg.WorkingDirectory = workingDirectory

	transport, err := parseTransport(cfg.Transport)
	if err != nil {
		return exitCodeExecutionFail, err
	}

	sandboxMode, err := parseSandboxMode(cfg.SandboxMode)
	if err != nil {
		return exitCodeExecutionFail, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	client := codex.NewCodex(codex.CodexOptions{
		Transport:         transport,
		CodexPathOverride: cfg.CodexPath,
		BaseUrl:           cfg.BaseURL,
		ApiKey:            cfg.APIKey,
		Verbose:           cfg.Verbose,
		VerboseWriter:     stderr,
	})

	thread := client.StartThread(codex.ThreadOptions{
		Model:                cfg.Model,
		SandboxMode:          sandboxMode,
		WorkingDirectory:     cfg.WorkingDirectory,
		NetworkAccessEnabled: true,
		ApprovalPolicy:       codex.ApprovalModeNever,
	})

	inputs := buildInputs(string(docBytes), parseSkills(cfg.SkillsCSV))
	turn, err := thread.Run(inputs, codex.TurnOptions{
		OutputSchema: buildResultOutputSchema(),
		Context:      ctx,
	})
	if err != nil {
		blocked := buildExecutionFailureResult(err)
		if cfg.JSONOutput {
			if writeErr := writeJSON(stdout, blocked); writeErr != nil {
				return exitCodeExecutionFail, writeErr
			}
		}

		report := renderBugReport(blocked, cfg.InputPath, now())
		if cfg.ReportPath != "" {
			if writeErr := os.WriteFile(cfg.ReportPath, []byte(report), 0o644); writeErr != nil {
				return exitCodeExecutionFail, fmt.Errorf("failed to write report file: %w", writeErr)
			}
			_, _ = fmt.Fprintf(stdout, "Bug report written to %s\n", cfg.ReportPath)
		} else {
			_, _ = io.WriteString(stdout, report)
		}
		return exitCodeBugFound, nil
	}

	result, err := parseAgentResult(turn.FinalResponse)
	if err != nil {
		return exitCodeExecutionFail, fmt.Errorf("failed to parse agent result JSON: %w\nraw: %s", err, turn.FinalResponse)
	}

	if cfg.JSONOutput {
		if writeErr := writeJSON(stdout, result); writeErr != nil {
			return exitCodeExecutionFail, writeErr
		}
	}

	if !needsBugReport(result) {
		if !cfg.JSONOutput {
			_, _ = fmt.Fprintf(stdout, "No blocking issues found. overall_result=%s\n", result.OverallResult)
		}
		return exitCodeSuccess, nil
	}

	report := renderBugReport(result, cfg.InputPath, now())
	if cfg.ReportPath != "" {
		if err := os.WriteFile(cfg.ReportPath, []byte(report), 0o644); err != nil {
			return exitCodeExecutionFail, fmt.Errorf("failed to write report file: %w", err)
		}
		_, _ = fmt.Fprintf(stdout, "Bug report written to %s\n", cfg.ReportPath)
	} else {
		_, _ = io.WriteString(stdout, report)
	}

	return exitCodeBugFound, nil
}

func parseFlags(args []string) (cliConfig, error) {
	cfg := cliConfig{}
	fs := flag.NewFlagSet("codex-webtest", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.StringVar(&cfg.InputPath, "input", "", "Path to the test instruction document")
	fs.StringVar(&cfg.ReportPath, "report", "", "Optional output path for bug report markdown")
	fs.StringVar(&cfg.Model, "model", "", "Model to run")
	fs.StringVar(&cfg.WorkingDirectory, "cwd", "", "Working directory for codex agent")
	fs.StringVar(&cfg.Transport, "transport", string(codex.TransportAppServer), "Transport: app-server or cli")
	fs.StringVar(&cfg.SandboxMode, "sandbox", string(codex.SandboxModeWorkspaceWrite), "Sandbox mode")
	fs.StringVar(&cfg.SkillsCSV, "skills", "playwright", "Comma-separated skill names")
	fs.StringVar(&cfg.CodexPath, "codex-path", "", "Optional codex binary path")
	fs.StringVar(&cfg.BaseURL, "base-url", "", "Optional API base URL")
	fs.StringVar(&cfg.APIKey, "api-key", "", "Optional API key")
	fs.DurationVar(&cfg.Timeout, "timeout", 20*time.Minute, "Turn timeout")
	fs.BoolVar(&cfg.JSONOutput, "json", false, "Print structured result JSON")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "Enable SDK verbose logs")

	if err := fs.Parse(args); err != nil {
		return cliConfig{}, fmt.Errorf("failed to parse flags: %w", err)
	}
	if cfg.InputPath == "" {
		return cliConfig{}, errors.New("missing required flag: --input")
	}
	return cfg, nil
}

func parseTransport(raw string) (codex.TransportMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(codex.TransportAppServer):
		return codex.TransportAppServer, nil
	case string(codex.TransportCLI):
		return codex.TransportCLI, nil
	default:
		return "", fmt.Errorf("unsupported transport: %s", raw)
	}
}

func parseSandboxMode(raw string) (codex.SandboxMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(codex.SandboxModeWorkspaceWrite):
		return codex.SandboxModeWorkspaceWrite, nil
	case string(codex.SandboxModeReadOnly):
		return codex.SandboxModeReadOnly, nil
	case string(codex.SandboxModeFullAccess):
		return codex.SandboxModeFullAccess, nil
	default:
		return "", fmt.Errorf("unsupported sandbox mode: %s", raw)
	}
}

func resolveWorkingDirectory(raw string) (string, error) {
	if raw == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current working directory: %w", err)
		}
		return cwd, nil
	}
	absPath, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("failed to resolve working directory: %w", err)
	}
	return absPath, nil
}

func parseSkills(skillsCSV string) []string {
	parts := strings.Split(skillsCSV, ",")
	seen := map[string]struct{}{}
	skills := make([]string, 0, len(parts))
	for _, part := range parts {
		skill := strings.TrimSpace(part)
		if skill == "" {
			continue
		}
		if _, ok := seen[skill]; ok {
			continue
		}
		seen[skill] = struct{}{}
		skills = append(skills, skill)
	}
	return skills
}

func buildInputs(document string, skills []string) []codex.UserInput {
	inputs := []codex.UserInput{codex.NewTextInput(buildTaskPrompt(document))}
	for _, skill := range skills {
		inputs = append(inputs, codex.NewSkillInput(skill))
	}
	return inputs
}

func buildTaskPrompt(document string) string {
	instruction := strings.TrimSpace(`你是一个网页自动化测试Agent。

任务要求：
1) 读取下方“测试文档”中的页面地址和测试用例。
2) 使用浏览器自动化能力逐条执行测试用例，禁止臆造执行结果。
3) 每个用例都必须给出执行结果（passed/failed/blocked）和证据。
4) 如果发现问题，输出可复现、可定位的 bug 信息。
5) 如果遇到执行阻塞（例如页面不可达/权限不足），必须明确记录原因和证据。
6) 仅输出符合给定 JSON Schema 的内容。`)

	return fmt.Sprintf("%s\n\n[测试文档开始]\n%s\n[测试文档结束]", instruction, document)
}

func buildResultOutputSchema() map[string]interface{} {
	caseSchema := map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]interface{}{
			"id":       map[string]interface{}{"type": "string"},
			"title":    map[string]interface{}{"type": "string"},
			"result":   map[string]interface{}{"type": "string", "enum": []string{"passed", "failed", "blocked"}},
			"steps":    map[string]interface{}{"type": "string"},
			"expected": map[string]interface{}{"type": "string"},
			"actual":   map[string]interface{}{"type": "string"},
			"evidence": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
		},
		"required": []string{"id", "title", "result", "steps", "expected", "actual", "evidence"},
	}

	bugSchema := map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]interface{}{
			"id":                 map[string]interface{}{"type": "string"},
			"title":              map[string]interface{}{"type": "string"},
			"severity":           map[string]interface{}{"type": "string", "enum": []string{"critical", "high", "medium", "low"}},
			"related_case_id":    map[string]interface{}{"type": "string"},
			"steps_to_reproduce": map[string]interface{}{"type": "string"},
			"expected":           map[string]interface{}{"type": "string"},
			"actual":             map[string]interface{}{"type": "string"},
			"evidence":           map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
		},
		"required": []string{"id", "title", "severity", "related_case_id", "steps_to_reproduce", "expected", "actual", "evidence"},
	}

	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]interface{}{
			"target_url":     map[string]interface{}{"type": "string"},
			"overall_result": map[string]interface{}{"type": "string", "enum": []string{"passed", "failed", "blocked"}},
			"summary":        map[string]interface{}{"type": "string"},
			"executed_cases": map[string]interface{}{"type": "array", "items": caseSchema},
			"bugs":           map[string]interface{}{"type": "array", "items": bugSchema},
		},
		"required": []string{"target_url", "overall_result", "summary", "executed_cases", "bugs"},
	}
}

func parseAgentResult(raw string) (testRunResult, error) {
	candidates := collectJSONCandidates(raw)
	lastErr := errors.New("no valid JSON candidate found")

	for _, candidate := range candidates {
		var result testRunResult
		if err := json.Unmarshal([]byte(candidate), &result); err != nil {
			lastErr = err
			continue
		}
		normalizeResult(&result)
		return result, nil
	}

	return testRunResult{}, lastErr
}

func collectJSONCandidates(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	candidates := []string{trimmed}

	if extracted := extractCodeFenceJSON(trimmed); extracted != "" {
		candidates = append([]string{extracted}, candidates...)
	}

	if extracted := extractJSONObject(trimmed); extracted != "" {
		candidates = append(candidates, extracted)
	}

	return dedupeStrings(candidates)
}

func extractCodeFenceJSON(input string) string {
	const fence = "```"
	startFence := strings.Index(input, fence)
	if startFence < 0 {
		return ""
	}

	afterFence := input[startFence+len(fence):]
	newlineIdx := strings.Index(afterFence, "\n")
	if newlineIdx < 0 {
		return ""
	}

	contentWithEndFence := afterFence[newlineIdx+1:]
	endFence := strings.Index(contentWithEndFence, fence)
	if endFence < 0 {
		return ""
	}

	return strings.TrimSpace(contentWithEndFence[:endFence])
}

func extractJSONObject(input string) string {
	start := strings.Index(input, "{")
	end := strings.LastIndex(input, "}")
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	return strings.TrimSpace(input[start : end+1])
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	output := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		output = append(output, value)
	}
	return output
}

func normalizeResult(result *testRunResult) {
	if result.ExecutedCases == nil {
		result.ExecutedCases = []executedCaseResult{}
	}
	if result.Bugs == nil {
		result.Bugs = []bugReportItem{}
	}
	for idx := range result.ExecutedCases {
		if result.ExecutedCases[idx].Evidence == nil {
			result.ExecutedCases[idx].Evidence = []string{}
		}
	}
	for idx := range result.Bugs {
		if result.Bugs[idx].Evidence == nil {
			result.Bugs[idx].Evidence = []string{}
		}
	}
}

func needsBugReport(result testRunResult) bool {
	if strings.EqualFold(result.OverallResult, "failed") || strings.EqualFold(result.OverallResult, "blocked") {
		return true
	}
	if len(result.Bugs) > 0 {
		return true
	}
	for _, testCase := range result.ExecutedCases {
		if strings.EqualFold(testCase.Result, "failed") || strings.EqualFold(testCase.Result, "blocked") {
			return true
		}
	}
	return false
}

func writeJSON(writer io.Writer, payload interface{}) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result JSON: %w", err)
	}
	if _, err := writer.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write result JSON: %w", err)
	}
	return nil
}

func renderBugReport(result testRunResult, sourcePath string, generatedAt time.Time) string {
	problemCases := collectProblemCases(result.ExecutedCases)

	var builder strings.Builder
	builder.WriteString("# Bug Report\n\n")
	builder.WriteString(fmt.Sprintf("- Source Document: `%s`\n", sourcePath))
	builder.WriteString(fmt.Sprintf("- Target URL: `%s`\n", result.TargetURL))
	builder.WriteString(fmt.Sprintf("- Overall Result: `%s`\n", result.OverallResult))
	builder.WriteString(fmt.Sprintf("- Generated At: `%s`\n", generatedAt.Format(time.RFC3339)))
	builder.WriteString("\n")
	builder.WriteString("## Summary\n\n")
	builder.WriteString(result.Summary)
	builder.WriteString("\n\n")

	if len(problemCases) > 0 {
		builder.WriteString("## Failed or Blocked Cases\n\n")
		for _, testCase := range problemCases {
			builder.WriteString(fmt.Sprintf("### %s %s\n\n", testCase.ID, testCase.Title))
			builder.WriteString(fmt.Sprintf("- Result: `%s`\n", testCase.Result))
			builder.WriteString(fmt.Sprintf("- Steps: %s\n", testCase.Steps))
			builder.WriteString(fmt.Sprintf("- Expected: %s\n", testCase.Expected))
			builder.WriteString(fmt.Sprintf("- Actual: %s\n", testCase.Actual))
			if len(testCase.Evidence) > 0 {
				builder.WriteString(fmt.Sprintf("- Evidence: %s\n", strings.Join(testCase.Evidence, "; ")))
			}
			builder.WriteString("\n")
		}
	}

	builder.WriteString("## Bugs\n\n")
	if len(result.Bugs) == 0 {
		builder.WriteString("No explicit bug object was returned, but test execution indicates failures/blockers.\n")
		return builder.String()
	}

	for _, bug := range result.Bugs {
		builder.WriteString(fmt.Sprintf("### %s %s\n\n", bug.ID, bug.Title))
		builder.WriteString(fmt.Sprintf("- Severity: `%s`\n", bug.Severity))
		builder.WriteString(fmt.Sprintf("- Related Case: `%s`\n", bug.RelatedCaseID))
		builder.WriteString(fmt.Sprintf("- Repro Steps: %s\n", bug.StepsToReproduce))
		builder.WriteString(fmt.Sprintf("- Expected: %s\n", bug.Expected))
		builder.WriteString(fmt.Sprintf("- Actual: %s\n", bug.Actual))
		if len(bug.Evidence) > 0 {
			builder.WriteString(fmt.Sprintf("- Evidence: %s\n", strings.Join(bug.Evidence, "; ")))
		}
		builder.WriteString("\n")
	}

	return builder.String()
}

func collectProblemCases(cases []executedCaseResult) []executedCaseResult {
	problemCases := make([]executedCaseResult, 0, len(cases))
	for _, testCase := range cases {
		if strings.EqualFold(testCase.Result, "failed") || strings.EqualFold(testCase.Result, "blocked") {
			problemCases = append(problemCases, testCase)
		}
	}
	return problemCases
}

func buildExecutionFailureResult(execErr error) testRunResult {
	message := strings.TrimSpace(execErr.Error())
	if message == "" {
		message = "unknown execution failure"
	}

	return testRunResult{
		TargetURL:     "unknown",
		OverallResult: "blocked",
		Summary:       "Execution was blocked before structured test result could be produced.",
		ExecutedCases: []executedCaseResult{
			{
				ID:       "EXEC-ERROR",
				Title:    "Agent execution",
				Result:   "blocked",
				Steps:    "Run codex-webtest with provided document",
				Expected: "Agent executes document instructions and returns structured test result",
				Actual:   message,
				Evidence: []string{message},
			},
		},
		Bugs: []bugReportItem{
			{
				ID:               "BUG-EXEC-ERROR",
				Title:            "Test execution blocked",
				Severity:         "high",
				RelatedCaseID:    "EXEC-ERROR",
				StepsToReproduce: "Run codex-webtest with the same input and environment",
				Expected:         "Test execution completes and returns structured output",
				Actual:           message,
				Evidence:         []string{message},
			},
		},
	}
}
