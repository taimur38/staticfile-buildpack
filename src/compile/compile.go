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

type StaticfileCompiler bp.Compiler

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

	c, err := bp.NewCompiler(buildDir, cacheDir, bp.NewLogger())
	err = c.CheckBuildpackValid()
	if err != nil {
		panic(err)
	}

	err = StaticfileCompiler(c).Compile()
	if err != nil {
		panic(err)
	}

	c.StagingComplete()
}

func (c StaticfileCompiler) Compile() error {
	var err error
	var sf Staticfile

	err = bp.LoadYAML(filepath.Join(c.BuildDir, "Staticfile"), &sf)
	if err != nil {
		c.Log.Error("Unable to: %s", err.Error())
		return err
	}

	appRootDir, err := c.GetAppRootDir(sf)
	if err != nil {
		c.Log.Error("Invalid root directory: %s", err.Error())
		return err
	}

	err = c.copyFilesToPublic(appRootDir, sf)
	if err != nil {
		c.Log.Error("Failed copying project files: %s", err.Error())
		return err
	}

	err = c.setupNginx()
	if err != nil {
		c.Log.Error("Unable to install nginx: %s", err.Error())
		return err
	}

	err = c.applyStaticfileConfig(sf)
	if err != nil {
		c.Log.Error("Could not use config from Staticfile: %s", err.Error())
		return err
	}

	err = c.WriteProfileD()
	if err != nil {
		c.Log.Error("Could not write .profile.d script: %s", err.Error())
		return err
	}

	return nil
}

func (c *StaticfileCompiler) GetAppRootDir(sf Staticfile) (string, error) {
	var rootDirRelative string

	if sf.RootDir != "" {
		rootDirRelative = sf.RootDir
	} else {
		rootDirRelative = "."
	}

	rootDirAbs, err := filepath.Abs(filepath.Join(c.BuildDir, rootDirRelative))
	if err != nil {
		return "", err
	}

	c.Log.BeginStep("Root folder %s", rootDirAbs)

	dirInfo, err := os.Stat(rootDirAbs)
	if err != nil {
		return "", fmt.Errorf("the application Staticfile specifies a root directory %s that does not exist", rootDirRelative)
	}

	if !dirInfo.IsDir() {
		return "", fmt.Errorf("the application Staticfile specifies a root directory %s that is a plain file, but was expected to be a directory", rootDirRelative)
	}

	return rootDirAbs, nil
}

func (c *StaticfileCompiler) copyFilesToPublic(appRootDir string, sf Staticfile) error {
	c.Log.BeginStep("Copying project files into public")

	publicDir := filepath.Join(c.BuildDir, "public")

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

		if strings.HasPrefix(file.Name(), ".") && !sf.HostDotFiles {
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

func (c *StaticfileCompiler) setupNginx() error {
	c.Log.BeginStep("Setting up nginx")

	nginx, err := c.Manifest.DefaultVersion("nginx")
	if err != nil {
		return err
	}
	c.Log.Info("Using Nginx version %s", nginx.Version)

	err = c.Manifest.FetchDependency(nginx, "/tmp/nginx.tgz")
	if err != nil {
		return err
	}

	err = bp.ExtractTarGz("/tmp/nginx.tgz", c.BuildDir)
	if err != nil {
		return err
	}

	confFiles := []string{"nginx.conf", "mime.types"}

	for _, file := range confFiles {
		var source string
		confDest := filepath.Join(c.BuildDir, "nginx", "conf", file)
		customConfFile := filepath.Join(c.BuildDir, "public", file)

		_, err = os.Stat(customConfFile)
		if err == nil {
			source = customConfFile
		} else {
			source = filepath.Join(c.Manifest.RootDir(), "conf", file)
		}

		err = bp.CopyFile(source, confDest)
		if err != nil {
			return err
		}
	}

	authFile := filepath.Join(c.BuildDir, "Staticfile.auth")
	_, err = os.Stat(authFile)
	if err == nil {
		c.Log.BeginStep("Enabling basic authentication using Staticfile.auth")
		e := bp.CopyFile(authFile, filepath.Join(c.BuildDir, "nginx", "conf", ".htpasswd"))
		if e != nil {
			return e
		}
		c.Log.Protip("Learn about basic authentication", "http://docs.cloudfoundry.org/buildpacks/staticfile/index.html#authentication")
	}

	return nil
}

func (c *StaticfileCompiler) applyStaticfileConfig(sf Staticfile) error {
	var err error
	nginxConfDir := filepath.Join(c.BuildDir, "nginx", "conf")

	if sf.HostDotFiles {
		c.Log.BeginStep("Enabling hosting of dotfiles")
		err = ioutil.WriteFile(filepath.Join(nginxConfDir, ".enable_dotfiles"), []byte("x"), 0666)
		if err != nil {
			return err
		}
	}

	if sf.LocationInclude != "" {
		err = ioutil.WriteFile(filepath.Join(nginxConfDir, ".enable_location_include"), []byte(sf.LocationInclude), 0666)
		if err != nil {
			return err
		}
	}

	if sf.DirectoryIndex != "" {
		c.Log.BeginStep("Enabling directory index for folders without index.html files")
		err = ioutil.WriteFile(filepath.Join(nginxConfDir, ".enable_directory_index"), []byte("x"), 0666)
		if err != nil {
			return err
		}
	}

	if sf.SSI == "enabled" {
		c.Log.BeginStep("Enabling SSI")
		err = ioutil.WriteFile(filepath.Join(nginxConfDir, ".enable_ssi"), []byte("x"), 0666)
		if err != nil {
			return err
		}
	}

	if sf.PushState == "enabled" {
		c.Log.BeginStep("Enabling pushstate")
		err = ioutil.WriteFile(filepath.Join(nginxConfDir, ".enable_pushstate"), []byte("x"), 0666)
		if err != nil {
			return err
		}
	}

	if sf.HSTS {
		c.Log.BeginStep("Enabling HSTS")
		err = ioutil.WriteFile(filepath.Join(nginxConfDir, ".enable_hsts"), []byte("x"), 0666)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *StaticfileCompiler) WriteProfileD() error {
	err := os.MkdirAll(filepath.Join(c.BuildDir, ".profile.d"), 0755)
	if err != nil {
		return err
	}

	script := filepath.Join(c.BuildDir, ".profile.d", "staticfile.sh")

	err = ioutil.WriteFile(script, []byte(InitScript), 0755)
	if err != nil {
		return err
	}

	return nil
}
