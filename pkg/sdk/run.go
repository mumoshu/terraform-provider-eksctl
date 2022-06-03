package sdk

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/armon/circbuf"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/mitchellh/go-linereader"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func Run(cmd *exec.Cmd) (*CommandResult, error) {
	const maxBufSize = 8 * 1024

	// Setup the command
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	cmd.Stdout = pw

	output, _ := circbuf.NewBuffer(maxBufSize)

	// Write everything we read from the pipe to the output buffer too
	tee := io.TeeReader(pr, output)

	// copy the teed output to the UI output
	copyDoneCh := make(chan struct{})
	//o := ctx.Value(schema.ProvOutputKey).(terraform.UIOutput)
	go copyOutput(debugOut{}, tee, copyDoneCh)

	logDebug("starting to run eksctl", strings.Join(cmd.Args, " "))

	cmdToLog := fmt.Sprintf("%s %s", cmd.Path, strings.Join(cmd.Args, " "))

	log.Printf("[DEBUG] starting command %q", cmdToLog)

	// Execute the command to completion
	runErr := cmd.Run()

	logDebug("closing pipe writer", strings.Join(cmd.Args, " "))

	if err := pw.Close(); err != nil {
		return nil, err
	}

	logDebug("waiting for copying output", strings.Join(cmd.Args, " "))

	select {
	case <-copyDoneCh:
		//case <-ctx.Done():
	}

	res := NewCommandResult()

	out := output.String()
	log.Printf("[DEBUG] command %q finished with output: \"%s\"", cmdToLog, out)
	var exitStatus int
	if runErr != nil {
		switch ee := runErr.(type) {
		case *exec.ExitError:
			// Propagate any non-zero exit status from the external command, rather than throwing it away,
			// so that helmfile could return its own exit code accordingly
			waitStatus := ee.Sys().(syscall.WaitStatus)
			exitStatus = waitStatus.ExitStatus()
			if exitStatus != 2 {
				return nil, fmt.Errorf("%s: %v\n%s", cmd.Path, runErr, out)
			}
			res.ExitStatus = exitStatus
		default:
			return nil, fmt.Errorf("running %q: %v\n%s", cmdToLog, runErr, out)
		}
	}

	res.Output = out

	log.Printf("[DEBUG] command new state: \"%v\"", res)

	return res, nil
}

func Hash(data interface{}) string {
	bs, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	first := sha256.New()
	first.Write(bs)
	return fmt.Sprintf("%x", first.Sum(nil))
}

func copyOutput(o Outputter, r io.Reader, doneCh chan<- struct{}) {
	defer close(doneCh)
	lr := linereader.New(r)
	for line := range lr.Ch {
		o.Output(line)
	}
}

func SetOutput(d *schema.ResourceData, v string) {
	d.Set(KeyOutput, v)
}

type debugOut struct{}

func (o debugOut) Output(line string) {
	logDebug("eksctl", line)
}

type Outputter interface {
	Output(string)
}

const KeyOutput = "output"

func logDebug(title string, data interface{}) {
	var buf bytes.Buffer

	d := json.NewEncoder(&buf)
	d.SetIndent("", "  ")

	if err := d.Encode(data); err != nil {
		panic(err)
	}

	log.Printf("[DEBUG] %s: %s", title, buf.String())
}

// CommandResult is a wrapper around both the input and output attributes that are relavent for updates
type CommandResult struct {
	Output     string
	ExitStatus int
}

// NewCommandResult is the constructor for CommandResult
func NewCommandResult() *CommandResult {
	return &CommandResult{}
}
