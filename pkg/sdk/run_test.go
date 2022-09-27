package sdk

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRun(t *testing.T) {
	var repeats string
	for i := 0; i < 2; i++ {
		if repeats != "" {
			repeats += "\n"
		}
		repeats = repeats + "stdout"
	}
	want := fmt.Sprintf(`%s
`, repeats)

	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprintf("%3d", i), func(t *testing.T) {
			t.Parallel()

			cmd := exec.Command("bash", "-c", "echo stdout; echo stdout;")
			//cmd.Stdin = bytes.NewReader([]byte("foo"))

			if r, err := Run(cmd); err != nil {
				t.Fatalf("unexpected error: %v", err)
			} else if d := cmp.Diff(want, r.Output); d != "" {
				t.Fatalf("unexpected output:\n%s", d)
			}
		})
	}
}
