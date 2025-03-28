package cmd

import (
	"fmt"
	"github.com/inovacc/cobra-cli/internal/project"
	"github.com/spf13/afero"
	"os"
	"testing"

	"github.com/spf13/viper"
)

func TestGoldenAddCmd(t *testing.T) {
	viper.Set("useViper", true)
	viper.Set("license", "apache")

	wd, _ := os.Getwd()
	newProject := project.NewProject(afero.NewMemMapFs(), fmt.Sprintf("%s/testproject", wd), "testproject", "github.com/inovacc/testproject")
	command := project.NewCommand("test", parentName, newProject)
	defer func(path string) {
		if err := os.RemoveAll(path); err != nil {
			t.Fatalf("could not remove path %s: %v", path, err)
		}
	}(command.AbsolutePath)

	assertNoErr(t, command.Project.Create())
	assertNoErr(t, command.Create())

	generatedFile := fmt.Sprintf("%s/cmd/%s.go", command.AbsolutePath, command.CmdName)
	goldenFile := fmt.Sprintf("testdata/%s.go.golden", command.CmdName)
	err := project.CompareFiles(generatedFile, goldenFile)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateCmdName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"cmdName", "cmdName"},
		{"cmd_name", "cmdName"},
		{"cmd-name", "cmdName"},
		{"cmd______Name", "cmdName"},
		{"cmd------Name", "cmdName"},
		{"cmd______name", "cmdName"},
		{"cmd------name", "cmdName"},
		{"cmdName-----", "cmdName"},
		{"cmdname-", "cmdname"},
	}

	for _, testCase := range testCases {
		got := project.ValidateCmdName(testCase.input)
		if testCase.expected != got {
			t.Errorf("Expected %q, got %q", testCase.expected, got)
		}
	}
}

func assertNoErr(t *testing.T, e error) {
	if e != nil {
		t.Error(e)
	}
}
