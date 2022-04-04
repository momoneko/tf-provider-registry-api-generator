package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"reflect"
	"strings"

	"github.com/mollie/tf-provider-registry-api-generator/versions"
)

func assertDiscoveryDocument(outputDir string, v1ProvidersPath string) {
	content := make(map[string]string)
	// path.Join does not add the trailing slash
	v1ProvidersPath = path.Join("/", v1ProvidersPath) + "/"
	expect := map[string]string{
		"providers.v1": v1ProvidersPath,
	}

	p := path.Join(outputDir, ".well-known", "terraform.json")
	err := readJson(p, &content)
	if err != nil {
		log.Printf("could not read content of %s, %s", p, err)
	}

	if !reflect.DeepEqual(expect, content) {
		log.Printf("INFO: writing content to %s", p)
		err := os.MkdirAll(path.Dir(p), os.FileMode(0700))
		if err != nil {
			log.Fatalf("ERROR: could not create .well-known: %s", err)
		}
		writeJson(p, expect)
	} else {
		log.Printf("INFO: discovery document is up-to-date\n")
	}
}

func readJson(filename string, object interface{}) error {
	r, err := os.Open(filename)
	if err != nil {
		if err.Error() == "storage: object doesn't exist" {
			return nil
		}
		return fmt.Errorf("ERROR: failed to read file %s, %s", filename, err)
	}
	defer r.Close()
	body, err := ioutil.ReadAll(r)
	if err != nil {
		return fmt.Errorf("ERROR: failed to read content from %s, %s", filename, err)
	}
	err = json.Unmarshal(body, &object)
	if err != nil {
		return fmt.Errorf("ERROR: failed to unmarshal %s, %s", filename, err)
	}

	return nil
}

func readShasums(filename string, shasums map[string]string) error {
	r, err := os.Open(filename)
	if err != nil {
		if err.Error() == "storage: object doesn't exist" {
			return nil
		}
		return fmt.Errorf("ERROR: failed to read file %s, %s", filename, err)
	}
	defer r.Close()

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			log.Fatalf("ERROR: expected %s to contain 2 fields on each line, found %d", filename, len(fields))
		}
		shasums[fields[1]] = fields[0]
	}
	return nil
}

func writeJson(filename string, content interface{}) {
	log.Printf("INFO: writing %s", filename)

	f, err := os.Create(filename)
	if err != nil {
		log.Print(err)
		return
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(content)
	if err != nil {
		log.Fatalf("INFO: failed to write %s, %s", filename, err)
	}
}

func writeProviderVersions(directory string, newVersions *versions.ProviderVersions) {
	var existing versions.ProviderVersions
	if err := readJson(path.Join(directory, "versions"), &existing); err != nil {
		log.Printf("ERROR: failed to read the %s/versions, %s", directory, err)
	}
	if reflect.DeepEqual(&existing, newVersions) {
		log.Printf("INFO: %s/versions already up-to-date", directory)
		return
	}
	existing.Merge(*newVersions)
	writeJson(path.Join(directory, "versions"), existing)
}

func writeProviderVersion(directory string, version *versions.BinaryMetaData) {
	filename := path.Join(directory, version.Version, "download", version.Os, version.Arch)

	dirname := path.Dir(filename)
	fmt.Println("Making directory" + dirname)
	err := os.MkdirAll(dirname, os.FileMode(0744))
	if err != nil {
		log.Println("Couldn't create directory", err)
	}
	writeJson(filename, version)
}

func WriteAPIDocuments(namespace string, binaries versions.BinaryMetaDataList) {
	outputDir := "registry"
	v1ProvidersDir := path.Join("registry", "v1", "providers")
	assertDiscoveryDocument(outputDir, v1ProvidersDir)

	providerDirectory := path.Join(outputDir, v1ProvidersDir, namespace)
	providers := binaries.ExtractVersions()

	for _, binary := range binaries {
		writeProviderVersion(path.Join(providerDirectory, binary.TypeName), &binary)
	}

	for name, versions := range providers {
		writeProviderVersions(path.Join(providerDirectory, name), versions)
	}

}
