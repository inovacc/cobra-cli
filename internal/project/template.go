package project

import (
	"embed"
	"fmt"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io/fs"
	"text/template"
	"time"
)

//go:embed tpl/*.tmpl
var templates embed.FS

func getLicenseHeader(name string) string {
	data, err := templates.ReadFile(fmt.Sprintf("tpl/header_%s.tmpl", name))
	if err != nil {
		return ""
	}
	return string(data)
}

type License struct {
	code            string   // The code name of the license
	Name            string   // The type of license in use
	PossibleMatches []string // Similar names to guess
	Header          string   // License header for source files
	Copyright       string   // Copyright line
	licensePath     string   // License file path
}

var licenseDefinitions = map[string]License{
	"apache2": {
		code:            "apache_2",
		Name:            "Apache 2.0",
		PossibleMatches: []string{"Apache-2.0", "apache", "apache20", "apache 2.0", "apache2.0", "apache-2.0"},
	},
	"mit": {
		code:            "mit",
		Name:            "MIT License",
		PossibleMatches: []string{"MIT", "mit"},
	},
	"bsd-3": {
		code:            "bsd_clause_3",
		Name:            "NewBSD",
		PossibleMatches: []string{"BSD-3-Clause", "bsd", "newbsd", "3 clause bsd", "3-clause bsd"},
	},
	"bsd-2": {
		code:            "bsd_clause_2",
		Name:            "Simplified BSD License",
		PossibleMatches: []string{"BSD-2-Clause", "freebsd", "simpbsd", "simple bsd", "2-clause bsd", "2 clause bsd", "simplified bsd license"},
	},
	"gpl-2": {
		code:            "gpl_2",
		Name:            "GNU General Public License 2.0",
		PossibleMatches: []string{"GPL-2.0", "gpl2", "gnu gpl2", "gplv2"},
	},
	"gpl-3": {
		code:            "gpl_3",
		Name:            "GNU General Public License 3.0",
		PossibleMatches: []string{"GPL-3.0", "gpl3", "gplv3", "gpl", "gnu gpl3", "gnu gpl"},
	},
	"lgpl": {
		code:            "lgpl",
		Name:            "GNU Lesser General Public License",
		PossibleMatches: []string{"LGPL-3.0", "lgpl", "lesser gpl", "gnu lgpl"},
	},
	"agpl": {
		code:            "agpl",
		Name:            "GNU Affero General Public License",
		PossibleMatches: []string{"AGPL-3.0", "agpl", "affero gpl", "gnu agpl"},
	},
}

func findLicense(name string) License {
	year := viper.GetString("year")
	if year == "" {
		year = time.Now().Format("2006")
	}

	author := viper.GetString("author")
	def, ok := licenseDefinitions[name]
	if !ok {
		def = License{
			code:            "none",
			Name:            "None",
			PossibleMatches: []string{"none", "false"},
		}
	}

	def.Copyright = fmt.Sprintf("Copyright © %s %s", year, author)
	def.Header = getLicenseHeader(def.code)
	def.licensePath = fmt.Sprintf("license_%s.tmpl", def.code)

	return def
}

func renderTemplate(afs afero.Fs, name, filename, patterns string, data any) error {
	file, err := afs.Create(filename)
	if err != nil {
		return err
	}
	defer func(mainFile afero.File) {
		if err := mainFile.Close(); err != nil {
			cobra.CheckErr(err)
		}
	}(file)

	templateFS, err := fs.Sub(templates, "tpl")
	if err != nil {
		return err
	}

	tmpl, err := template.New(name).ParseFS(templateFS, patterns)
	if err != nil {
		return err
	}

	if err := tmpl.ExecuteTemplate(file, patterns, data); err != nil {
		return err
	}
	return nil
}

func mainTemplate(afs afero.Fs, filename string, data any) error {
	return renderTemplate(afs, "main", filename, "main.tmpl", data)
}

func rootTemplate(afs afero.Fs, filename string, data any) error {
	return renderTemplate(afs, "root", filename, "root.tmpl", data)
}

func addCommandTemplate(afs afero.Fs, filename string, data any) error {
	return renderTemplate(afs, "sub", filename, "add_command.tmpl", data)
}

func licenseTemplate(afs afero.Fs, filename string) error {
	userLicense := viper.GetString("license")
	if userLicense != "" {
		license := findLicense(userLicense)
		if err := renderTemplate(afs, "license", filename, license.licensePath, license); err != nil {
			return err
		}
	}
	return nil
}
