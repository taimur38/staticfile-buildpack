package staticfile_buildpack

import
(
	bp "github.com/cloudfoundry/libbuildpack"
	"os"
	"path/filepath"
)

func main() {
	build_dir := os.Args[1]
	cache_dir := os.Args[2]
        log := bp.NewLogger()
	bp_dir := os.Getenv("BUILDPACK_DIR")
	manifest, err := bp.NewManifest(filepath.Join(bp_dir, "manifest.yml"))
	if err != nil {
		panic(err)
	}

	err = Compile(build_dir, cache_dir, manifest, log)
	if err != nil {
		panic(err)
	}
}

func Compile(build_dir, cache_dir string, manifest bp.Manifest, log bp.Logger) error {
	version, err := manifest.Version()
	if err != nil {
		log.Error("Could not determine buildpack version")
		return err
	}

	log.BeginStep("Staticfile Buildpack Version %s", version)

	return nil
}
