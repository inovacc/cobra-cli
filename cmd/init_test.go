package cmd

import (
	"fmt"
	"github.com/inovacc/cobra-cli/internal/project"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func getProject() *project.Project {
	wd, _ := os.Getwd()
	return &project.Project{
		AbsolutePath: fmt.Sprintf("%s/testproject", wd),
		Legal:        project.GetLicense(),
		Copyright:    project.CopyrightLine(),
		AppName:      "cmd",
		PkgName:      "github.com/inovacc/cobra-cli",
		Viper:        true,
	}
}

func TestGoldenInitCmd(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		pkgName   string
		expectErr bool
	}{
		{
			name:      "successfully creates a project based on module",
			args:      []string{"testproject"},
			pkgName:   "github.com/inovacc/testproject",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set("useViper", true)
			viper.Set("license", "apache")
			projectPath, err := project.InitializeProject(tt.args)
			defer func() {
				if projectPath != "" {
					if err := os.RemoveAll(projectPath); err != nil {
						t.Fatalf("could not remove path %s: %v", projectPath, err)
					}
				}
			}()

			if !tt.expectErr && err != nil {
				t.Fatalf("did not expect an error, got %s", err)
			}
			if tt.expectErr {
				if err == nil {
					t.Fatal("expected an error but got none")
				} else {
					// got an expected error nothing more to do
					return
				}
			}

			expectedFiles := []string{"LICENSE", "main.go", "cmd/root.go"}
			for _, f := range expectedFiles {
				generatedFile := fmt.Sprintf("%s/%s", projectPath, f)
				goldenFile := fmt.Sprintf("testdata/%s.golden", filepath.Base(f))
				if err := project.CompareFiles(generatedFile, goldenFile); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
