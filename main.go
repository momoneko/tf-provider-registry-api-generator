package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/docopt/docopt-go"
	"github.com/mollie/tf-provider-registry-api-generator/signing_key"
	"github.com/mollie/tf-provider-registry-api-generator/versions"
)

type Options struct {
	Namespace          string
	Url                string
	Fingerprint        string
	Protocols          string
	Help               bool
	Version            bool
	ProviderReleaseDir string
	protocols          []string
}

var (
	version       = "dev"
	commit        = "none"
	date          = "unknown"
	builtBy       = "unknown"
	protocolRegex = regexp.MustCompile(`^[0-9]+\.[0-9]+$`)
)

func main() {
	var options Options
	usage := `generate terraform provider registry API documents.

Usage:
  tf-provider-registry-api-generator  [--fingerprint FINGERPRINT] [--protocols PROTOCOLS] --url URL --namespace NAMESPACE --provider-release-dir PROVIDER_RELEASE_DIR
  tf-provider-registry-api-generator version
  tf-provider-registry-api-generator -h | --help

Options:
  --url URL                                   - of the static website.
  --namespace NAMESPACE                       - for the providers.
  --fingerprint FINGERPRINT                   - of the public key used to sign, defaults to environment variable GPG_FINGERPRINT.
  --protocols PROTOCOL                        - comma separated list of supported provider protocols by the provider [default: 5.0]
  --provider-release-dir PROVIDER_RELEASE_DIR - directory where the packed provider zips are located
  -h --help                  - shows this.
`

	arguments, err := docopt.ParseDoc(usage)
	if err != nil {
		log.Fatalf("ERROR: failed to parse command line, %s", err)
	}
	if err = arguments.Bind(&options); err != nil {
		log.Fatalf("ERROR: failed to bind arguments from command line, %s", err)
	}

	if options.Version {
		fmt.Printf("%s\n", version)
		os.Exit(0)
	}

	options.protocols = make([]string, 0)
	for _, p := range strings.Split(options.Protocols, ",") {
		if !protocolRegex.Match([]byte(p)) {
			log.Fatalf("ERROR: %s is not a version number", p)
		}
		options.protocols = append(options.protocols, p)
	}
	if len(options.protocols) == 0 {
		log.Fatalf("ERROR: no protocols specified")
	}

	if options.Fingerprint == "" {
		options.Fingerprint = os.Getenv("GPG_FINGERPRINT")
		if options.Fingerprint == "" {
			log.Fatalf("ERROR: no fingerprint specified")
		}
	}

	signingKey := signing_key.GetPublicSigningKey(options.Fingerprint)

	filenames := make([]string, 5)
	err = filepath.Walk(
		options.ProviderReleaseDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			filenames = append(filenames, path)
			return nil
		})
	if err != nil {
		log.Println(err)
	}

	shasums := make(map[string]string)
	for _, filename := range filenames {
		if strings.HasSuffix(filename, "SHA256SUMS") {
			err = readShasums(filename, shasums)
			if err != nil {
				log.Fatalf("%s", err)
			}
		}
	}

	binaries := versions.CreateFromFileList(filenames, options.Url, signingKey, shasums, options.protocols)
	providers := binaries.ExtractVersions()
	if len(providers) == 0 {
		log.Fatalf("ERROR: no terraform provider binaries detected")
	}

	WriteAPIDocuments(options.Namespace, binaries)
}
