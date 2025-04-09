package project

import (
	"embed"
	"errors"
	"fmt"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"os"
	"path"
	"path/filepath"
	"text/template"
)

//go:embed tpl/*.tmpl
var templates embed.FS

type Command struct {
	CmdName   string
	CmdParent string
	*Project
}

type Project struct {
	PkgName      string
	AbsolutePath string
	AppName      string
	Legal        *License
}

func NewProject(args []string) (*Project, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	if len(args) > 0 {
		if args[0] != "." {
			wd = filepath.Join(wd, args[0])
		}
	}

	return &Project{
		AbsolutePath: wd,
		PkgName:      getModImportPath(),
		AppName:      path.Base(wd),
	}, nil
}

func (p *Project) SetPkgName(value string) {
	p.PkgName = value
}

func (p *Project) SetAppName(value string) {
	p.AppName = value
}

func (p *Project) SetAbsolutePath(value string) {
	p.AbsolutePath = value
}

type Generator struct {
	afs       afero.Fs
	templates embed.FS
	none      bool
	project   *Project
}

func NewProjectGenerator(fs afero.Fs, project *Project) (*Generator, error) {
	return &Generator{
		afs:       fs,
		templates: templates,
		project: &Project{
			PkgName:      project.PkgName,
			AbsolutePath: project.AbsolutePath,
			AppName:      project.AppName,
		},
	}, nil
}

func (g *Generator) SetLicense() error {
	license, err := newLicense(templates)
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

func (g *Generator) GetProjectPath() string {
	return g.project.AbsolutePath
}

// CreateProject sets up the project structure and files.
func (g *Generator) CreateProject() error {
	if g.project.Legal == nil {
		return errors.New("no legal project")
	}

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

	if err := g.renderTemplate(g.getFileContentRoot()); err != nil {
		return err
	}

	return nil
}

// AddCommandProject sets up the project structure and files for a new command.
func (g *Generator) AddCommandProject() error {
	// Ensure base directory exists
	if _, err := g.afs.Stat(g.project.AbsolutePath); err != nil {
		if err := g.afs.MkdirAll(g.project.AbsolutePath, 0754); err != nil {
			return err
		}
	}

	if err := g.renderTemplate(g.getFileContentRoot()); err != nil {
		return err
	}

	return nil
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

func (g *Generator) getFileContentRoot() (Content, error) {
	rootPath := fmt.Sprintf("%s/cmd", g.project.AbsolutePath)
	if _, err := g.afs.Stat(rootPath); err != nil {
		if err := g.afs.MkdirAll(rootPath, 0751); err != nil {
			return Content{Dirty: true}, err
		}
	}

	templateName := "tpl/root.tmpl"

	if g.none {
		templateName = "tpl/root_none.tmpl"
	}

	data, err := g.templates.ReadFile(templateName)
	if err != nil {
		return Content{Dirty: true}, err
	}

	return Content{
		Name:            "root",
		FilePath:        fmt.Sprintf("%s/cmd/root.go", g.project.AbsolutePath),
		TemplateContent: string(data),
		Data:            g.project,
	}, nil
}

func (g *Generator) getFileContentSub() (Content, error) {
	subPath := fmt.Sprintf("%s/cmd/%s.go", g.project.AbsolutePath, g.project.AppName)
	if _, err := g.afs.Stat(subPath); err != nil {
		if err := g.afs.MkdirAll(subPath, 0751); err != nil {
			return Content{Dirty: true}, err
		}
	}

	templateName := "tpl/add_command.tmpl"

	if g.none {
		templateName = "tpl/add_command_none.tmpl"
	}

	data, err := g.templates.ReadFile(templateName)
	if err != nil {
		return Content{Dirty: true}, err
	}

	return Content{
		Name:            "add_command", // from input command
		FilePath:        subPath,
		TemplateContent: string(data),
		Data:            g.project,
	}, nil
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
