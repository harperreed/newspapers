package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

// TestLoadConfig tests the loadConfig function
func TestLoadConfig(t *testing.T) {
	// Test successful config loading
	tempFile, err := ioutil.TempFile("", "config*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tempFile.Name())

	configData := []byte(`
pdf_urls:
  - https://example.com/newspaper1.pdf
  - https://example.com/newspaper2.pdf
cache_time: 1h
`)
	_, err = tempFile.Write(configData)
	assert.NoError(t, err)
	tempFile.Close()

	config, err := loadConfig(tempFile.Name())
	assert.NoError(t, err)
	assert.Equal(t, 2, len(config.PDFURLs))
	assert.Equal(t, time.Hour, config.CacheTime)

	// Test error handling for missing file
	_, err = loadConfig("non_existent_file.yaml")
	assert.Error(t, err)

	// Test error handling for invalid YAML
	invalidFile, err := ioutil.TempFile("", "invalid_config*.yaml")
	assert.NoError(t, err)
	defer os.Remove(invalidFile.Name())

	invalidData := []byte(`
pdf_urls:
  - https://example.com/newspaper1.pdf
cache_time: invalid
`)
	_, err = invalidFile.Write(invalidData)
	assert.NoError(t, err)
	invalidFile.Close()

	_, err = loadConfig(invalidFile.Name())
	assert.Error(t, err)
}

// TestGenerateCacheFilename tests the generateCacheFilename function
func TestGenerateCacheFilename(t *testing.T) {
	url := "https://example.com/newspaper.pdf"
	filename1 := generateCacheFilename(url)
	filename2 := generateCacheFilename(url)

	// Test correct filename generation
	assert.Contains(t, filename1, ".jpg")
	assert.Contains(t, filename2, ".jpg")

	// Test consistency for the same URL
	assert.Equal(t, filename1[:64], filename2[:64]) // Compare hash part
}

// TestGetCoverURL tests the getCoverURL function
func TestGetCoverURL(t *testing.T) {
	// Test successful URL extraction
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html><body><img id="giornale-img" src="/cover.jpg"></body></html>`))
	}))
	defer server.Close()

	coverURL, err := getCoverURL(server.URL)
	assert.NoError(t, err)
	assert.Equal(t, "https://www.frontpages.com/cover.jpg", coverURL)

	// Test error handling for invalid HTML
	invalidServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html><body>No image here</body></html>`))
	}))
	defer invalidServer.Close()

	_, err = getCoverURL(invalidServer.URL)
	assert.Error(t, err)
}

// TestDownloadImage tests the downloadImage function
func TestDownloadImage(t *testing.T) {
	// Test successful image download
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fake image data"))
	}))
	defer server.Close()

	err := downloadImage(server.URL)
	assert.NoError(t, err)

	// Test error handling for network issues
	err = downloadImage("http://non-existent-url.com")
	assert.Error(t, err)
}

// TestConvertPDFToImage tests the convertPDFToImage function
func TestConvertPDFToImage(t *testing.T) {
	// Test successful PDF to image conversion
	pdfData := []byte("%PDF-1.7\n1 0 obj\n<<\n/Type /Catalog\n/Pages 2 0 R\n>>\nendobj\n2 0 obj\n<<\n/Type /Pages\n/Kids [3 0 R]\n/Count 1\n>>\nendobj\n3 0 obj\n<<\n/Type /Page\n/Parent 2 0 R\n/Resources <<\n/Font <<\n/F1 4 0 R\n>>\n>>\n/MediaBox [0 0 300 144]\n/Contents 5 0 R\n>>\nendobj\n4 0 obj\n<<\n/Type /Font\n/Subtype /Type1\n/BaseFont /Helvetica\n>>\nendobj\n5 0 obj\n<< /Length 55 >>\nstream\nBT\n/F1 12 Tf\n100 100 Td\n(Hello, World!) Tj\nET\nendstream\nendobj\nxref\n0 6\n0000000000 65535 f \n0000000009 00000 n \n0000000058 00000 n \n0000000115 00000 n \n0000000274 00000 n \n0000000341 00000 n \ntrailer\n<<\n/Size 6\n/Root 1 0 R\n>>\nstartxref\n447\n%%EOF")

	imgData, err := convertPDFToImage(bytes.NewReader(pdfData))
	assert.NoError(t, err)
	assert.NotEmpty(t, imgData)

	// Test error handling for invalid PDF data
	invalidPDFData := []byte("This is not a valid PDF")
	_, err = convertPDFToImage(bytes.NewReader(invalidPDFData))
	assert.Error(t, err)
}
