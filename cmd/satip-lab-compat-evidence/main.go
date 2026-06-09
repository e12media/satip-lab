package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/e12media/satip-lab/internal/compatibility"
)

func main() {
	input := flag.String("input", "", "sanitized RTSP trace summary JSON")
	profileYAML := flag.String("profile-yaml", "", "optional compatibility profile YAML to check against the trace")
	behaviorYAML := flag.Bool("behavior-yaml", false, "print a compatibility profile behavior YAML snippet")
	flag.Parse()

	if *input == "" {
		fmt.Fprintln(os.Stderr, "--input is required")
		os.Exit(2)
	}
	traceBody, err := os.ReadFile(*input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read trace evidence: %v\n", err)
		os.Exit(1)
	}
	if err := compatibility.ValidateTraceEvidence(traceBody); err != nil {
		fmt.Fprintf(os.Stderr, "validate trace evidence: %v\n", err)
		os.Exit(1)
	}
	if *profileYAML != "" {
		profileBody, err := os.ReadFile(*profileYAML)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read profile YAML: %v\n", err)
			os.Exit(1)
		}
		if err := compatibility.CheckProfileBehaviorAgainstTrace(profileBody, traceBody); err != nil {
			fmt.Fprintf(os.Stderr, "check profile behavior: %v\n", err)
			os.Exit(1)
		}
	}
	if *behaviorYAML {
		body, err := compatibility.TraceEvidenceBehaviorYAML(traceBody)
		if err != nil {
			fmt.Fprintf(os.Stderr, "render behavior YAML: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(string(body))
		return
	}
	fmt.Println("compatibility evidence OK")
}
