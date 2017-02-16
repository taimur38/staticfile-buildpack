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

var skipCopyFile = map[string]bool{
	"Staticfile":      true,
	"Staticfile.auth": true,
	"manifest.yml":    true,
	".profile":        true,
	"stackato.yml":    true,
}

type Compiler struct {
	BuildDir string
	CacheDir string
	Manifest bp.Manifest
	Log      bp.Logger
}

func main() {
	var err error

	buildDir := os.Args[1]
	cacheDir := os.Args[2]

	bpDir := os.Getenv("BUILDPACK_DIR")

	if bpDir == "" {
		bpDir, err = filepath.Abs(filepath.Join(filepath.Dir(os.Args[0]), ".."))

		if err != nil {
			panic(err)
		}
	}

	manifest, err := bp.NewManifest(bpDir)
	if err != nil {
		panic(err)
	}

	c := &Compiler{BuildDir: buildDir,
		CacheDir: cacheDir,
		Manifest: manifest,
		Log:      bp.NewLogger()}

	err = c.Compile()
	if err != nil {
		panic(err)
	}
}

func (c *Compiler) Compile() error {
	version, err := c.Manifest.Version()
	if err != nil {
		c.Log.Error("Could not determine buildpack version: %s", err.Error())
		return err
	}

	c.Log.BeginStep("Staticfile Buildpack version %s", version)

	err = c.Manifest.CheckStackSupport()
	if err != nil {
		c.Log.Error("Stack not supported by buildpack: %s", err.Error())
		return err
	}

	c.Manifest.CheckBuildpackVersion(c.CacheDir)

	var sf Staticfile
	bp.LoadYAML(filepath.Join(c.BuildDir, "Staticfile"), &sf)

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
		c.Log.Error("Couldn't use config from Staticfile: %s", err.Error())
		return err
	}

	err = bp.CopyFile(filepath.Join(c.Manifest.RootDir(), "bin", "boot.sh"), filepath.Join(c.BuildDir, "boot.sh"))
	if err != nil {
		c.Log.Error("Couldn't copy boot.sh: %s", err.Error())
		return err
	}

	c.Manifest.StoreBuildpackMetadata(c.CacheDir)

	return nil
}

func (c *Compiler) GetAppRootDir(sf Staticfile) (string, error) {
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

func (c *Compiler) copyFilesToPublic(appRootDir string, sf Staticfile) error {
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

func (c *Compiler) setupNginx() error {
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

func (c *Compiler) applyStaticfileConfig(sf Staticfile) error {
	var err error
	nginxConfDir := filepath.Join(c.BuildDir, "nginx", "conf")

	if sf.HostDotFiles {
		c.Log.BeginStep("Enabling hosting of dotfiles")
		err = ioutil.WriteFile(filepath.Join(nginxConfDir, ".enable_dotfiles"), []byte("x"), 0755)
		if err != nil {
			return err
		}
	}

	if sf.LocationInclude != "" {
		err = ioutil.WriteFile(filepath.Join(nginxConfDir, ".enable_location_include"), []byte(sf.LocationInclude), 0755)
		if err != nil {
			return err
		}
	}

	if sf.DirectoryIndex != "" {
		c.Log.BeginStep("Enabling directory index for folders without index.html files")
		err = ioutil.WriteFile(filepath.Join(nginxConfDir, ".enable_directory_index"), []byte("x"), 0755)
		if err != nil {
			return err
		}
	}

	if sf.SSI == "enabled" {
		c.Log.BeginStep("Enabling SSI")
		err = ioutil.WriteFile(filepath.Join(nginxConfDir, ".enable_ssi"), []byte("x"), 0755)
		if err != nil {
			return err
		}
	}

	if sf.PushState == "enabled" {
		c.Log.BeginStep("Enabling pushstate")
		err = ioutil.WriteFile(filepath.Join(nginxConfDir, ".enable_pushstate"), []byte("x"), 0755)
		if err != nil {
			return err
		}
	}

	if sf.HSTS {
		c.Log.BeginStep("Enabling HSTS")
		err = ioutil.WriteFile(filepath.Join(nginxConfDir, ".enable_hsts"), []byte("x"), 0755)
		if err != nil {
			return err
		}
	}

	return nil
}
