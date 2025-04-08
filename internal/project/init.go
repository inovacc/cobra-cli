package project

//
//import (
//	"encoding/json"
//	"fmt"
//	"github.com/spf13/afero"
//	"github.com/spf13/cobra"
//	"os"
//	"os/exec"
//	"path"
//	"path/filepath"
//	"strings"
//)
//
//func InitializeProject(args []string) (string, error) {
//	wd, err := os.Getwd()
//	if err != nil {
//		return "", err
//	}
//
//	if len(args) > 0 {
//		if args[0] != "." {
//			wd = fmt.Sprintf("%s/%s", wd, args[0])
//		}
//	}
//
//	modName := getModImportPath()
//	newProject := NewProject(afero.NewOsFs(), wd, modName, path.Base(modName))
//
//	if err := newProject.Create(); err != nil {
//		return "", err
//	}
//
//	return newProject.AbsolutePath, nil
//}
//
//func getModImportPath() string {
//	mod, cd := parseModInfo()
//	return path.Join(mod.Path, fileToURL(strings.TrimPrefix(cd.Dir, mod.Dir)))
//}
//
//func fileToURL(in string) string {
//	i := strings.Split(in, string(filepath.Separator))
//	return path.Join(i...)
//}
//
//func parseModInfo() (Mod, CurDir) {
//	var (
//		mod Mod
//		dir CurDir
//	)
//
//	m := modInfoJSON("-m")
//	cobra.CheckErr(json.Unmarshal(m, &mod))
//
//	// Unsure why, but if no module is present Path is set to this string.
//	if mod.Path == "command-line-arguments" {
//		cobra.CheckErr("Please run `go mod init <MODNAME>` before `cobra-cli init`")
//	}
//
//	e := modInfoJSON("-e")
//	cobra.CheckErr(json.Unmarshal(e, &dir))
//
//	return mod, dir
//}
//
//type Mod struct {
//	Path, Dir, GoMod string
//}
//
//type CurDir struct {
//	Dir string
//}
//
//func GoGet(mod string) error {
//	return exec.Command("go", "get", mod).Run()
//}
//
//func modInfoJSON(args ...string) []byte {
//	cmdArgs := append([]string{"list", "-json"}, args...)
//	out, err := exec.Command("go", cmdArgs...).Output()
//	cobra.CheckErr(err)
//	return out
//}
