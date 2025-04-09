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
	"gopkg.in/yaml.v3"
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
	Legal        *License `json:"-" yaml:"-"`
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
	Afs       afero.Fs `json:"-" yaml:"-"`
	Templates embed.FS `json:"-" yaml:"-"`
	None      bool
	Project   *Project
	Content   []Content
}

func NewProjectGenerator(fs afero.Fs, project *Project) (*Generator, error) {
	project.CmdName = validateCmdName(project.Args)

	return &Generator{
		Afs:       fs,
		Templates: templates,
		Project:   project,
		Content:   []Content{},
	}, nil
}

func (g *Generator) SetLicense() error {
	license, err := newLicense(templates)
	if err != nil {
		return err
	}
	g.Project.Legal = license
	g.None = g.Project.Legal.Code == "None"
	return nil
}

type Content struct {
	Dirty            bool
	Name             string
	FilePath         string
	TemplateFilePath string
	TemplateContent  string
	Data             any
}

func (g *Generator) GetProjectPath() string {
	return g.Project.AbsolutePath
}

func (g *Generator) CmdName() string {
	return g.Project.CmdName
}

// CreateProject sets up the Project structure and files.
func (g *Generator) CreateProject() error {
	if g.Project.Legal == nil {
		return errors.New("no legal Project")
	}

	if err := g.getFileContentLicense(); err != nil {
		return err
	}

	// Ensure base directory exists
	if !stat(g.Afs, g.Project.PkgName) {
		if err := g.Afs.MkdirAll(g.Project.PkgName, 0754); err != nil {
			return err
		}
	}

	if err := g.getFileContentMain(); err != nil {
		return err
	}

	if err := g.getFileContentRoot(); err != nil {
		return err
	}

	//TODO for debuging purpose
	{
		file, _ := os.OpenFile("data.yaml", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		defer file.Close()

		if err := yaml.NewEncoder(file).Encode(g); err != nil {
			return err
		}
	}

	//if err := g.renderTemplate(); err != nil {
	//	return err
	//}

	return nil
}

// AddCommandProject sets up the Project structure and files for a new command.
func (g *Generator) AddCommandProject() error {
	rootFilePath := filepath.Join(g.Project.PkgName, "cmd", "root.go")
	if !stat(g.Afs, rootFilePath) {
		return fmt.Errorf("no root file found on: %s", rootFilePath)
	}

	if !g.findLicense() {
		return errors.New("no legal Project")
	}

	// Ensure base directory exists
	if stat(g.Afs, g.Project.AbsolutePath) {
		if err := g.Afs.MkdirAll(g.Project.AbsolutePath, 0754); err != nil {
			return err
		}
	}

	if err := g.getFileContentSub(); err != nil {
		return err
	}

	//if err := g.renderTemplate(); err != nil {
	//	return err
	//}

	return nil
}

func (g *Generator) getFileContentMain() error {
	content := Content{
		Name:             "main",
		TemplateFilePath: "tpl/main.tmpl",
		FilePath:         fmt.Sprintf("%s/main.go", g.Project.AbsolutePath),
		Dirty:            true,
	}

	if g.None {
		content.TemplateFilePath = "tpl/main_none.tmpl"
	}

	defer func() {
		g.Content = append(g.Content, content)
	}()

	data, err := g.Templates.ReadFile(content.TemplateFilePath)
	if err != nil {
		return err
	}

	content.TemplateContent = string(data)
	content.Data = g.Project
	content.Dirty = false

	g.Content = append(g.Content, content)
	return nil
}

func (g *Generator) getFileContentLicense() error {
	content := Content{
		Name:             "license",
		FilePath:         fmt.Sprintf("%s/LICENSE", g.Project.PkgName),
		TemplateFilePath: fmt.Sprintf("tpl/license_%s.tmpl", g.Project.Legal.Code),
		Dirty:            true,
	}

	defer func() {
		g.Content = append(g.Content, content)
	}()

	if !g.None {
		data, err := g.Templates.ReadFile(content.TemplateFilePath)
		if err != nil {
			return err
		}

		content.TemplateContent = string(data)
		content.Data = g.Project.Legal
		content.Dirty = false
	}

	return nil
}

func (g *Generator) getFileContentRoot() error {
	content := Content{
		Name:             "root",
		TemplateFilePath: "tpl/root.tmpl",
		FilePath:         fmt.Sprintf("%s/cmd", g.Project.PkgName),
		Dirty:            true,
	}

	if g.None {
		content.TemplateFilePath = "tpl/root_none.tmpl"
	}

	defer func() {
		g.Content = append(g.Content, content)
	}()

	if !stat(g.Afs, content.FilePath) {
		if err := g.Afs.MkdirAll(content.FilePath, 0751); err != nil {
			return err
		}
	}

	content.FilePath = fmt.Sprintf("%s/root.go", g.Project.PkgName)

	data, err := g.Templates.ReadFile(content.TemplateFilePath)
	if err != nil {
		return err
	}

	content.TemplateContent = string(data)
	content.Data = g.Project
	content.Dirty = false

	return nil
}

func (g *Generator) getFileContentSub() error {
	content := Content{
		Name:             "add_command",
		FilePath:         fmt.Sprintf("%s/cmd/%s.go", g.Project.AbsolutePath, g.Project.AppName),
		TemplateFilePath: "tpl/add_command.tmpl",
		Dirty:            true,
	}

	if g.None {
		content.TemplateFilePath = "tpl/add_command_none.tmpl"
	}

	defer func() {
		g.Content = append(g.Content, content)
	}()

	if !stat(g.Afs, content.FilePath) {
		if err := g.Afs.MkdirAll(content.FilePath, 0751); err != nil {
			return err
		}
	}

	data, err := g.Templates.ReadFile(content.TemplateFilePath)
	if err != nil {
		return err
	}

	content.TemplateContent = string(data)
	content.Data = Command{
		CmdParent: "rootCmd",
		CmdName:   g.Project.CmdName,
		Project:   g.Project,
	}
	content.Dirty = false

	return nil
}

func (g *Generator) renderTemplate() error {
	for _, content := range g.Content {
		if content.Dirty {
			continue
		}

		if err := renderFileContent(g.Afs, content); err != nil {
			return err
		}
	}
	return nil
}

func (g *Generator) findLicense() bool {
	if !stat(g.Afs, g.Project.AbsolutePath) {
		return false
	}

	licensePath := fmt.Sprintf("%s/LICENSE", g.Project.AbsolutePath)
	if !stat(g.Afs, licensePath) {
		return false
	}

	license, err := afero.ReadFile(g.Afs, licensePath)
	if err != nil {
		return false
	}

	// load all licenses from Templates
	licenses, err := g.Templates.ReadDir("tpl")
	if err != nil {
		return false
	}

	// check if the license Content matches any of the Templates
	for _, file := range licenses {
		if file.IsDir() {
			continue
		}

		if !strings.Contains(file.Name(), "license_") {
			continue
		}

		templatePath := fmt.Sprintf("tpl/%s", file.Name())
		templateContent, err := g.Templates.ReadFile(templatePath)
		if err != nil {
			return false
		}

		if string(license) == string(templateContent) {
			licenseCode := strings.TrimSuffix(strings.TrimPrefix(path.Base(templatePath), "license_"), ".tmpl")
			licenseCode = strings.ReplaceAll(licenseCode, "_", "")

			viper.Set("license", licenseCode)

			// extract copyright from license
			author, err := g.extractAuthorFromFile(fmt.Sprintf("%s/main.go", g.Project.AbsolutePath))
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
	file, err := g.Afs.Open(filePath)
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

func renderFileContent(afs afero.Fs, content Content) error {
	file, err := afs.Create(content.FilePath)
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

	return tmpl.Execute(file, content.Data)
}

func stat(afs afero.Fs, namePath string) bool {
	if _, err := afs.Stat(namePath); err != nil {
		return false
	}
	return true
}

func validateCmdName(args []string) string {
	var source string
	if len(args) > 0 {
		source = args[0]
	}
	i := 0
	l := len(args)
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
			// We know what args[i] is dash or underscore and args[i+1] is
			// uppered character, so make i = i+2.
			i += 2
			continue
		}

		// If the current character isn't dash or underscore,
		// just add it.
		if output != "" {
			output += args[i]
		}
		i++
	}

	if output == "" {
		return source // args is initially valid name.
	}
	return output
}

type License struct {
	Code            string   // The Code name of the license
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
			Code:            "apache_2",
			Name:            "Apache 2.0",
			PossibleMatches: []string{"Apache-2.0", "apache", "apache20", "apache 2.0", "apache2.0", "apache-2.0"},
		},
		"mit": {
			Code:            "mit",
			Name:            "MIT License",
			PossibleMatches: []string{"MIT", "mit"},
		},
		"bsd-3": {
			Code:            "bsd_clause_3",
			Name:            "NewBSD",
			PossibleMatches: []string{"BSD-3-Clause", "bsd", "newbsd", "3 clause bsd", "3-clause bsd"},
		},
		"bsd-2": {
			Code:            "bsd_clause_2",
			Name:            "Simplified BSD License",
			PossibleMatches: []string{"BSD-2-Clause", "freebsd", "simpbsd", "simple bsd", "2-clause bsd", "2 clause bsd", "simplified bsd license"},
		},
		"gpl-2": {
			Code:            "gpl_2",
			Name:            "GNU General Public License 2.0",
			PossibleMatches: []string{"GPL-2.0", "gpl2", "gnu gpl2", "gplv2"},
		},
		"gpl-3": {
			Code:            "gpl_3",
			Name:            "GNU General Public License 3.0",
			PossibleMatches: []string{"GPL-3.0", "gpl3", "gplv3", "gpl", "gnu gpl3", "gnu gpl"},
		},
		"lgpl": {
			Code:            "lgpl",
			Name:            "GNU Lesser General Public License",
			PossibleMatches: []string{"LGPL-3.0", "lgpl", "lesser gpl", "gnu lgpl"},
		},
		"agpl": {
			Code:            "agpl",
			Name:            "GNU Affero General Public License",
			PossibleMatches: []string{"AGPL-3.0", "agpl", "affero gpl", "gnu agpl"},
		},
	}

	def, ok := licenseDefinitions[viper.GetString("license")]
	if !ok {
		def = &License{
			Code:            "None",
			Name:            "None",
			PossibleMatches: []string{"None", "false"},
		}
	}

	def.Copyright = fmt.Sprintf("Copyright © %s %s", year, viper.GetString("author"))

	if def.Code != "None" {
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
	data, err := templates.ReadFile(fmt.Sprintf("tpl/header_%s.tmpl", l.Code))
	if err != nil {
		return err
	}
	l.Header = string(data)
	return nil
}

func (l *License) getLicenseBody(templates embed.FS) error {
	data, err := templates.ReadFile(fmt.Sprintf("tpl/license_%s.tmpl", l.Code))
	if err != nil {
		return err
	}
	l.Body = string(data)
	return nil
}
