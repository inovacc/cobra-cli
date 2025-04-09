package project

import (
	"bufio"
	"bytes"
	"crypto/md5"
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
	cobra.CheckErr(InitSrcPaths())
}

func detectGoPaths() ([]string, error) {
	goPath := os.Getenv("GOPATH")
	if goPath != "" {
		return filepath.SplitList(goPath), nil
	}

	goExecutable := os.Getenv("COBRA_GO_EXECUTABLE")
	if goExecutable == "" {
		goExecutable = "go"
	}

	out, err := exec.Command(goExecutable, "env", "GOPATH").Output()
	if err != nil {
		return nil, fmt.Errorf("could not detect GOPATH: %w", err)
	}

	paths := filepath.SplitList(strings.TrimSpace(string(out)))
	if len(paths) == 0 {
		return nil, errors.New("$GOPATH is not set or could not be determined")
	}

	return paths, nil
}

func buildSrcPaths(goPaths []string) []string {
	srcs := make([]string, 0, len(goPaths))
	for _, gp := range goPaths {
		srcs = append(srcs, filepath.Join(gp, "src"))
	}
	return srcs
}

func InitSrcPaths() error {
	goPaths, err := detectGoPaths()
	if err != nil {
		return err
	}
	srcPaths = buildSrcPaths(goPaths)
	return nil
}

func runDiff(pathA, pathB string) (string, error) {
	diffPath, err := exec.LookPath("diff")
	if err != nil {
		return "", nil // diff no disponible
	}

	var output bytes.Buffer
	cmd := exec.Command(diffPath, "-u", "--strip-trailing-cr", pathA, pathB)
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Run(); err != nil {
		return output.String(), nil // ignora error de diff, lo importante es la salida
	}
	return output.String(), nil
}

func compareContent(contentA, contentB []byte) error {
	if !bytes.Equal(ensureLF(contentA), ensureLF(contentB)) {
		return errors.New("byte slices differ")
	}
	return nil
}

func validateCmdName(args []string) string {
	var source string
	if len(args) > 0 {
		source = args[0]
	}

	var sb strings.Builder
	capitalize := false

	for i := 0; i < len(source); i++ {
		ch := source[i]
		if ch == '-' || ch == '_' {
			capitalize = true
			continue
		}
		if capitalize {
			sb.WriteByte(byte(unicode.ToUpper(rune(ch))))
			capitalize = false
		} else {
			sb.WriteByte(ch)
		}
	}

	if sb.Len() == 0 {
		return source
	}
	return sb.String()
}

// ensureLF converts any \r\n to \n
func ensureLF(content []byte) []byte {
	return bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
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
	Legal        *License //`json:"-" yaml:"-"`
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
	Afs       afero.Fs            `json:"-" yaml:"-"`
	Templates embed.FS            `json:"-" yaml:"-"`
	Licenses  map[string]*License `json:"licenses" yaml:"licenses"`
	None      bool
	Project   *Project
	Content   []Content
}

func NewProjectGenerator(fs afero.Fs, project *Project) (*Generator, error) {
	project.CmdName = validateCmdName(project.Args)

	return &Generator{
		Afs:       fs,
		Templates: templates,
		Licenses:  contentLicenses(templates),
		Project:   project,
		Content:   []Content{},
	}, nil
}

