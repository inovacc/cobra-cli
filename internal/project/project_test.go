package project

import (
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"testing"
)

var afs = afero.NewMemMapFs()

func TestGenerateRoot(t *testing.T) {
	viper.SetDefault("author", "NAME HERE <EMAIL ADDRESS>")
	viper.SetDefault("license", "apache2")
	viper.SetDefault("projectName", "testApp")
	defer viper.Reset()

	project, err := NewProject([]string{"myproject"})
	if err != nil {
		t.Fatal(err)
	}

	project.SetPkgName("github.com/acme/myproject")

	generator, err := NewProjectGenerator(afs, project)
	if err != nil {
		t.Fatal(err)
	}

	if err := generator.SetLicense(); err != nil {
		t.Fatal(err)
	}

	if err := generator.CreateProject(); err != nil {
		t.Fatalf("Error creating Project: %v", err)
	}

	// Check LICENSE
	if !generator.None {
		assertFileMatchesGolden(t, afs,
			filepath.Join(generator.Project.AbsolutePath, "LICENSE"),
			"testdata/LICENSE.golden")
	}

	// Check main.go
	assertFileMatchesGolden(t, afs,
		filepath.Join(generator.Project.AbsolutePath, "main.go"),
		func() string {
			if generator.None {
				return "testdata/main_none.golden"
			}
			return "testdata/main.go.golden"
		}(),
	)

	// Check cmd/root.go
	assertFileMatchesGolden(t, afs,
		filepath.Join(generator.Project.AbsolutePath, "cmd/root.go"),
		func() string {
			if generator.None {
				return "testdata/root_none.golden"
			}
			return "testdata/root.golden"
		}(),
	)

	// Check config files
	assertFileMatchesGolden(t, afs,
		filepath.Join(generator.Project.AbsolutePath, "internal/config/config.go"),
		"testdata/config.golden")

	assertFileMatchesGolden(t, afs,
		filepath.Join(generator.Project.AbsolutePath, "internal/config/config_test.go"),
		"testdata/config_test.golden")

	// Check service
	assertFileMatchesGolden(t, afs,
		filepath.Join(generator.Project.AbsolutePath, "internal/service/service.go"),
		"testdata/service.golden")
}

func TestGenerateSub(t *testing.T) {
	viper.SetDefault("projectName", "testApp")
	defer viper.Reset()

	project, err := NewProject([]string{"service"})
	if err != nil {
		t.Fatal(err)
	}

	project.SetPkgName("github.com/acme/myproject")

	generator, err := NewProjectGenerator(afs, project)
	if err != nil {
		t.Fatal(err)
	}

	if err := generator.AddCommandProject(); err != nil {
		t.Fatalf("Error creating sub command: %v", err)
	}

	// Check subcommand
	assertFileMatchesGolden(t, afs,
		filepath.Join(generator.Project.AbsolutePath, "service.go"),
		func() string {
			if generator.None {
				return "testdata/add_command_none.golden"
			}
			return "testdata/add_command.golden"
		}(),
	)
}

func assertFileMatchesGolden(t *testing.T, fs afero.Fs, filePath string, goldenPath string) {
	t.Helper()

	exists, err := afero.Exists(fs, filePath)
	if err != nil || !exists {
		t.Fatalf("Expected file does not exist: %s", filePath)
	}

	actual, err := afero.ReadFile(fs, filePath)
	if err != nil {
		t.Fatalf("Error reading generated file: %s\n%v", filePath, err)
	}

	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("Error reading golden file: %s\n%v", goldenPath, err)
	}

	if err := compareContent(actual, expected); err != nil {
		t.Fatalf("Mismatch for %s:\n%v", filePath, err)
	}
}
