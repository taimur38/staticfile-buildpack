package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"io/ioutil"

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

	err = Compile(buildDir, cacheDir, manifest)
	if err != nil {
		panic(err)
	}
}

func Compile(buildDir, cacheDir string, manifest bp.Manifest) error {
	version, err := manifest.Version()
	if err != nil {
		bp.Log.Error("Could not determine buildpack version: %s", err.Error())
		return err
	}

	bp.Log.BeginStep("Staticfile Buildpack version %s", version)

	err = manifest.CheckStackSupport()
	if err != nil {
		bp.Log.Error("Stack not supported by buildpack: %s", err.Error())
		return err
	}

	manifest.CheckBuildpackVersion(cacheDir)

	var sf Staticfile
	bp.LoadYAML(filepath.Join(buildDir, "Staticfile"), &sf)

	appRootDir, err := getAppRootDir(buildDir, sf)
	if err != nil {
		bp.Log.Error("Invalid root directory: %s", err.Error())
		return err
	}

	err = copyFilesToPublic(buildDir, appRootDir, sf)
	if err != nil {
		bp.Log.Error("Failed copying project files: %s", err.Error())
		return err
	}

	err = setupNginx(buildDir, manifest)
	if err != nil {
		bp.Log.Error("Unable to install nginx: %s", err.Error())
		return err
	}

	err = applyStaticfileConfig(buildDir, sf)
	if err != nil {
		bp.Log.Error("Couldn't use config from Staticfile: %s", err.Error())
		return err
	}

	err = bp.CopyFile(filepath.Join(manifest.RootDir(), "bin", "boot.sh"), filepath.Join(buildDir, "boot.sh"))
	if err != nil {
		bp.Log.Error("Couldn't copy boot.sh: %s", err.Error())
		return err
	}

	manifest.StoreBuildpackMetadata(cacheDir)

	return nil
}

func getAppRootDir(buildDir string, sf Staticfile) (string, error) {
	var rootDirRelative string

	if sf.RootDir != "" {
		rootDirRelative = sf.RootDir
	} else {
		rootDirRelative = "."
	}

	rootDirAbs, err := filepath.Abs(filepath.Join(buildDir, rootDirRelative))
	if err != nil {
		return "", err
	}

	bp.Log.BeginStep("Root folder %s", rootDirAbs)

	dirInfo, err := os.Stat(rootDirAbs)
	if err != nil {
		return "", fmt.Errorf("the application Staticfile specifies a root directory %s that does not exist", rootDirRelative)
	}

	if !dirInfo.IsDir() {
		return "", fmt.Errorf("the application Staticfile specifies a root directory %s that is a plain file, but was expected to be a directory", rootDirRelative)
	}

	return rootDirAbs, nil
}

func copyFilesToPublic(buildDir string, appRootDir string, sf Staticfile) error {
	bp.Log.BeginStep("Copying project files into public")

	publicDir := filepath.Join(buildDir, "public")

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

func setupNginx(buildDir string, manifest bp.Manifest) error {
	bp.Log.BeginStep("Setting up nginx")

	nginx, err := manifest.DefaultVersion("nginx")
	if err != nil {
		return err
	}
	bp.Log.Info("Using Nginx version %s", nginx.Version)

	err = manifest.FetchDependency(nginx, "/tmp/nginx.tgz")
	if err != nil {
		return err
	}

	err = bp.ExtractTarGz("/tmp/nginx.tgz", buildDir)
	if err != nil {
		return err
	}

	confFiles := []string{"nginx.conf", "mime.types"}

	for _, file := range confFiles {
		var source string
		confDest := filepath.Join(buildDir, "nginx", "conf", file)
		customConfFile := filepath.Join(buildDir, "public", file)

		_, err = os.Stat(customConfFile)
		if err == nil {
			source = customConfFile
		} else {
			source = filepath.Join(manifest.RootDir(), "conf", file)
		}

		err = bp.CopyFile(source, confDest)
		if err != nil {
			return err
		}
	}

	authFile := filepath.Join(buildDir, "Staticfile.auth")
	_, err = os.Stat(authFile)
	if err == nil {
		bp.Log.BeginStep("Enabling basic authentication using Staticfile.auth")
		e := bp.CopyFile(authFile, filepath.Join(buildDir, "nginx", "conf", ".htpasswd"))
		if e != nil {
			return e
		}
		bp.Log.Protip("Learn about basic authentication", "http://docs.cloudfoundry.org/buildpacks/staticfile/index.html#authentication")
	}

	return nil
}

func applyStaticfileConfig(buildDir string, sf Staticfile) error {
	var err error
	nginxConfDir := filepath.Join(buildDir, "nginx", "conf")

	if sf.HostDotFiles {
		bp.Log.BeginStep("Enabling hosting of dotfiles")
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
		bp.Log.BeginStep("Enabling directory index for folders without index.html files")
		err = ioutil.WriteFile(filepath.Join(nginxConfDir, ".enable_directory_index"), []byte("x"), 0755)
		if err != nil {
			return err
		}
	}

	if sf.SSI == "enabled" {
		bp.Log.BeginStep("Enabling SSI")
		err = ioutil.WriteFile(filepath.Join(nginxConfDir, ".enable_ssi"), []byte("x"), 0755)
		if err != nil {
			return err
		}
	}

	if sf.PushState == "enabled" {
		bp.Log.BeginStep("Enabling pushstate")
		err = ioutil.WriteFile(filepath.Join(nginxConfDir, ".enable_pushstate"), []byte("x"), 0755)
		if err != nil {
			return err
		}
	}

	if sf.HSTS {
		bp.Log.BeginStep("Enabling HSTS")
		err = ioutil.WriteFile(filepath.Join(nginxConfDir, ".enable_hsts"), []byte("x"), 0755)
		if err != nil {
			return err
		}
	}

	return nil
}
