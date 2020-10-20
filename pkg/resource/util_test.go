package resource

import (
	"bytes"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"os/exec"
	"strings"
	"testing"
)

func TestRun_err(t *testing.T) {
	var repeats string
	for i := 0; i<1000; i++ {
		if repeats != "" {
			repeats += "\n"
		}
		repeats = repeats + "stdout"
	}
	want := fmt.Sprintf(`command "/bin/bash": exit status 1
%s
stderr
`, repeats)

	for i := 0; i<100; i++ {
		t.Run(fmt.Sprintf("%3d", i), func(t *testing.T) {
			t.Parallel()

			cmd := exec.Command("bash", "-c", "yes stdout | head -n 1000; echo stderr 1>&2; exit 1")
			cmd.Stdin = bytes.NewReader([]byte("foo"))

			if _, err := Run(cmd); err != nil {
				if d := cmp.Diff(want, err.Error()); d != "" {
					t.Fatalf("running %s %s:\n%s", cmd.Path, strings.Join(cmd.Args, " "), d)
				}
			} else {
				t.Fatal("no expected error occuered")
			}
		})
	}
}

func TestRun(t *testing.T) {var repeats string
	for i := 0; i<1000; i++ {
		if repeats != "" {
			repeats += "\n"
		}
		repeats = repeats + "stdout"
	}
	want := fmt.Sprintf(`%s
stderr
`, repeats)

	for i := 0; i<100; i++ {
		t.Run(fmt.Sprintf("%3d", i), func(t *testing.T) {
			t.Parallel()

			cmd := exec.Command("bash", "-c", "yes stdout | head -n 1000; echo stderr 1>&2")
			cmd.Stdin = bytes.NewReader([]byte("foo"))

			if r, err := Run(cmd); err != nil {
				t.Fatalf("unexpected error: %v", err)
			} else if d := cmp.Diff(want, r.Output); d != "" {
				t.Fatalf("unexpected output:\n%s", d)
			}
		})
	}
}
