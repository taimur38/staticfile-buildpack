package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	bp "github.com/cloudfoundry/libbuildpack"
)

type Staticfile struct {
	RootDir         string `yaml:"root"`
	HostDotFiles    bool   `yaml:"host_dot_files"`
	LocationInclude string `yaml:"location_include"`
	DirectoryIndex  string `yaml:"directory"`
	SSI             string `yaml:"ssi"`
	PushState       string `yaml:"pushstate"`
	HSTS            bool   `yaml:"http_strict_transport_security"`
}

type StaticfileCompiler struct {
	Compiler *bp.Compiler
	Config   Staticfile
}

var skipCopyFile = map[string]bool{
	"Staticfile":      true,
	"Staticfile.auth": true,
	"manifest.yml":    true,
	".profile":        true,
	"stackato.yml":    true,
}

func main() {
	buildDir := os.Args[1]
	cacheDir := os.Args[2]

	compiler, err := bp.NewCompiler(buildDir, cacheDir, bp.NewLogger())
	err = compiler.CheckBuildpackValid()
	if err != nil {
		panic(err)
	}

	sc := StaticfileCompiler{Compiler: compiler, Config: Staticfile{}}
	err = sc.Compile()
	if err != nil {
		panic(err)
	}

	compiler.StagingComplete()
}

func (sc *StaticfileCompiler) LoadStaticFile() error {
	err := bp.LoadYAML(filepath.Join(sc.Compiler.BuildDir, "Staticfile"), &sc.Config)
	if err != nil {
		return err
	}

	return nil
}

func (sc *StaticfileCompiler) Compile() error {
	var err error

	err = sc.LoadStaticFile()
	if err != nil {
		sc.Compiler.Log.Error("Unable to load Staticfile: %s", err.Error())
		return err

	}

	appRootDir, err := sc.GetAppRootDir()
	if err != nil {
		sc.Compiler.Log.Error("Invalid root directory: %s", err.Error())
		return err
	}

	err = sc.copyFilesToPublic(appRootDir)
	if err != nil {
		sc.Compiler.Log.Error("Failed copying project files: %s", err.Error())
		return err
	}

	err = sc.setupNginx()
	if err != nil {
		sc.Compiler.Log.Error("Unable to install nginx: %s", err.Error())
		return err
	}

	err = sc.applyStaticfileConfig()
	if err != nil {
		sc.Compiler.Log.Error("Could not use config from Staticfile: %s", err.Error())
		return err
	}

	err = sc.WriteProfileD()
	if err != nil {
		sc.Compiler.Log.Error("Could not write .profile.d script: %s", err.Error())
		return err
	}

	return nil
}

func (sc *StaticfileCompiler) GetAppRootDir() (string, error) {
	var rootDirRelative string

	if sc.Config.RootDir != "" {
		rootDirRelative = sc.Config.RootDir
	} else {
		rootDirRelative = "."
	}

	rootDirAbs, err := filepath.Abs(filepath.Join(sc.Compiler.BuildDir, rootDirRelative))
	if err != nil {
		return "", err
	}

	sc.Compiler.Log.BeginStep("Root folder %s", rootDirAbs)

	dirInfo, err := os.Stat(rootDirAbs)
	if err != nil {
		return "", fmt.Errorf("the application Staticfile specifies a root directory %s that does not exist", rootDirRelative)
	}

	if !dirInfo.IsDir() {
		return "", fmt.Errorf("the application Staticfile specifies a root directory %s that is a plain file, but was expected to be a directory", rootDirRelative)
	}

	return rootDirAbs, nil
}

func (sc *StaticfileCompiler) copyFilesToPublic(appRootDir string) error {
	sc.Compiler.Log.BeginStep("Copying project files into public")

	publicDir := filepath.Join(sc.Compiler.BuildDir, "public")

	if publicDir == appRootDir {
		return nil
	}

	tmpDir, err := ioutil.TempDir("", "XXXXX")
	if err != nil {
		return err
	}

	files, err := ioutil.ReadDir(appRootDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if skipCopyFile[file.Name()] {
			continue
		}

		if strings.HasPrefix(file.Name(), ".") && !sc.Config.HostDotFiles {
			continue
		}

		err = os.Rename(filepath.Join(appRootDir, file.Name()), filepath.Join(tmpDir, file.Name()))
		if err != nil {
			return err
		}
	}

	err = os.Rename(tmpDir, publicDir)
	if err != nil {
		return err
	}

	return nil
}

