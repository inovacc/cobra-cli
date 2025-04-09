package generator

import (
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	// Set up viper defaults
	viper.SetDefault("author", "NAME HERE <EMAIL ADDRESS>")
	viper.SetDefault("license", "apache2")
	viper.SetDefault("projectName", "testApp")

	// Run the tests
	code := m.Run()

	// Clean up
	viper.Reset()

	os.Exit(code)
}

func TestGenerate(t *testing.T) {
	afs := afero.NewMemMapFs()
	project, err := NewProject([]string{"myproject"})
	if err != nil {
		t.Fatal(err)
	}

	project.SetPkgName("github.com/acme/myproject")
	project.SetAbsolutePath("github.com/acme")

	generator, err := NewGenerator(afs, project)
	if err != nil {
		t.Fatal(err)
	}

	if err := generator.SetLicense(); err != nil {
		t.Fatal(err)
	}

	if err := generator.CreateProject(); err != nil {
		t.Fatalf("Error creating project: %v", err)
	}

	// Check if the LICENSE file was created
	{
		if !generator.none {
			data, err := afero.ReadFile(afs, filepath.Join(generator.project.AbsolutePath, "LICENSE"))
			if err != nil {
				t.Fatalf("Error reading LICENSE file: %v", err)
			}

			templateName := "testdata/LICENSE.golden"

			goldenFile, err := os.ReadFile(templateName)
			if err != nil {
				t.Fatalf("Error reading golden file: %v", err)
			}

			if err := CompareContent(data, goldenFile); err != nil {
				t.Fatalf("Error comparing files: %v", err)
			}
		}
	}

	// Check if the main.go file was created
	{
		data, err := afero.ReadFile(afs, filepath.Join(generator.project.AbsolutePath, "main.go"))
		if err != nil {
			t.Fatalf("Error reading main.go file: %v", err)
		}

		templateName := "testdata/main.go.golden"

		if generator.none {
			templateName = "testdata/main_none.golden"
		}

		goldenFile, err := os.ReadFile(templateName)
		if err != nil {
			t.Fatalf("Error reading golden file: %v", err)
		}

		if err := CompareContent(data, goldenFile); err != nil {
			t.Fatalf("Error comparing files: %v", err)
		}
	}

	// Check if the cmd/root.go file was created
	{
		data, err := afero.ReadFile(afs, filepath.Join(generator.project.AbsolutePath, "cmd/root.go"))
		if err != nil {
			t.Fatalf("Error reading root.go file: %v", err)
		}

		templateName := "testdata/root.golden"

		if generator.none {
			templateName = "testdata/root_none.golden"
		}

		goldenFile, err := os.ReadFile(templateName)
		if err != nil {
			t.Fatalf("Error reading golden file: %v", err)
		}

		if err := CompareContent(data, goldenFile); err != nil {
			t.Fatalf("Error comparing files: %v", err)
		}
	}
}
