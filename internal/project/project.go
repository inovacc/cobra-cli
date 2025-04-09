package project

import (
	"bufio"
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"
	"unicode"
)

//go:embed tpl/*.tmpl
var templates embed.FS

var srcPaths []string

func init() {
	// Initialize srcPaths.
	envGoPath := os.Getenv("GOPATH")
	goPaths := filepath.SplitList(envGoPath)
	if len(goPaths) == 0 {
		// Adapted from https://github.com/Masterminds/glide/pull/798/files.
		// As of Go 1.8 the GOPATH is no longer required to be set. Instead, there
		// is a default value. If there is no GOPATH check for the default value.
		// Note, checking the GOPATH first to avoid invoking the go toolchain if
		// possible.

		goExecutable := os.Getenv("COBRA_GO_EXECUTABLE")
		if len(goExecutable) <= 0 {
			goExecutable = "go"
		}

		out, err := exec.Command(goExecutable, "env", "GOPATH").Output()
		cobra.CheckErr(err)

		toolchainGoPath := strings.TrimSpace(string(out))
		goPaths = filepath.SplitList(toolchainGoPath)
		if len(goPaths) == 0 {
			cobra.CheckErr("$GOPATH is not set")
		}
	}
	srcPaths = make([]string, 0, len(goPaths))
	for _, goPath := range goPaths {
		srcPaths = append(srcPaths, filepath.Join(goPath, "src"))
	}
}

// ensureLF converts any \r\n to \n
func ensureLF(content []byte) []byte {
	return bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
}

func CompareContent(contentA, contentB []byte) error {
	if !bytes.Equal(ensureLF(contentA), ensureLF(contentB)) {
		output := new(bytes.Buffer)
		_, _ = fmt.Fprintf(output, "Contents are not equal!\n\n")

		diffPath, err := exec.LookPath("diff")
		if err != nil {
			// Don't execute diff if it can't be found.
			return nil
		}
		diffCmd := exec.Command(diffPath, "-u", "--strip-trailing-cr")
		diffCmd.Stdin = bytes.NewReader(contentA)
		diffCmd.Stdout = output
		diffCmd.Stderr = output

		if err := diffCmd.Run(); err != nil {
			_, _ = fmt.Fprintf(output, "\n%s", err.Error())
		}
		return errors.New(output.String())
	}
	return nil
}

func getModImportPath() string {
	mod, cd := parseModInfo()
	return path.Join(mod.Path, fileToURL(strings.TrimPrefix(cd.Dir, mod.Dir)))
}

func fileToURL(in string) string {
	i := strings.Split(in, string(filepath.Separator))
	return path.Join(i...)
}

type Mod struct {
	Path, Dir, GoMod string
}

type CurDir struct {
	Dir string
}

func parseModInfo() (Mod, CurDir) {
	var (
		mod Mod
		dir CurDir
	)

	m := modInfoJSON("-m")
	cobra.CheckErr(json.Unmarshal(m, &mod))

	// Unsure why, but if no module is present Path is set to this string.
	if mod.Path == "command-line-arguments" {
		cobra.CheckErr("Please run `go mod init <MODNAME>` before `cobra-cli init`")
	}

	e := modInfoJSON("-e")
	cobra.CheckErr(json.Unmarshal(e, &dir))

	return mod, dir
}

func GoGet(mod string) error {
	return exec.Command("go", "get", mod).Run()
}

func modInfoJSON(args ...string) []byte {
	cmdArgs := append([]string{"list", "-json"}, args...)
	out, err := exec.Command("go", cmdArgs...).Output()
	cobra.CheckErr(err)
	return out
}

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
	CmdName      string
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
	project.CmdName = validateCmdName(project.Args[0])

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

func (g *Generator) CmdName() string {
	return g.project.CmdName
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
	if !g.parentProject() {
		return errors.New("no parent project")
	}

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

	command := Command{
		CmdParent: "rootCmd",
		CmdName:   g.project.CmdName,
		Project:   g.project,
	}

	return Content{
		Name:            "add_command", // from input command
		FilePath:        subPath,
		TemplateContent: string(data),
		Data:            command,
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

			// extract copyright from license
			author, err := g.extractAuthorFromFile(fmt.Sprintf("%s/main.go", g.project.AbsolutePath))
			if err != nil {
				return false
			}

			viper.Set("author", author)

			return g.SetLicense() == nil
		}
	}

	return true
}

func (g *Generator) extractAuthorFromFile(filePath string) (string, error) {
	file, err := g.afs.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func(file afero.File) {
		if err := file.Close(); err != nil {
			log.Println(err)
		}
	}(file)

	re := regexp.MustCompile(`^Copyright © \d{4}\s+`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if re.MatchString(line) {
			return re.ReplaceAllString(line, ""), nil
		}
	}
	return "", fmt.Errorf("no matching copyright line found")
}

func (g *Generator) parentProject() bool {

}

func validateCmdName(source string) string {
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
