package project

import (
	"embed"
	"errors"
	"fmt"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"
)

//go:embed tpl/*.tmpl
var templates embed.FS

type Command struct {
	CmdName   string
	CmdParent string
	*Project
}

type Project struct {
	Args         []string
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
		Args:         args,
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
		project:   project,
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
	if !g.findLicense() {
		return errors.New("no legal project")
	}

	// Ensure base directory exists
	if _, err := g.afs.Stat(g.project.AbsolutePath); err != nil {
		if err := g.afs.MkdirAll(g.project.AbsolutePath, 0754); err != nil {
			return err
		}
	}

	if err := g.renderTemplate(g.getFileContentSub()); err != nil {
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
		Data: Command{
			CmdName: g.validateCmdName(g.project.Args[0]),
			Project: g.project,
		},
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

func (g *Generator) findLicense() bool {
	if _, err := g.afs.Stat(g.project.AbsolutePath); err != nil {
		return false
	}

	licensePath := fmt.Sprintf("%s/LICENSE", g.project.AbsolutePath)

	if _, err := g.afs.Stat(licensePath); err != nil {
		return false
	}

	license, err := afero.ReadFile(g.afs, licensePath)
	if err != nil {
		return false
	}

	// load all licenses from templates
	licenses, err := g.templates.ReadDir("tpl")
	if err != nil {
		return false
	}

	// check if the license content matches any of the templates
	for _, file := range licenses {
		if file.IsDir() {
			continue
		}

		if !strings.Contains(file.Name(), "license_") {
			continue
		}

		templatePath := fmt.Sprintf("tpl/%s", file.Name())

		templateContent, err := g.templates.ReadFile(templatePath)
		if err != nil {
			return false
		}

		if string(license) == string(templateContent) {
			templatePath = path.Base(templatePath)
			licenseCode := strings.TrimSuffix(strings.TrimPrefix(templatePath, "license_"), ".tmpl")
			licenseCode = strings.ReplaceAll(licenseCode, "_", "")

			viper.Set("license", licenseCode)

			return g.SetLicense() == nil
		}
	}

	return true
}

func (g *Generator) validateCmdName(source string) string {
	i := 0
	l := len(source)
	// The output is initialized on demand, then first dash or underscore
	// occurs.
	var output string

	for i < l {
		if source[i] == '-' || source[i] == '_' {
			if output == "" {
				output = source[:i]
			}

			// If it's last rune, and it's dash or underscore,
			// don't add it output and break the loop.
			if i == l-1 {
				break
			}

			// If next character is dash or underscore,
			// just skip the current character.
			if source[i+1] == '-' || source[i+1] == '_' {
				i++
				continue
			}

			// If the current character is dash or underscore,
			// upper next letter and add to output.
			output += string(unicode.ToUpper(rune(source[i+1])))
			// We know, what source[i] is dash or underscore and source[i+1] is
			// uppered character, so make i = i+2.
			i += 2
			continue
		}

		// If the current character isn't dash or underscore,
		// just add it.
		if output != "" {
			output += string(source[i])
		}
		i++
	}

	if output == "" {
		return source // source is initially valid name.
	}
	return output
}
