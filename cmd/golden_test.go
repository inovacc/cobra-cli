package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

func init() {
	// Mute commands.
	addCmd.SetOut(new(bytes.Buffer))
	addCmd.SetErr(new(bytes.Buffer))
	initCmd.SetOut(new(bytes.Buffer))
	initCmd.SetErr(new(bytes.Buffer))
}

// ensureLF converts any \r\n to \n
func ensureLF(content []byte) []byte {
	return bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
}

// compareFiles compares the content of files with pathA and pathB.
// If contents are equal, it returns nil.
// If not, it returns which files are not equal
// and diff (if system has diff command) between these files.
func compareFiles(pathA, pathB string) error {
	contentA, err := os.ReadFile(pathA)
	if err != nil {
		return err
	}
	contentB, err := os.ReadFile(pathB)
	if err != nil {
		return err
	}
	if !bytes.Equal(ensureLF(contentA), ensureLF(contentB)) {
		output := new(bytes.Buffer)
		_, _ = fmt.Fprintf(output, "%q and %q are not equal!\n\n", pathA, pathB)

		diffPath, err := exec.LookPath("diff")
		if err != nil {
			// Don't execute diff if it can't be found.
			return nil
		}
		diffCmd := exec.Command(diffPath, "-u", "--strip-trailing-cr", pathA, pathB)
		diffCmd.Stdout = output
		diffCmd.Stderr = output

		_, _ = fmt.Fprintf(output, "$ diff -u %s %s\n", pathA, pathB)
		if err := diffCmd.Run(); err != nil {
			_, _ = fmt.Fprintf(output, "\n%s", err.Error())
		}
		return errors.New(output.String())
	}
	return nil
}
