package versions

import (
	"fmt"
	"log"
	"path"
	"reflect"
	"regexp"

	"github.com/mollie/tf-provider-registry-api-generator/signing_key"
)

type GpgSigningKey struct {
	KeyID          string  `json:"key_id"`
	ASCIIArmor     string  `json:"ascii_armor"`
	TrustSignature string  `json:"trust_signature"`
	Source         string  `json:"source"`
	SourceURL      *string `json:"source_url"`
}

type BinaryMetaData struct {
	Protocols           []string `json:"protocols"`
	Os                  string   `json:"os"`
	Arch                string   `json:"arch"`
	Filename            string   `json:"filename"`
	DownloadURL         string   `json:"download_url"`
	ShasumsURL          string   `json:"shasums_url"`
	ShasumsSignatureURL string   `json:"shasums_signature_url"`
	Shasum              string   `json:"shasum"`
	SigningKeys         struct {
		GpgPublicKeys []GpgSigningKey `json:"gpg_public_keys"`
	} `json:"signing_keys"`
	Version  string `json:"-"`
	TypeName string `json:"-"`
}

func (l *BinaryMetaData) Equals(o *BinaryMetaData) bool {
	result := l.Os == o.Os &&
		l.Arch == o.Arch &&
		l.Filename == o.Filename &&
		l.ShasumsURL == o.ShasumsURL &&
		l.ShasumsSignatureURL == o.ShasumsSignatureURL &&
		l.Shasum == o.Shasum

	result = result && reflect.DeepEqual(l.Protocols, o.Protocols)
	result = result && reflect.DeepEqual(l.SigningKeys, o.SigningKeys)
	return result
}

type BinaryMetaDataList []BinaryMetaData

func (l BinaryMetaDataList) ExtractVersions() map[string]*ProviderVersions {
	result := make(map[string]*ProviderVersions)
	for _, meta := range l {
		versions, ok := result[meta.TypeName]
		if !ok {
			versions = &ProviderVersions{}
			result[meta.TypeName] = versions
		}
		versions.Add(&meta)
	}
	return result
}

func (m *BinaryMetaData) Platform() Platform {
	return Platform{Os: m.Os, Arch: m.Arch}
}

var (
	releaseName = regexp.MustCompile(`(terraform-provider-)(?P<type>[^_]*)_(?P<version>[0-9]+\.[0-9]+\.[0-9]+)_((SHA256SUMS.*|((?P<os>[^_]*)_(?P<arch>[^.]*)(\.zip))))`)

	binaryNameExpression = regexp.MustCompile(`(terraform-provider-)(?P<type>[^_]*)_(?P<version>[0-9]+\.[0-9]+\.[0-9]+)_(?P<os>[^_]*)_(?P<arch>[^.]*)(\.zip)`)
	subExpressionNames   = binaryNameExpression.SubexpNames()
)

func MakeFromFileName(baseURL string, filePath string, shasums map[string]string, protocols []string) *BinaryMetaData {
	filename := path.Base(filePath)
	matches := binaryNameExpression.FindStringSubmatch(filename)
	if matches == nil {
		return nil
	}
	metadata := BinaryMetaData{}
	for i, name := range subExpressionNames {
		switch name {
		case "type":
			metadata.TypeName = matches[i]
		case "version":
			metadata.Version = matches[i]
		case "os":
			metadata.Os = matches[i]
		case "arch":
			metadata.Arch = matches[i]
		default:
			// ignore
		}
	}

	metadata.DownloadURL = fmt.Sprintf("%s/%s", baseURL, filename)
	metadata.Protocols = protocols
	metadata.ShasumsURL = fmt.Sprintf("%s/terraform-provider-%s_%s_SHA256SUMS",
		baseURL, metadata.TypeName, metadata.Version)
	metadata.ShasumsSignatureURL = fmt.Sprintf("%s/terraform-provider-%s_%s_SHA256SUMS.sig",
		baseURL, metadata.TypeName, metadata.Version)
	metadata.Filename = filename

	var ok bool
	if metadata.Shasum, ok = shasums[filename]; !ok {
		log.Fatalf("ERROR: no shasum found found %s", filename)
	}

	return &metadata
}

func CreateFromFileList(files []string, baseURL string, signingKey signing_key.PGPSigningKey, shasums map[string]string, protocols []string) BinaryMetaDataList {

	result := make(BinaryMetaDataList, 0, len(files))

	for _, f := range files {
		log.Println("Writing file", f)
		metadata := MakeFromFileName(baseURL, f, shasums, protocols)
		if metadata != nil {
			result = append(result, *metadata)
		}
	}

	result.SetPGPSigningKey(signingKey)

	return result
}

func (l BinaryMetaDataList) SetPGPSigningKey(signingKey signing_key.PGPSigningKey) {
	for i := range l {
		(l)[i].SigningKeys.GpgPublicKeys = []GpgSigningKey{{KeyID: signingKey.KeyID,
			ASCIIArmor: signingKey.ASCIIArmor}}
	}
}
