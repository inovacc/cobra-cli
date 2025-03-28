package project

import (
	"fmt"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"text/template"

	"github.com/inovacc/cobra-cli/tpl"
	"github.com/spf13/cobra"
)

type Project struct {
	afs          afero.Fs
	PkgName      string
	Copyright    string
	AbsolutePath string
	Legal        License
	Viper        bool
	AppName      string
}

// NewProject creates a new project structure.
func NewProject(fs afero.Fs, absPath, pkgName, appName string) *Project {
	return &Project{
		afs:          fs,
		PkgName:      pkgName,
		Copyright:    CopyrightLine(),
		AbsolutePath: absPath,
		Legal:        GetLicense(),
		Viper:        viper.GetBool("useViper"),
		AppName:      appName,
	}
}

type Command struct {
	CmdName   string
	CmdParent string
	*Project
}

// NewCommand creates a new command structure.
func NewCommand(cmdName, cmdParent string, project *Project) *Command {
	return &Command{
		CmdName:   cmdName,
		CmdParent: cmdParent,
		Project:   project,
	}
}

// Create sets up the project structure and files.
func (p *Project) Create() error {
	// Ensure base directory exists
	if _, err := p.afs.Stat(p.AbsolutePath); err != nil {
		if err := p.afs.MkdirAll(p.AbsolutePath, 0754); err != nil {
			return err
		}
	}

	// Create main.go
	mainPath := fmt.Sprintf("%s/main.go", p.AbsolutePath)
	mainFile, err := p.afs.Create(mainPath)
	if err != nil {
		return err
	}
	defer func(mainFile afero.File) {
		if err := mainFile.Close(); err != nil {
			cobra.CheckErr(err)
		}
	}(mainFile)

	mainTemplate := template.Must(template.New("main").Parse(string(tpl.MainTemplate())))
	if err := mainTemplate.Execute(mainFile, p); err != nil {
		return err
	}

	// Ensure /cmd directory exists
	cmdPath := fmt.Sprintf("%s/cmd", p.AbsolutePath)
	if _, err := p.afs.Stat(cmdPath); err != nil {
		if err := p.afs.MkdirAll(cmdPath, 0751); err != nil {
			return err
		}
	}

	// Create cmd/root.go
	rootFilePath := fmt.Sprintf("%s/root.go", cmdPath)
	rootFile, err := p.afs.Create(rootFilePath)
	if err != nil {
		return err
	}
	defer func(rootFile afero.File) {
		if err := rootFile.Close(); err != nil {
			cobra.CheckErr(err)
		}
	}(rootFile)

	rootTemplate := template.Must(template.New("root").Parse(string(tpl.RootTemplate())))
	if err := rootTemplate.Execute(rootFile, p); err != nil {
		return err
	}
	return p.createLicenseFile()
}

func (p *Project) createLicenseFile() error {
	data := map[string]any{
		"copyright": CopyrightLine(),
	}
	licensePath := fmt.Sprintf("%s/LICENSE", p.AbsolutePath)
	licenseFile, err := p.afs.Create(licensePath)
	if err != nil {
		return err
	}
	defer func(licenseFile afero.File) {
		if err := licenseFile.Close(); err != nil {
			cobra.CheckErr(err)
		}
	}(licenseFile)

	licenseTemplate := template.Must(template.New("license").Parse(p.Legal.Text))
	return licenseTemplate.Execute(licenseFile, data)
}

// Create generates a new command file under /cmd.
func (c *Command) Create() error {
	cmdFilePath := fmt.Sprintf("%s/cmd/%s.go", c.AbsolutePath, c.CmdName)
	cmdFile, err := c.afs.Create(cmdFilePath)
	if err != nil {
		return err
	}
	defer func(cmdFile afero.File) {
		if err := cmdFile.Close(); err != nil {
			cobra.CheckErr(err)
		}
	}(cmdFile)

	commandTemplate := template.Must(template.New("sub").Parse(string(tpl.AddCommandTemplate())))
	return commandTemplate.Execute(cmdFile, c)
}
