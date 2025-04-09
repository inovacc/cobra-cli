package generator

import (
	"embed"
	"fmt"
	"github.com/spf13/viper"
	"time"
)

type License struct {
	code            string   // The code name of the license
	Name            string   // The type of license in use
	PossibleMatches []string // Similar names to guess
	Header          string   // License header for source files
	Body            string   // License body
	Copyright       string   // Copyright line
}

func newLicense(templates embed.FS) (*License, error) {
	year := viper.GetString("year")
	if year == "" {
		year = time.Now().Format("2006")
	}

	licenseDefinitions := map[string]*License{
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

	def, ok := licenseDefinitions[viper.GetString("license")]
	if !ok {
		def = &License{
			code:            "none",
			Name:            "None",
			PossibleMatches: []string{"none", "false"},
		}
	}

	def.Copyright = fmt.Sprintf("Copyright © %s %s", year, viper.GetString("author"))

	if def.code != "none" {
		if err := def.getLicenseHeader(templates); err != nil {
			return nil, err
		}

		if err := def.getLicenseBody(templates); err != nil {
			return nil, err
		}
	}

	return def, nil
}

func (l *License) getLicenseHeader(templates embed.FS) error {
	data, err := templates.ReadFile(fmt.Sprintf("tpl/header_%s.tmpl", l.code))
	if err != nil {
		return err
	}
	l.Header = string(data)
	return nil
}

func (l *License) getLicenseBody(templates embed.FS) error {
	data, err := templates.ReadFile(fmt.Sprintf("tpl/license_%s.tmpl", l.code))
	if err != nil {
		return err
	}
	l.Body = string(data)
	return nil
}
