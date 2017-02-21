package main_test

import (
	c "compile"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"bytes"

	bp "github.com/cloudfoundry/libbuildpack"
	"github.com/golang/mock/gomock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

//go:generate mockgen -source=vendor/github.com/cloudfoundry/libbuildpack/yaml.go --destination=mocks_yaml_test.go --package=main_test

var _ = Describe("Compile", func() {
	var (
		sf       c.Staticfile
		err      error
		buildDir string
		cacheDir string
		manifest bp.Manifest
		compiler *c.StaticfileCompiler
		logger   bp.Logger
		mockCtrl *gomock.Controller
		mockYaml *MockYAML
		buffer   *bytes.Buffer
	)

	BeforeEach(func() {
		buildDir, err = ioutil.TempDir("", "build")
		Expect(err).To(BeNil())

		cacheDir, err = ioutil.TempDir("", "cache")
		Expect(err).To(BeNil())

		manifest, err = bp.NewManifest("fixtures/standard_manifest")
		Expect(err).To(BeNil())

		buffer = new(bytes.Buffer)

		logger = bp.NewLogger()
		logger.SetOutput(buffer)

		mockCtrl = gomock.NewController(GinkgoT())
		mockYaml = NewMockYAML(mockCtrl)
	})

	JustBeforeEach(func() {
		bpc := &bp.Compiler{BuildDir: buildDir,
			CacheDir: cacheDir,
			Manifest: manifest,
			Log:      logger}

		compiler = &c.StaticfileCompiler{Compiler: bpc,
			Config: sf,
			YAML:   mockYaml}
	})

	Describe("LoadStaticfile", func() {
		Context("the staticfile does not exist", func() {
			BeforeEach(func() {
				mockYaml.EXPECT().Load(filepath.Join(buildDir, "Staticfile"), gomock.Any()).Return(os.ErrNotExist)
			})
			It("does not return an error", func() {
				err = compiler.LoadStaticfile()
				Expect(err).To(BeNil())
			})

			It("has default values", func() {
				err = compiler.LoadStaticfile()
				Expect(err).To(BeNil())
				Expect(compiler.Config.RootDir).To(Equal(""))
				Expect(compiler.Config.HostDotFiles).To(Equal(false))
				Expect(compiler.Config.LocationInclude).To(Equal(""))
				Expect(compiler.Config.DirectoryIndex).To(Equal(false))
				Expect(compiler.Config.SSI).To(Equal(false))
				Expect(compiler.Config.PushState).To(Equal(false))
				Expect(compiler.Config.HSTS).To(Equal(false))
				Expect(compiler.Config.ForceHTTPS).To(Equal(false))
				Expect(compiler.Config.BasicAuth).To(Equal(false))
			})

			It("does not log enabling statements", func() {
				Expect(buffer.String()).To(Equal(""))
			})
		})
		Context("the staticfile exists", func() {
			JustBeforeEach(func() {
				err = compiler.LoadStaticfile()
				Expect(err).To(BeNil())
			})

			Context("and sets root", func() {
				BeforeEach(func() {
					mockYaml.EXPECT().Load(filepath.Join(buildDir, "Staticfile"), gomock.Any()).Do(func(_ string, hash *map[string]string) {
						(*hash)["root"] = "root_test"
					})
				})
				It("sets RootDir", func() {
					Expect(compiler.Config.RootDir).To(Equal("root_test"))
				})
			})

			Context("and sets host_dot_files", func() {
				BeforeEach(func() {
					mockYaml.EXPECT().Load(filepath.Join(buildDir, "Staticfile"), gomock.Any()).Do(func(_ string, hash *map[string]string) {
						(*hash)["host_dot_files"] = "true"
					})
				})
				It("sets HostDotFiles", func() {
					Expect(compiler.Config.HostDotFiles).To(Equal(true))
				})
				It("Logs", func() {
					Expect(buffer.String()).To(Equal("-----> Enabling hosting of dotfiles\n"))
				})
			})

			Context("and sets location_include", func() {
				BeforeEach(func() {
					mockYaml.EXPECT().Load(filepath.Join(buildDir, "Staticfile"), gomock.Any()).Do(func(_ string, hash *map[string]string) {
						(*hash)["location_include"] = "a/b/c"
					})
				})
				It("sets location_include", func() {
					Expect(compiler.Config.LocationInclude).To(Equal("a/b/c"))
				})
				It("Logs", func() {
					Expect(buffer.String()).To(Equal("-----> Enabling location include file a/b/c\n"))
				})
			})

			Context("and sets directory", func() {
				BeforeEach(func() {
					mockYaml.EXPECT().Load(filepath.Join(buildDir, "Staticfile"), gomock.Any()).Do(func(_ string, hash *map[string]string) {
						(*hash)["directory"] = "any_string"
					})
				})
				It("sets location_include", func() {
					Expect(compiler.Config.DirectoryIndex).To(Equal(true))
				})
				It("Logs", func() {
					Expect(buffer.String()).To(Equal("-----> Enabling directory index for folders without index.html files\n"))
				})
			})

			Context("and sets ssi", func() {
				BeforeEach(func() {
					mockYaml.EXPECT().Load(filepath.Join(buildDir, "Staticfile"), gomock.Any()).Do(func(_ string, hash *map[string]string) {
						(*hash)["ssi"] = "enabled"
					})
				})
				It("sets ssi", func() {
					Expect(compiler.Config.SSI).To(Equal(true))
				})
				It("Logs", func() {
					Expect(buffer.String()).To(Equal("-----> Enabling SSI\n"))
				})
			})

			Context("and sets pushstate", func() {
				BeforeEach(func() {
					mockYaml.EXPECT().Load(filepath.Join(buildDir, "Staticfile"), gomock.Any()).Do(func(_ string, hash *map[string]string) {
						(*hash)["pushstate"] = "enabled"
					})
				})
				It("sets pushstate", func() {
					Expect(compiler.Config.PushState).To(Equal(true))
				})
				It("Logs", func() {
					Expect(buffer.String()).To(Equal("-----> Enabling pushstate\n"))
				})
			})

			Context("and sets http_strict_transport_security", func() {
				BeforeEach(func() {
					mockYaml.EXPECT().Load(filepath.Join(buildDir, "Staticfile"), gomock.Any()).Do(func(_ string, hash *map[string]string) {
						(*hash)["http_strict_transport_security"] = "true"
					})
				})
				It("sets pushstate", func() {
					Expect(compiler.Config.HSTS).To(Equal(true))
				})
				It("Logs", func() {
					Expect(buffer.String()).To(Equal("-----> Enabling HSTS\n"))
				})
			})

			Context("and sets force_https", func() {
				BeforeEach(func() {
					mockYaml.EXPECT().Load(filepath.Join(buildDir, "Staticfile"), gomock.Any()).Do(func(_ string, hash *map[string]string) {
						(*hash)["force_https"] = "true"
					})
				})
				It("sets force_https", func() {
					Expect(compiler.Config.ForceHTTPS).To(Equal(true))
				})
				It("Logs", func() {
					Expect(buffer.String()).To(Equal("-----> Enabling HTTPS redirect\n"))
				})
			})
		})

		Context("Staticfile.auth is present", func() {
			BeforeEach(func() {
				mockYaml.EXPECT().Load(gomock.Any(), gomock.Any())

				err = ioutil.WriteFile(filepath.Join(buildDir, "Staticfile.auth"), []byte("some credentials"), 0666)
				Expect(err).To(BeNil())
			})

			JustBeforeEach(func() {
				err = compiler.LoadStaticfile()
				Expect(err).To(BeNil())
			})

			It("sets BasicAuth", func() {
				Expect(compiler.Config.BasicAuth).To(Equal(true))
			})
			It("Logs", func() {
				Expect(buffer.String()).To(ContainSubstring("-----> Enabling basic authentication using Staticfile.auth\n"))
			})
		})

		Context("the staticfile exists and is not valid", func() {
			BeforeEach(func() {
				mockYaml.EXPECT().Load(filepath.Join(buildDir, "Staticfile"), gomock.Any()).Return(errors.New("a yaml parsing error"))
			})

			It("returns an error", func() {
				err = compiler.LoadStaticfile()
				Expect(err).NotTo(BeNil())
			})
		})
	})

	Describe("GetAppRootDir", func() {
		var (
			returnDir string
		)

		JustBeforeEach(func() {
			returnDir, err = compiler.GetAppRootDir()
		})

		Context("the staticfile has a root directory specified", func() {
			Context("the directory does not exist", func() {
				BeforeEach(func() {
					sf.RootDir = "not_exist"
				})

				It("logs the staticfile's root directory", func() {
					Expect(buffer.String()).To(ContainSubstring("-----> Root folder"))
					Expect(buffer.String()).To(ContainSubstring("not_exist"))

				})

				It("returns an error", func() {
					Expect(returnDir).To(Equal(""))
					Expect(err).NotTo(BeNil())
					Expect(err.Error()).To(ContainSubstring("the application Staticfile specifies a root directory"))
					Expect(err.Error()).To(ContainSubstring("that does not exist"))
				})
			})

			Context("the directory exists but is actually a file", func() {
				BeforeEach(func() {
					ioutil.WriteFile(filepath.Join(buildDir, "actually_a_file"), []byte("xxx"), 0666)
					sf.RootDir = "actually_a_file"
				})

				It("logs the staticfile's root directory", func() {
					Expect(buffer.String()).To(ContainSubstring("-----> Root folder"))
					Expect(buffer.String()).To(ContainSubstring("actually_a_file"))
				})

				It("returns an error", func() {
					Expect(returnDir).To(Equal(""))
					Expect(err).NotTo(BeNil())
					Expect(err.Error()).To(ContainSubstring("the application Staticfile specifies a root directory"))
					Expect(err.Error()).To(ContainSubstring("that is a plain file"))
				})
			})

			Context("the directory exists", func() {
				BeforeEach(func() {
					os.Mkdir(filepath.Join(buildDir, "a_directory"), 0777)
					sf.RootDir = "a_directory"
				})

				It("logs the staticfile's root directory", func() {
					Expect(buffer.String()).To(ContainSubstring("-----> Root folder"))
					Expect(buffer.String()).To(ContainSubstring("a_directory"))
				})

				It("returns the full directory path", func() {
					Expect(err).To(BeNil())
					Expect(returnDir).To(Equal(filepath.Join(buildDir, "a_directory")))
				})
			})
		})

		Context("the staticfile does not have an root directory", func() {
			BeforeEach(func() {
				sf.RootDir = ""
			})

			It("logs the build directory as the root directory", func() {
				Expect(buffer.String()).To(ContainSubstring("-----> Root folder"))
				Expect(buffer.String()).To(ContainSubstring(buildDir))
			})
			It("returns the build directory", func() {
				Expect(err).To(BeNil())
				Expect(returnDir).To(Equal(buildDir))
			})
		})
	})

	Describe("WriteProfileD", func() {
		var (
			info           os.FileInfo
			profileDScript string
		)
		BeforeEach(func() {
			profileDScript = filepath.Join(buildDir, ".profile.d", "staticfile.sh")
		})

		Context(".profile.d directory exists", func() {
			BeforeEach(func() {
				err = os.Mkdir(filepath.Join(buildDir, ".profile.d"), 0777)
				Expect(err).To(BeNil())
			})

			It("creates the file as an executable", func() {
				err = compiler.WriteProfileD()
				Expect(err).To(BeNil())
				Expect(profileDScript).To(BeAnExistingFile())

				info, err = os.Stat(profileDScript)
				Expect(err).To(BeNil())

				// make sure at least 1 executable bit is set
				Expect(info.Mode().Perm() & 0111).NotTo(Equal(os.FileMode(0000)))
			})

		})
		Context(".profile.d directory does not exist", func() {
			It("creates the file as an executable", func() {
				err = compiler.WriteProfileD()
				Expect(err).To(BeNil())
				Expect(profileDScript).To(BeAnExistingFile())

				info, err = os.Stat(profileDScript)
				Expect(err).To(BeNil())

				// make sure at least 1 executable bit is set
				Expect(info.Mode().Perm() & 0111).NotTo(Equal(0000))
			})
		})
	})
})
