package cliflags

import (
	"flag"
	"testing"
)

func TestRepeatableVerbose(t *testing.T) {
	var c Common
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	c.Bind(fs)
	if err := fs.Parse([]string{"-v", "-v", "-v"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Verbose != 3 {
		t.Errorf("expected Verbose=3 after three -v, got %d", c.Verbose)
	}
}

func TestJSONImpliesNonInteractiveAndNoColor(t *testing.T) {
	var c Common
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	c.Bind(fs)
	if err := fs.Parse([]string{"--json"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	c.Resolve()
	if !c.NonInteractive || !c.NoColor {
		t.Errorf("--json should imply --non-interactive and --no-color, got NI=%v NoColor=%v", c.NonInteractive, c.NoColor)
	}
}

func TestEnvVarsRespected(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("CI", "true")
	var c Common
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	c.Bind(fs)
	_ = fs.Parse(nil)
	c.Resolve()
	if !c.NoColor {
		t.Errorf("NO_COLOR should set NoColor")
	}
	if !c.NonInteractive {
		t.Errorf("CI should set NonInteractive")
	}
}

func TestYesShortAlias(t *testing.T) {
	var c Common
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	c.Bind(fs)
	if err := fs.Parse([]string{"-y"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !c.Yes {
		t.Errorf("-y should set Yes")
	}
}
