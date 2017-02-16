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
		compiler *c.Compiler
		logger   bp.Logger
	)

	BeforeEach(func() {
		buildDir, err = ioutil.TempDir("", "build")
		Expect(err).To(BeNil())

		cacheDir, err = ioutil.TempDir("", "cache")
		Expect(err).To(BeNil())

		manifest, err = bp.NewManifest("fixtures/standard")
		Expect(err).To(BeNil())

		logger = bp.NewLogger()
		logger.SetOutput(ioutil.Discard)
	})

	JustBeforeEach(func() {
		compiler = &c.Compiler{BuildDir: buildDir,
			CacheDir: cacheDir,
			Manifest: nil,
			Log:      logger}
	})

	AfterEach(func() {
		err = os.RemoveAll(buildDir)
		Expect(err).To(BeNil())

		err = os.RemoveAll(cacheDir)
		Expect(err).To(BeNil())
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
					returnDir, err = compiler.GetAppRootDir(sf)
					Expect(buffer.String()).To(ContainSubstring("-----> Root folder"))
					Expect(buffer.String()).To(ContainSubstring("not_exist"))

				})

				It("returns an error", func() {
					returnDir, err = compiler.GetAppRootDir(sf)
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
					returnDir, err = compiler.GetAppRootDir(sf)
					Expect(buffer.String()).To(ContainSubstring("-----> Root folder"))
					Expect(buffer.String()).To(ContainSubstring("actually_a_file"))

				})

				It("returns an error", func() {
					returnDir, err = compiler.GetAppRootDir(sf)
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
					returnDir, err = compiler.GetAppRootDir(sf)
					Expect(buffer.String()).To(ContainSubstring("-----> Root folder"))
					Expect(buffer.String()).To(ContainSubstring("a_directory"))

				})

				It("returns the full directory path", func() {
					returnDir, err = compiler.GetAppRootDir(sf)
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
				returnDir, err = compiler.GetAppRootDir(sf)
				Expect(buffer.String()).To(ContainSubstring("-----> Root folder"))
				Expect(buffer.String()).To(ContainSubstring(buildDir))
			})
			It("returns the build directory", func() {
				returnDir, err = compiler.GetAppRootDir(sf)
				Expect(err).To(BeNil())
				Expect(returnDir).To(Equal(buildDir))
			})
		})
	})
})
