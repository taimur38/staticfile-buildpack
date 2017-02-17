package main_test

import (
	c "compile"
	"io/ioutil"
	"os"
	"path/filepath"

	bp "github.com/cloudfoundry/libbuildpack"

	"bytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Compile", func() {
	var (
		sf       c.Staticfile
		err      error
		buildDir string
		cacheDir string
		manifest bp.Manifest
		compiler *c.StaticfileCompiler
		logger   bp.Logger
	)

	BeforeEach(func() {
		buildDir, err = ioutil.TempDir("", "build")
		Expect(err).To(BeNil())

		cacheDir, err = ioutil.TempDir("", "cache")
		Expect(err).To(BeNil())

		manifest, err = bp.NewManifest("fixtures/standard_manifest")
		Expect(err).To(BeNil())

		logger = bp.NewLogger()
		logger.SetOutput(ioutil.Discard)
	})

	JustBeforeEach(func() {

		bpc := &bp.Compiler{BuildDir: buildDir,
			CacheDir: cacheDir,
			Manifest: manifest,
			Log:      logger}
		compiler = &c.StaticfileCompiler{Compiler: bpc, Config: sf}
	})

	Describe("LoadStaticfile", func() {
		Context("the staticfile does not exist", func() {
			BeforeEach(func() {
				buildDir = "fixtures/no_staticfile"
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
				Expect(compiler.Config.DirectoryIndex).To(Equal(""))
				Expect(compiler.Config.SSI).To(Equal(""))
				Expect(compiler.Config.PushState).To(Equal(""))
				Expect(compiler.Config.HSTS).To(Equal(false))

			})
		})
		Context("the staticfile exists and is valid", func() {
			BeforeEach(func() {
				buildDir = "fixtures/valid_staticfile"
			})

			It("loads the staticfile into the compiler struct", func() {
				err = compiler.LoadStaticfile()
				Expect(err).To(BeNil())
				Expect(compiler.Config.RootDir).To(Equal("root_test"))
				Expect(compiler.Config.HostDotFiles).To(Equal(true))
				Expect(compiler.Config.LocationInclude).To(Equal("location_include_test"))
				Expect(compiler.Config.DirectoryIndex).To(Equal("directory_test"))
				Expect(compiler.Config.SSI).To(Equal("ssi_test"))
				Expect(compiler.Config.PushState).To(Equal("pushstate_test"))
				Expect(compiler.Config.HSTS).To(Equal(true))
			})
		})
		Context("the staticfile exists and is not valid", func() {
			BeforeEach(func() {
				buildDir = "fixtures/invalid_staticfile"
			})

			It("returns an error", func() {
				err = compiler.LoadStaticfile()
				Expect(err).NotTo(BeNil())
			})
		})
	})

	Describe("GetAppRootDir", func() {
		var (
			buffer    *bytes.Buffer
			returnDir string
		)
		BeforeEach(func() {
			buffer = new(bytes.Buffer)
			logger = bp.NewLogger()
			logger.SetOutput(buffer)
		})

		Context("the staticfile has a root directory specified", func() {
			Context("the directory does not exist", func() {
				BeforeEach(func() {
					sf.RootDir = "not_exist"
				})

				It("logs the staticfile's root directory", func() {
					returnDir, err = compiler.GetAppRootDir()
					Expect(buffer.String()).To(ContainSubstring("-----> Root folder"))
					Expect(buffer.String()).To(ContainSubstring("not_exist"))

				})

				It("returns an error", func() {
					returnDir, err = compiler.GetAppRootDir()
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
					returnDir, err = compiler.GetAppRootDir()
					Expect(buffer.String()).To(ContainSubstring("-----> Root folder"))
					Expect(buffer.String()).To(ContainSubstring("actually_a_file"))

				})

				It("returns an error", func() {
					returnDir, err = compiler.GetAppRootDir()
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
					returnDir, err = compiler.GetAppRootDir()
					Expect(buffer.String()).To(ContainSubstring("-----> Root folder"))
					Expect(buffer.String()).To(ContainSubstring("a_directory"))

				})

				It("returns the full directory path", func() {
					returnDir, err = compiler.GetAppRootDir()
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
				returnDir, err = compiler.GetAppRootDir()
				Expect(buffer.String()).To(ContainSubstring("-----> Root folder"))
				Expect(buffer.String()).To(ContainSubstring(buildDir))
			})
			It("returns the build directory", func() {
				returnDir, err = compiler.GetAppRootDir()
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
