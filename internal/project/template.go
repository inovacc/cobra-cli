package project

import (
	"embed"
	"fmt"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"text/template"
	"time"
)

//go:embed /tpl/*.tmpl
var templates embed.FS

func generateTemplate(afs afero.Fs, name, filename, patterns string, data any) error {
	file, err := afs.Create(filename)
	if err != nil {
		return err
	}
	defer func(mainFile afero.File) {
		if err := mainFile.Close(); err != nil {
			cobra.CheckErr(err)
		}
	}(file)

	tmpl, err := template.New(name).ParseFS(templates, patterns)
	if err != nil {
		return err
	}

	if err := tmpl.Execute(file, data); err != nil {
		return err
	}
	return nil
}

func mainTemplate(afs afero.Fs, filename string, data any) error {
	return generateTemplate(afs, "main", filename, "tpl/main.tmpl", data)
}

func rootTemplate(afs afero.Fs, filename string, data any) error {
	return generateTemplate(afs, "root", filename, "tpl/root.tmpl", data)
}

func addCommandTemplate(afs afero.Fs, filename string, data any) error {
	return generateTemplate(afs, "sub", filename, "tpl/add_command.tmpl", data)
}

func licenseTemplate(afs afero.Fs, filename string) error {
	userLicense := viper.GetString("license")
	if userLicense != "" {
		license := findLicense(userLicense)
		if err := generateTemplate(afs, "license", filename, license.LicensePath, license); err != nil {
			return err
		}
	}
	return nil
}

type License struct {
	code            string   // The code name of the license
	Name            string   // The type of license in use
	PossibleMatches []string // Similar names to guess
	Text            string   // License text data
	Header          string   // License header for source files
	Copyright       string   // Copyright line
	LicensePath     string   // License file path
}

func findLicense(name string) License {
	year := viper.GetString("year")
	if year == "" {
		year = time.Now().Format("2006")
	}

	license := License{
		Copyright: fmt.Sprintf("Copyright © %s %s", year, viper.GetString("author")),
	}

	switch name {
	case "apache2":
		license.code = "apache_2"
		license.Name = "Apache 2.0"
		license.PossibleMatches = []string{"Apache-2.0", "apache", "apache20", "apache 2.0", "apache2.0", "apache-2.0"}
	case "mit":
		license.code = "mit"
		license.Name = "MIT License"
		license.PossibleMatches = []string{"MIT", "mit"}
	case "bsd-3":
		license.code = "bsd_clause_3"
		license.Name = "NewBSD"
		license.PossibleMatches = []string{"BSD-3-Clause", "bsd", "newbsd", "3 clause bsd", "3-clause bsd"}
	case "bsd-2":
		license.code = "bsd_clause_2"
		license.Name = "Simplified BSD License"
		license.PossibleMatches = []string{"BSD-2-Clause", "freebsd", "simpbsd", "simple bsd", "2-clause bsd", "2 clause bsd", "simplified bsd license"}
	case "gpl-2":
		license.code = "gpl_2"
		license.Name = "GNU General Public License 2.0"
		license.PossibleMatches = []string{"GPL-2.0", "gpl2", "gnu gpl2", "gplv2"}
	case "gpl-3":
		license.code = "gpl_3"
		license.Name = "GNU General Public License 3.0"
		license.PossibleMatches = []string{"GPL-3.0", "gpl3", "gplv3", "gpl", "gnu gpl3", "gnu gpl"}
	case "lgpl":
		license.code = "lgpl"
		license.Name = "GNU Lesser General Public License"
		license.PossibleMatches = []string{"LGPL-3.0", "lgpl", "lesser gpl", "gnu lgpl"}
	case "agpl":
		license.code = "agpl"
		license.Name = "GNU Affero General Public License"
		license.PossibleMatches = []string{"AGPL-3.0", "agpl", "affero gpl", "gnu agpl"}
	default:
		license.code = "none"
		license.Name = "None"
		license.PossibleMatches = []string{"none", "false"}
	}

	license.LicensePath = fmt.Sprintf("tpl/license_%s.tmpl", license.code)
	return license
}
