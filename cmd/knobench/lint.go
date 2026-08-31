package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/uknoAI/kno-benchmarks/harness"
	"gopkg.in/yaml.v3"
)

// lintCmd enforces the rules that must not depend on anybody reading a
// document: no self-hosted job reachable from a fork PR, no provider-secret
// job without an environment gate, and no paid invocation without both cost
// ceilings.
func lintCmd(argv []string) error {
	fs := flag.NewFlagSet("lint", flag.ExitOnError)
	root := fs.String("root", ".", "repository root")
	matrixPath := fs.String("matrix", "matrix.yaml", "path to the committed matrix")
	if err := fs.Parse(argv); err != nil {
		return err
	}

	var findings []string
	findings = append(findings, harness.LintMatrix(filepath.Join(*root, *matrixPath))...)

	wfDir := filepath.Join(*root, ".github", "workflows")
	entries, err := os.ReadDir(wfDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", wfDir, err)
	}
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yml") && !strings.HasSuffix(e.Name(), ".yaml")) {
			continue
		}
		path := filepath.Join(wfDir, e.Name())
		blob, rerr := os.ReadFile(path) //nolint:gosec // repository-relative path.
		if rerr != nil {
			return fmt.Errorf("read %s: %w", path, rerr)
		}
		findings = append(findings, LintWorkflow(e.Name(), blob)...)
	}

	for _, f := range findings {
		fmt.Fprintln(os.Stderr, "lint: "+f)
	}
	if len(findings) > 0 {
		return fmt.Errorf("%d lint finding(s)", len(findings))
	}
	fmt.Fprintln(os.Stderr, "lint: clean")
	return nil
}

type workflow struct {
	On   yaml.Node                 `yaml:"on"`
	Jobs map[string]workflowJobRaw `yaml:"jobs"`
}

type workflowJobRaw struct {
	RunsOn      yaml.Node `yaml:"runs-on"`
	Environment yaml.Node `yaml:"environment"`
}

// forkTriggers are the events that can carry code from outside the repository.
// A self-hosted runner reachable from one of them is a persistent-compromise
// vector, not a benchmark.
var forkTriggers = []string{"pull_request", "pull_request_target"}

// LintWorkflow checks one workflow file. It is exported so the rule is
// asserted by a test rather than by inspection.
func LintWorkflow(name string, blob []byte) []string {
	var wf workflow
	if err := yaml.Unmarshal(blob, &wf); err != nil {
		return []string{fmt.Sprintf("%s: parse: %v", name, err)}
	}
	triggers := triggerNames(&wf.On)
	forkReachable := false
	for _, t := range triggers {
		for _, ft := range forkTriggers {
			if t == ft {
				forkReachable = true
			}
		}
	}

	text := string(blob)
	usesProviderSecret := mentionsProviderSecret(text)

	var out []string
	for jobName, job := range wf.Jobs {
		runsOn := strings.Join(scalarList(&job.RunsOn), ",")
		if strings.Contains(runsOn, "self-hosted") && forkReachable {
			out = append(out, fmt.Sprintf("%s: job %q runs on a self-hosted runner and the workflow is triggerable by %v", name, jobName, forkTriggers))
		}
		if usesProviderSecret && job.Environment.IsZero() {
			out = append(out, fmt.Sprintf("%s: job %q can reach a provider secret without an `environment:` gate", name, jobName))
		}
	}
	return out
}

func mentionsProviderSecret(text string) bool {
	for _, s := range []string{
		"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GOOGLE_API_KEY",
		"AWS_SECRET_ACCESS_KEY", "LANGSMITH_API_KEY", "LANGFUSE_SECRET_KEY",
		"BRAINTRUST_API_KEY", "HF_TOKEN",
	} {
		if strings.Contains(text, s) {
			return true
		}
	}
	return false
}

func triggerNames(n *yaml.Node) []string {
	if n == nil || n.IsZero() {
		return nil
	}
	switch n.Kind {
	case yaml.ScalarNode:
		return []string{n.Value}
	case yaml.SequenceNode:
		var out []string
		for _, c := range n.Content {
			out = append(out, c.Value)
		}
		return out
	case yaml.MappingNode:
		var out []string
		for i := 0; i < len(n.Content); i += 2 {
			out = append(out, n.Content[i].Value)
		}
		return out
	}
	return nil
}

func scalarList(n *yaml.Node) []string {
	if n == nil || n.IsZero() {
		return nil
	}
	switch n.Kind {
	case yaml.ScalarNode:
		return []string{n.Value}
	case yaml.SequenceNode:
		var out []string
		for _, c := range n.Content {
			out = append(out, c.Value)
		}
		return out
	}
	return nil
}
