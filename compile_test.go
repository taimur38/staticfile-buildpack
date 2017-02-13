package staticfile_buildpack_test

import (
	main "github.com/cloudfoundry/staticfile_buildpack"
	"github.com/cloudfoundry/staticfile_buildpack/mocks"
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/golang/mock/gomock"
	"errors"
)

//go:generate mockgen -package mocks -destination mocks/libbuildpack.go github.com/cloudfoundry/libbuildpack Manifest,Logger

var _ = Describe("Compile", func() {
	var buildDir, cacheDir string
	var mockCtrl *gomock.Controller

	BeforeEach(func() {
		var err error
		mockCtrl = gomock.NewController(GinkgoT())
		buildDir, err = ioutil.TempDir("", "")
		Expect(err).ToNot(HaveOccurred())
		cacheDir, err = ioutil.TempDir("", "")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(buildDir)
		os.RemoveAll(cacheDir)
		mockCtrl.Finish()
	})

	It("logs the buildpack version", func() {
		mockLogger := mocks.NewMockLogger(mockCtrl)
		mockManifest := mocks.NewMockManifest(mockCtrl)

		mockManifest.EXPECT().Version().Return("1.38", nil)
		mockLogger.EXPECT().BeginStep("Staticfile Buildpack Version %s", "1.38")

		err := main.Compile(buildDir, cacheDir, mockManifest, mockLogger)
		Expect(err).To(BeNil())
	})

	It("logs error if no buildpack version", func() {
		mockLogger := mocks.NewMockLogger(mockCtrl)
		mockManifest := mocks.NewMockManifest(mockCtrl)

		expectedError := errors.New("unknown")
		mockManifest.EXPECT().Version().Return("", expectedError)
		mockLogger.EXPECT().Error("Could not determine buildpack version")

		err := main.Compile(buildDir, cacheDir, mockManifest, mockLogger)
		Expect(err).To(Equal(expectedError))
	})
})
