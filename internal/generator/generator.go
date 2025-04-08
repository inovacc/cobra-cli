package generator

import (
	"embed"
	"fmt"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"path/filepath"
	"text/template"
)

//go:embed tpl/*.tmpl
var templates embed.FS

type Generator struct {
	afs       afero.Fs
	templates embed.FS
	none      bool
	project   *Project
}

func NewGenerator(fs afero.Fs, pkgName, appName string) (*Generator, error) {
	exists, err := afero.Exists(fs, "go.mod")
	if err != nil {
		return nil, err
	}

	if !exists {
		file, err := fs.Create("go.mod")
		if err != nil {
			return nil, fmt.Errorf("error creating go.mod file %v", err)
		}
		defer func(mainFile afero.File) {
			if err := mainFile.Close(); err != nil {
				cobra.CheckErr(err)
			}
		}(file)

		if _, err := file.WriteString("module github.com/acme/myproject\n\ngo 1.24"); err != nil {
			return nil, fmt.Errorf("error writing to go.mod file %v", err)
		}
	}

	file, err := fs.Open("go.mod")
	if err != nil {
		return nil, fmt.Errorf("error opening go.mod file %v", err)
	}

	return &Generator{
		afs:       fs,
		templates: templates,
		project: &Project{
			PkgName:      pkgName,
			AbsolutePath: filepath.Dir(file.Name()),
			AppName:      appName,
		},
	}, nil
}

func (g *Generator) SetLicense(name string) error {
	license, err := newLicense(name, templates)
	if err != nil {
		return err
	}
	g.project.Legal = license
	g.none = g.project.Legal.code == "none"
	return nil
}

type Content struct {
	Dirty           bool
	Name            string
	FilePath        string
	TemplateContent string
	Data            any
}

func (g *Generator) getFileContentMain() (Content, error) {
	templateName := "tpl/main.tmpl"

	if g.none {
		templateName = "tpl/main_none.tmpl"
	}

	data, err := g.templates.ReadFile(templateName)
	if err != nil {
		return Content{
			Dirty: true,
		}, err
	}

	return Content{
		Name:            "main",
		FilePath:        fmt.Sprintf("%s/main.go", g.project.AbsolutePath),
		TemplateContent: string(data),
		Data:            g.project,
	}, nil
}

func (g *Generator) getFileContentLicense() (Content, error) {
	if !g.none {
		templateName := fmt.Sprintf("tpl/license_%s.tmpl", g.project.Legal.code)

		data, err := g.templates.ReadFile(templateName)
		if err != nil {
			return Content{Dirty: true}, err
		}

		return Content{
			Name:            "license",
			FilePath:        fmt.Sprintf("%s/LICENSE", g.project.AbsolutePath),
			TemplateContent: string(data),
			Data:            g.project.Legal,
		}, nil
	}

	return Content{Dirty: true}, nil
}

// CreateProject sets up the project structure and files.
func (g *Generator) CreateProject() error {
	// Ensure base directory exists
	if _, err := g.afs.Stat(g.project.AbsolutePath); err != nil {
		if err := g.afs.MkdirAll(g.project.AbsolutePath, 0754); err != nil {
			return err
		}
	}

	if err := g.renderTemplate(g.getFileContentLicense()); err != nil {
		return err
	}

	if err := g.renderTemplate(g.getFileContentMain()); err != nil {
		return err
	}

	//filesMap["main"] = mainContent
	//filesMap["root"] = fmt.Sprintf("%s/cmd/root.go", g.project.AbsolutePath)
	//filesMap["license"] = fmt.Sprintf("%s/LICENSE", g.project.AbsolutePath)

	// Ensure /cmd directory exists
	//if _, err := g.afs.Stat(filesMap["root"]); err != nil {
	//	if err := g.afs.MkdirAll(filesMap["root"], 0751); err != nil {
	//		return err
	//	}
	//}

	//if err := mainTemplate(filesMap["main"] , g); err != nil {
	//	return err
	//}
	//
	//// Create cmd/root.go
	//if err := rootTemplate(filesMap["root"], g); err != nil {
	//	return err
	//}
	//
	//if err := licenseTemplate( filesMap["license"]); err != nil {
	//	return err
	//}

	return nil
}

func (g *Generator) renderTemplate(content Content, err error) error {
	if content.Dirty {
		return nil
	}

	file, err := g.afs.Create(content.FilePath)
	if err != nil {
		return err
	}
	defer func(mainFile afero.File) {
		if err := mainFile.Close(); err != nil {
			cobra.CheckErr(err)
		}
	}(file)

	tmpl, err := template.New(content.Name).Parse(content.TemplateContent)
	if err != nil {
		return err
	}

	if err := tmpl.Execute(file, content.Data); err != nil {
		return err
	}
	return nil
}

//
//func mainTemplate(afs afero.Fs, filename string, data any, none bool) error {
//	return renderTemplate(afs, "main", filename, "main.tmpl", data, none)
//}
//
//func rootTemplate(afs afero.Fs, filename string, data any, none bool) error {
//	return renderTemplate(afs, "root", filename, "root.tmpl", data, none)
//}
//
//func addCommandTemplate(afs afero.Fs, filename string, data any, none bool) error {
//	return renderTemplate(afs, "sub", filename, "add_command.tmpl", data, none)
//}
//
//func licenseTemplate(afs afero.Fs, filename string) error {
//	userLicense := viper.GetString("license")
//	license := findLicense(userLicense)
//	if license.code != "none" {
//		if err := renderTemplate(afs, "license", filename, license.licensePath, license); err != nil {
//			return err
//		}
//	}
//	return nil
//}