func (sc *StaticfileCompiler) setupNginx() error {
	sc.Compiler.Log.BeginStep("Setting up nginx")

	nginx, err := sc.Compiler.Manifest.DefaultVersion("nginx")
	if err != nil {
		return err
	}
	sc.Compiler.Log.Info("Using Nginx version %s", nginx.Version)

	err = sc.Compiler.Manifest.FetchDependency(nginx, "/tmp/nginx.tgz")
	if err != nil {
		return err
	}

	err = bp.ExtractTarGz("/tmp/nginx.tgz", sc.Compiler.BuildDir)
	if err != nil {
		return err
	}

	confFiles := []string{"nginx.conf", "mime.types"}

	for _, file := range confFiles {
		var source string
		confDest := filepath.Join(sc.Compiler.BuildDir, "nginx", "conf", file)
		customConfFile := filepath.Join(sc.Compiler.BuildDir, "public", file)

		_, err = os.Stat(customConfFile)
		if err == nil {
			source = customConfFile
		} else {
			source = filepath.Join(sc.Compiler.Manifest.RootDir(), "conf", file)
		}

		err = bp.CopyFile(source, confDest)
		if err != nil {
			return err
		}
	}

	authFile := filepath.Join(sc.Compiler.BuildDir, "Staticfile.auth")
	_, err = os.Stat(authFile)
	if err == nil {
		sc.Compiler.Log.BeginStep("Enabling basic authentication using Staticfile.auth")
		e := bp.CopyFile(authFile, filepath.Join(sc.Compiler.BuildDir, "nginx", "conf", ".htpasswd"))
		if e != nil {
			return e
		}
		sc.Compiler.Log.Protip("Learn about basic authentication", "http://docs.cloudfoundry.org/buildpacks/staticfile/index.html#authentication")
	}

	return nil
}

func (sc *StaticfileCompiler) applyStaticfileConfig() error {
	var err error
	nginxConfDir := filepath.Join(sc.Compiler.BuildDir, "nginx", "conf")

	if sc.Config.HostDotFiles {
		sc.Compiler.Log.BeginStep("Enabling hosting of dotfiles")
		err = ioutil.WriteFile(filepath.Join(nginxConfDir, ".enable_dotfiles"), []byte("x"), 0666)
		if err != nil {
			return err
		}
	}

	if sc.Config.LocationInclude != "" {
		err = ioutil.WriteFile(filepath.Join(nginxConfDir, ".enable_location_include"), []byte(sc.Config.LocationInclude), 0666)
		if err != nil {
			return err
		}
	}

	if sc.Config.DirectoryIndex != "" {
		sc.Compiler.Log.BeginStep("Enabling directory index for folders without index.html files")
		err = ioutil.WriteFile(filepath.Join(nginxConfDir, ".enable_directory_index"), []byte("x"), 0666)
		if err != nil {
			return err
		}
	}

	if sc.Config.SSI == "enabled" {
		sc.Compiler.Log.BeginStep("Enabling SSI")
		err = ioutil.WriteFile(filepath.Join(nginxConfDir, ".enable_ssi"), []byte("x"), 0666)
		if err != nil {
			return err
		}
	}

	if sc.Config.PushState == "enabled" {
		sc.Compiler.Log.BeginStep("Enabling pushstate")
		err = ioutil.WriteFile(filepath.Join(nginxConfDir, ".enable_pushstate"), []byte("x"), 0666)
		if err != nil {
			return err
		}
	}

	if sc.Config.HSTS {
		sc.Compiler.Log.BeginStep("Enabling HSTS")
		err = ioutil.WriteFile(filepath.Join(nginxConfDir, ".enable_hsts"), []byte("x"), 0666)
		if err != nil {
			return err
		}
	}

	return nil
}

func (sc *StaticfileCompiler) WriteProfileD() error {
	err := os.MkdirAll(filepath.Join(sc.Compiler.BuildDir, ".profile.d"), 0755)
	if err != nil {
		return err
	}

	script := filepath.Join(sc.Compiler.BuildDir, ".profile.d", "staticfile.sh")

	err = ioutil.WriteFile(script, []byte(InitScript), 0755)
	if err != nil {
		return err
	}

	return nil
}
