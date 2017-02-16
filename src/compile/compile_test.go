package main_test

import (
	c "compile"
	"io/ioutil"
	"os"

	bp "github.com/cloudfoundry/libbuildpack"

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

		compiler = &c.Compiler{BuildDir: buildDir,
			CacheDir: cacheDir,
			Manifest: nil,
			Log:      logger}
	})

	AfterEach(func() {
		err = os.RemoveAll(buildDir)
		Expect(err).To(BeNil())
	})

	Describe("GetAppRootDir", func() {
		var (
			returnDir string
		)
		Context("the staticfile has an alternate directory", func() {
			Context("the directory does not exist", func() {
				BeforeEach(func() {
					sf.RootDir = "not_exist"
				})

				It("does some error stuff", func() {
					returnDir, err = compiler.GetAppRootDir(sf)
					Expect(returnDir).To(Equal(""))
					Expect(err).NotTo(BeNil())
				})

			})
			Context("the directory exists but is actually a file", func() {

			})
			Context("the directory exists", func() {

			})

		})
	})

})
