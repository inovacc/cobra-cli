package project

import (
	"bytes"
	"crypto/md5"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

func gitInit() error {
	cmd := exec.Command("git", "init")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func goModInitWithName(modName string) error {
	if _, err := os.Stat("go.mod"); err == nil {
		fmt.Println("go.mod already exists, skipping `go mod init`.")
		return nil
	}
	cmd := exec.Command("go", "mod", "init", modName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func goModInit() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	modName := path.Base(wd)
	cmd := exec.Command("go", "mod", "init", modName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type Command struct {
	CmdName          string
	CmdParent        string
	ExtractedLicense string
	*Project
}

type Project struct {
	Args         []string
	PkgName      string
	AbsolutePath string
	AppName      string
	CmdName      string
	Legal        *License
	NewProject   bool
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
		Legal:        &License{},
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

	license, ok := contentLicenses(templates)[viper.GetString("license")]
	if ok {
		project.Legal = license
	}

	return &Generator{
		None:      project.Legal.Code == "none",
		Afs:       fs,
		Templates: templates,
		Project:   project,
		Content:   []Content{},
	}, nil
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

func (g *Generator) PrepareModels() error {
	if err := g.getFileContentLicense(); err != nil {
		return err
	}

	if err := g.getFileContentMain(); err != nil {
		return err
	}

	if err := g.getFileContentRoot(); err != nil {
		return err
	}

	if err := g.getFileContentConfig(); err != nil {
		return err
	}

	if err := g.getFileContentService(); err != nil {
		return err
	}

	if g.Project.NewProject {
		if err := g.getFileContentIgnore(); err != nil {
			return err
		}

		if err := g.getFileContentReadme(); err != nil {
			return err
		}
	}

	return nil
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

	configPath := filepath.Join(g.Project.AbsolutePath, "internal", "config")
	if !stat(g.Afs, configPath) {
		if err := g.Afs.MkdirAll(configPath, 0751); err != nil {
			return err
		}
	}

	servicePath := filepath.Join(g.Project.AbsolutePath, "internal", "service")
	if !stat(g.Afs, servicePath) {
		if err := g.Afs.MkdirAll(servicePath, 0751); err != nil {
			return err
		}
	}

	if g.Project.NewProject {
		if err := g.goModInit(); err != nil {
		}

		if err := g.gitInit(); err != nil {
		}
	}

	if err := g.renderTemplate(); err != nil {
		return err
	}

	return nil
}

// AddCommandProject sets up the Project structure and files for a new command.
func (g *Generator) AddCommandProject() error {
	// find LICENSE and root.go file in project
	_, rootGo, err := findLicenseAndRootGo(g.Afs, g.Project.AbsolutePath)
	if err != nil {
		return err
	}

	if !stat(g.Afs, rootGo) {
		return fmt.Errorf("no root file found on: %s", rootGo)
	}

	g.Project.AbsolutePath = filepath.Dir(rootGo)

	// Ensure base directory exists
	if !stat(g.Afs, g.Project.AbsolutePath) {
		if err := g.Afs.MkdirAll(g.Project.AbsolutePath, 0754); err != nil {
			return err
		}
	}

	if err := g.getFileContentSub(rootGo); err != nil {
		return err
	}

	if err := g.renderTemplate(); err != nil {
		return err
	}

	return nil
}

func (g *Generator) goModInit() error {
	if g.Project.PkgName == "" {
		return goModInit()
	}
	return goModInitWithName(g.Project.PkgName)
}

func (g *Generator) gitInit() error {
	return gitInit()
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

func (g *Generator) getFileContentConfig() error {
	content1 := Content{
		Name:             "config",
		TemplateFilePath: "tpl/config.tmpl",
		FilePath:         fmt.Sprintf("%s/internal/config/config.go", g.Project.AbsolutePath),
		Dirty:            true,
	}

	content2 := Content{
		Name:             "config_test",
		TemplateFilePath: "tpl/config_test.tmpl",
		FilePath:         fmt.Sprintf("%s/internal/config/config_test.go", g.Project.AbsolutePath),
		Dirty:            true,
	}

	content3 := Content{
		Name:             "config_custom",
		TemplateFilePath: "tpl/custom.tmpl",
		FilePath:         fmt.Sprintf("%s/internal/config/custom.go", g.Project.AbsolutePath),
		Dirty:            true,
	}

	defer func() {
		g.Content = append(g.Content, content1)
		g.Content = append(g.Content, content2)
		g.Content = append(g.Content, content3)
	}()

	data1, err := g.Templates.ReadFile(content1.TemplateFilePath)
	if err != nil {
		return err
	}

	data2, err := g.Templates.ReadFile(content2.TemplateFilePath)
	if err != nil {
		return err
	}

	data3, err := g.Templates.ReadFile(content3.TemplateFilePath)
	if err != nil {
		return err
	}

	content1.TemplateContent = string(data1)
	content1.Data = g.Project
	content1.Dirty = false

	content2.TemplateContent = string(data2)
	content2.Data = g.Project
	content2.Dirty = false

	content3.TemplateContent = string(data3)
	content3.Data = g.Project
	content3.Dirty = false
	return nil
}

func (g *Generator) getFileContentService() error {
	content := Content{
		Name:             "service",
		TemplateFilePath: "tpl/service.tmpl",
		FilePath:         fmt.Sprintf("%s/internal/service/service.go", g.Project.AbsolutePath),
		Dirty:            true,
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

func (g *Generator) getFileContentIgnore() error {
	content := Content{
		Name:             "gitignore",
		TemplateFilePath: "tpl/gitignore.tmpl",
		FilePath:         fmt.Sprintf("%s/.gitignore", g.Project.AbsolutePath),
		Dirty:            true,
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

func (g *Generator) getFileContentReadme() error {
	content := Content{
		Name:             "readme",
		TemplateFilePath: "tpl/readme.tmpl",
		FilePath:         fmt.Sprintf("%s/README.md", g.Project.AbsolutePath),
		Dirty:            true,
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

func (g *Generator) getFileContentSub(rootGo string) error {
	content := Content{
		Name:             "add_command",
		FilePath:         fmt.Sprintf("%s/%s.go", g.Project.AbsolutePath, g.Project.AppName),
		TemplateFilePath: "tpl/add_command.tmpl",
		Dirty:            true,
	}

	comment, err := extractBlockCommentBeforePackage(g.Afs, rootGo)
	if err != nil {
		return err
	}

	if comment == "" {
		g.None = true
		content.TemplateFilePath = "tpl/add_command_none.tmpl"
	}

	defer func() {
		g.Content = append(g.Content, content)
	}()

	data, err := g.Templates.ReadFile(content.TemplateFilePath)
	if err != nil {
		return err
	}

	content.TemplateContent = string(data)
	content.Data = Command{
		CmdParent:        "rootCmd",
		CmdName:          g.Project.CmdName,
		Project:          g.Project,
		ExtractedLicense: comment,
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

func findLicenseAndRootGo(fs afero.Fs, root string) (string, string, error) {
	var licensePath, rootGoPath string

	root = filepath.Join(root, "..")

	err := afero.Walk(fs, root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		switch info.Name() {
		case "LICENSE":
			licensePath = path
		case "root.go":
			rootGoPath = path
		}
		return nil
	})

	if err != nil {
		return "", "", err
	}

	if licensePath == "" || rootGoPath == "" {
		return "", "", fmt.Errorf("missing file(s): LICENSE=%v, root.go=%v", licensePath != "", rootGoPath != "")
	}

	return licensePath, rootGoPath, nil
}

func extractBlockCommentBeforePackage(fs afero.Fs, filePath string) (string, error) {
	content, err := afero.ReadFile(fs, filePath)
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`(?s)/\*.*?\*/\s*package\s+cmd`)
	match := re.Find(content)
	if match == nil {
		return "", nil
	}

	block := regexp.MustCompile(`(?s)/\*.*?\*/`).Find(match)
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(string(block), "/*", ""), "*/", "")), nil
}
