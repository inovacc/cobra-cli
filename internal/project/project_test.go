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
	viper.SetDefault("license", "mit")
	viper.SetDefault("projectName", "testApp")
	defer viper.Reset()

	project, err := NewProject([]string{"myproject"})
	if err != nil {
		t.Fatal(err)
	}

	project.SetPkgName("github.com/acme/myproject")
	//project.SetAbsolutePath("github.com/acme")

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

	// Check if the LICENSE file was created
	{
		if !generator.None {
			data, err := afero.ReadFile(afs, filepath.Join(generator.Project.AbsolutePath, "LICENSE"))
			if err != nil {
				t.Fatalf("Error reading LICENSE file: %v", err)
			}

			templateName := "testdata/LICENSE.golden"

			goldenFile, err := os.ReadFile(templateName)
			if err != nil {
				t.Fatalf("Error reading golden file: %v", err)
			}

			if err := compareContent(data, goldenFile); err != nil {
				t.Fatalf("Error comparing files: %v", err)
			}
		}
	}

	// Check if the main.go file was created
	{
		data, err := afero.ReadFile(afs, filepath.Join(generator.Project.AbsolutePath, "main.go"))
		if err != nil {
			t.Fatalf("Error reading main.go file: %v", err)
		}

		templateName := "testdata/main.go.golden"

		if generator.None {
			templateName = "testdata/main_none.golden"
		}

		goldenFile, err := os.ReadFile(templateName)
		if err != nil {
			t.Fatalf("Error reading golden file: %v", err)
		}

		if err := compareContent(data, goldenFile); err != nil {
			t.Fatalf("Error comparing files: %v", err)
		}
	}

	// Check if the cmd/root.go file was created
	{
		data, err := afero.ReadFile(afs, filepath.Join(generator.Project.AbsolutePath, "cmd/root.go"))
		if err != nil {
			t.Fatalf("Error reading root.go file: %v", err)
		}

		templateName := "testdata/root.golden"

		if generator.None {
			templateName = "testdata/root_none.golden"
		}

		goldenFile, err := os.ReadFile(templateName)
		if err != nil {
			t.Fatalf("Error reading golden file: %v", err)
		}

		if err := compareContent(data, goldenFile); err != nil {
			t.Fatalf("Error comparing files: %v", err)
		}
	}
}

func TestGenerateSub(t *testing.T) {
	viper.SetDefault("projectName", "testApp")
	defer viper.Reset()

	project, err := NewProject([]string{"service"})
	if err != nil {
		t.Fatal(err)
	}

	project.SetPkgName("github.com/acme/myproject")
	project.SetAbsolutePath("github.com/acme")

	generator, err := NewProjectGenerator(afs, project)
	if err != nil {
		t.Fatal(err)
	}

	if err := generator.AddCommandProject(); err != nil {
		t.Fatalf("Error creating sub command: %v", err)
	}

	// Check if the cmd/service.go file was created
	{
		data, err := afero.ReadFile(afs, filepath.Join(generator.Project.AbsolutePath, "cmd/service.go"))
		if err != nil {
			t.Fatalf("Error reading root.go file: %v", err)
		}

		templateName := "testdata/add_command.golden"

		if generator.None {
			templateName = "testdata/add_command_none.golden"
		}

		goldenFile, err := os.ReadFile(templateName)
		if err != nil {
			t.Fatalf("Error reading golden file: %v", err)
		}

		if err := compareContent(data, goldenFile); err != nil {
			t.Fatalf("Error comparing files: %v", err)
		}
	}
}
