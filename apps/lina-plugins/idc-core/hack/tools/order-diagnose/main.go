// Command order-diagnose summarizes one local order through authenticated GETs.
// Credentials stay in memory and are never included in reports or error text.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

// main owns CLI parsing and bounded execution; diagnose.go owns API projection.
func main() {
	if err := runCLI(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// runCLI supports normal environment credentials or an explicitly selected
// private session file. It never discovers credentials or attempts login.
func runCLI(args []string, out, errOut io.Writer) error {
	var (
		flags   = flag.NewFlagSet("order-diagnose", flag.ContinueOnError)
		base    = flags.String("base-url", "http://127.0.0.1:6030", "Local API origin; localhost or a loopback IP only")
		order   = flags.String("order", "", "Exact order number")
		session = flags.String("session-file", "", "Explicit private JSON session file containing accessToken")
	)
	flags.SetOutput(errOut)
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("invalid arguments; use --help")
	}
	if flags.NArg() != 0 || !referencePattern.MatchString(*order) || len(*order) > 40 {
		return fmt.Errorf("provide a valid exact order number with --order")
	}
	client, err := newDiagnosticClient(*base, os.Getenv("LINA_DIAG_TOKEN"), *session)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	report := client.diagnose(ctx, *order)
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(report); err != nil {
		return fmt.Errorf("could not write the diagnostic summary")
	}
	if !report.Complete {
		return fmt.Errorf("diagnostic evidence is incomplete; inspect issues and do not provision again based on this report")
	}
	return nil
}
