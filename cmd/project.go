package cmd

import (
	"fmt"
	"os"
	"text/template"

	"github.com/inovacc/cobra-cli/tpl"
	"github.com/spf13/cobra"
)

// Project contains name, license and paths to projects.
type Project struct {
	// v2
	PkgName      string
	Copyright    string
	AbsolutePath string
	Legal        License
	Viper        bool
	AppName      string
}

type Command struct {
	CmdName   string
	CmdParent string
	*Project
}

func (p *Project) Create() error {
	// check if AbsolutePath exists
	if _, err := os.Stat(p.AbsolutePath); os.IsNotExist(err) {
		// create directory
		if err := os.Mkdir(p.AbsolutePath, 0754); err != nil {
			return err
		}
	}

	// create main.go
	mainFile, err := os.Create(fmt.Sprintf("%s/main.go", p.AbsolutePath))
	if err != nil {
		return err
	}
	defer func(mainFile *os.File) {
		if err := mainFile.Close(); err != nil {
			cobra.CheckErr(err)
		}
	}(mainFile)

	mainTemplate := template.Must(template.New("main").Parse(string(tpl.MainTemplate())))
	err = mainTemplate.Execute(mainFile, p)
	if err != nil {
		return err
	}

	// create cmd/root.go
	if _, err = os.Stat(fmt.Sprintf("%s/cmd", p.AbsolutePath)); os.IsNotExist(err) {
		cobra.CheckErr(os.Mkdir(fmt.Sprintf("%s/cmd", p.AbsolutePath), 0751))
	}
	rootFile, err := os.Create(fmt.Sprintf("%s/cmd/root.go", p.AbsolutePath))
	if err != nil {
		return err
	}
	defer func(rootFile *os.File) {
		if err := rootFile.Close(); err != nil {
			cobra.CheckErr(err)
		}
	}(rootFile)

	rootTemplate := template.Must(template.New("root").Parse(string(tpl.RootTemplate())))
	err = rootTemplate.Execute(rootFile, p)
	if err != nil {
		return err
	}

	// create license
	return p.createLicenseFile()
}

func (p *Project) createLicenseFile() error {
	data := map[string]interface{}{
		"copyright": copyrightLine(),
	}
	licenseFile, err := os.Create(fmt.Sprintf("%s/LICENSE", p.AbsolutePath))
	if err != nil {
		return err
	}
	defer func(licenseFile *os.File) {
		if err := licenseFile.Close(); err != nil {
			cobra.CheckErr(err)
		}
	}(licenseFile)

	licenseTemplate := template.Must(template.New("license").Parse(p.Legal.Text))
	return licenseTemplate.Execute(licenseFile, data)
}

func (c *Command) Create() error {
	cmdFile, err := os.Create(fmt.Sprintf("%s/cmd/%s.go", c.AbsolutePath, c.CmdName))
	if err != nil {
		return err
	}
	defer func(cmdFile *os.File) {
		if err := cmdFile.Close(); err != nil {
			cobra.CheckErr(err)
		}
	}(cmdFile)

	commandTemplate := template.Must(template.New("sub").Parse(string(tpl.AddCommandTemplate())))
	err = commandTemplate.Execute(cmdFile, c)
	if err != nil {
		return err
	}
	return nil
}
