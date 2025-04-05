package project

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
)

var fs afero.Fs

func init() {
	fs = afero.NewOsFs()
	_, err := git.PlainOpen(filepath.Clean("."))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to open git repository: %s\n", err)
	}
}

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
		AbsolutePath: absPath,
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
	if err := mainTemplate(p.afs, mainPath, p); err != nil {
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
	if err := rootTemplate(p.afs, rootFilePath, p); err != nil {
		return err
	}

	licensePath := fmt.Sprintf("%s/LICENSE", p.AbsolutePath)
	if err := licenseTemplate(p.afs, licensePath); err != nil {
		return err
	}

	return nil
}

// Create generates a new command file under /cmd.
func (c *Command) Create() error {
	cmdFilePath := fmt.Sprintf("%s/cmd/%s.go", c.AbsolutePath, c.CmdName)
	return addCommandTemplate(c.afs, cmdFilePath, c)
}