func (g *Generator) SetLicense() error {
	license, ok := g.Licenses[viper.GetString("license")]
	if !ok {
		return errors.New("license file not found in templates")
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

	// Ensure base directory exists
	if !stat(g.Afs, g.Project.AbsolutePath) {
		if err := g.Afs.MkdirAll(g.Project.AbsolutePath, 0754); err != nil {
			return err
		}
	}

	rootPath := filepath.Join(g.Project.AbsolutePath, "cmd")
	if !stat(g.Afs, rootPath) {
		if err := g.Afs.MkdirAll(rootPath, 0751); err != nil {
			return err
		}
	}

	if err := g.getFileContentLicense(); err != nil {
		return err
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
	return nil
}

func (g *Generator) getFileContentLicense() error {
	content := Content{
		Name:             "license",
		FilePath:         fmt.Sprintf("%s/LICENSE", g.Project.AbsolutePath),
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
		FilePath:         fmt.Sprintf("%s/cmd/root.go", g.Project.AbsolutePath),
		Dirty:            true,
	}

	if g.None {
		content.TemplateFilePath = "tpl/root_none.tmpl"
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

func hashLicenseContent(templates embed.FS, code string) string {
	data, err := templates.ReadFile(fmt.Sprintf("tpl/license_%s.tmpl", code))
	if err != nil {
		return "invalid hash"
	}
	return fmt.Sprintf("%X", md5.Sum(data))
}

func getLicenseHeader(templates embed.FS, code string) string {
	data, err := templates.ReadFile(fmt.Sprintf("tpl/header_%s.tmpl", code))
	if err != nil {
		return "No header license content"
	}
	return string(data)
}

func getLicenseBody(templates embed.FS, code string) string {
	data, err := templates.ReadFile(fmt.Sprintf("tpl/license_%s.tmpl", code))
	if err != nil {
		return "No license content"
	}
	return string(data)
}

type License struct {
	Code            string   // The Code name of the license
	Name            string   // The type of license in use
	PossibleMatches []string // Similar names to guess
	Header          string   // License header for source files
	Body            string   // License body
	Copyright       string   // Copyright line
	HashLicense     string   // HashLicense for quick search
}

func contentLicenses(templates embed.FS) map[string]*License {
	year := viper.GetString("year")
	if year == "" {
		year = time.Now().Format("2006")
	}

	return map[string]*License{
		"apache2": {
			Code:            "apache_2",
			Name:            "Apache 2.0",
			PossibleMatches: []string{"Apache-2.0", "apache", "apache20", "apache 2.0", "apache2.0", "apache-2.0"},
			Header:          getLicenseHeader(templates, "apache_2"),
			Body:            getLicenseBody(templates, "apache_2"),
			Copyright:       fmt.Sprintf("Copyright © %s %s", year, viper.GetString("author")),
			HashLicense:     hashLicenseContent(templates, "apache_2"),
		},
		"mit": {
			Code:            "mit",
			Name:            "MIT License",
			PossibleMatches: []string{"MIT", "mit"},
			Header:          getLicenseHeader(templates, "mit"),
			Body:            getLicenseBody(templates, "mit"),
			Copyright:       fmt.Sprintf("Copyright © %s %s", year, viper.GetString("author")),
			HashLicense:     hashLicenseContent(templates, "mit"),
		},
		"bsd3": {
			Code:            "bsd_clause_3",
			Name:            "NewBSD",
			PossibleMatches: []string{"BSD-3-Clause", "bsd", "newbsd", "3 clause bsd", "3-clause bsd"},
			Header:          getLicenseHeader(templates, "bsd_clause_3"),
			Body:            getLicenseBody(templates, "bsd_clause_3"),
			Copyright:       fmt.Sprintf("Copyright © %s %s", year, viper.GetString("author")),
			HashLicense:     hashLicenseContent(templates, "bsd_clause_3"),
		},
		"bsd2": {
			Code:            "bsd_clause_2",
			Name:            "Simplified BSD License",
			PossibleMatches: []string{"BSD-2-Clause", "freebsd", "simpbsd", "simple bsd", "2-clause bsd", "2 clause bsd", "simplified bsd license"},
			Header:          getLicenseHeader(templates, "bsd_clause_2"),
			Body:            getLicenseBody(templates, "bsd_clause_2"),
			Copyright:       fmt.Sprintf("Copyright © %s %s", year, viper.GetString("author")),
			HashLicense:     hashLicenseContent(templates, "bsd_clause_2"),
		},
		"gpl2": {
			Code:            "gpl_2",
			Name:            "GNU General Public License 2.0",
			PossibleMatches: []string{"GPL-2.0", "gpl2", "gnu gpl2", "gplv2"},
			Header:          getLicenseHeader(templates, "gpl_2"),
			Body:            getLicenseBody(templates, "gpl_2"),
			Copyright:       fmt.Sprintf("Copyright © %s %s", year, viper.GetString("author")),
			HashLicense:     hashLicenseContent(templates, "gpl_2"),
		},
		"gpl3": {
			Code:            "gpl_3",
			Name:            "GNU General Public License 3.0",
			PossibleMatches: []string{"GPL-3.0", "gpl3", "gplv3", "gpl", "gnu gpl3", "gnu gpl"},
			Header:          getLicenseHeader(templates, "gpl_3"),
			Body:            getLicenseBody(templates, "gpl_3"),
			Copyright:       fmt.Sprintf("Copyright © %s %s", year, viper.GetString("author")),
			HashLicense:     hashLicenseContent(templates, "gpl_3"),
		},
		"lgpl": {
			Code:            "lgpl",
			Name:            "GNU Lesser General Public License",
			PossibleMatches: []string{"LGPL-3.0", "lgpl", "lesser gpl", "gnu lgpl"},
			Header:          getLicenseHeader(templates, "lgpl"),
			Body:            getLicenseBody(templates, "lgpl"),
			Copyright:       fmt.Sprintf("Copyright © %s %s", year, viper.GetString("author")),
			HashLicense:     hashLicenseContent(templates, "lgpl"),
		},
		"agpl": {
			Code:            "agpl",
			Name:            "GNU Affero General Public License",
			PossibleMatches: []string{"AGPL-3.0", "agpl", "affero gpl", "gnu agpl"},
			Header:          getLicenseHeader(templates, "agpl"),
			Body:            getLicenseBody(templates, "agpl"),
			Copyright:       fmt.Sprintf("Copyright © %s %s", year, viper.GetString("author")),
			HashLicense:     hashLicenseContent(templates, "agpl"),
		},
		"none": {
			Code:            "none",
			Name:            "None License",
			PossibleMatches: []string{"none", "false"},
			Copyright:       fmt.Sprintf("Copyright © %s %s", year, viper.GetString("author")),
		},
	}
}
