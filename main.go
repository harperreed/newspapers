
package main

import (
    "bytes"
    "crypto/sha256"
    "fmt"
    "html/template"
    "io"
    "io/ioutil"
    "image/jpeg"
    "log"
    "math/rand"
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "time"

    "github.com/PuerkitoBio/goquery"
    "github.com/gen2brain/go-fitz"
    "gopkg.in/yaml.v2"
)

// Config struct defines the structure of the configuration file.
type Config struct {
    PDFURLs   []string      `yaml:"pdf_urls"`   // List of PDF URLs to process.
    CacheTime time.Duration `yaml:"cache_time"` // Duration to cache the images before re-fetching.
}

// loadConfig reads a YAML configuration file and unmarshals it into a Config struct.
func loadConfig(file string) (*Config, error) {
    log.Printf("Loading configuration from file: %s", file)
    data, err := ioutil.ReadFile(file)
    if err != nil {
        log.Printf("Error reading config file: %v", err)
        return nil, err
    }

    var config Config
    err = yaml.Unmarshal(data, &config)
    if err != nil {
        log.Printf("Error unmarshaling config data: %v", err)
        return nil, err
    }

    log.Printf("Configuration loaded successfully")
    return &config, nil
}

// generateCacheFilename generates a filename for caching the image. It creates a unique
// filename by hashing the URL and appending the current date to ensure the cache file
// is easily identifiable and includes a timestamp for potential cache invalidation purposes.
// The SHA-256 hash function is used to generate a fixed-length hash from the URL.
// The resulting filename is in the format of "<hash>_<date>.jpg".
func generateCacheFilename(url string) string {
    // Generate a hash of the URL using SHA-256 for a unique identifier
    hash := sha256.Sum256([]byte(url))
    hashStr := fmt.Sprintf("%x", hash)
    log.Printf("Generated hash for URL '%s': %s", url, hashStr)

    // Get today's date in the format MM-DD-YYYY for appending to the filename
    today := time.Now().Format("01-02-2006")
    log.Printf("Today's date for cache filename: %s", today)

    // Create the cache filename using the hash and today's date
    fileName := fmt.Sprintf("%s_%s.jpg", hashStr, today)
    log.Printf("Generated cache filename: %s", fileName)

    return fileName
}

// getCoverURL fetches the cover image URL from a given webpage URL.
func getCoverURL(url string) (string, error) {
    log.Printf("Fetching cover URL from: %s", url)
    res, err := http.Get(url)
    if err != nil {
        log.Printf("Error fetching URL: %v", err)
        return "", err
    }
    defer res.Body.Close()

    doc, err := goquery.NewDocumentFromReader(res.Body)
    if err != nil {
        log.Printf("Error creating document from reader: %v", err)
        return "", err
    }

    imgTag := doc.Find("img#giornale-img")
    if imgTag.Length() > 0 {
        src, exists := imgTag.Attr("src")
        if exists {
            coverURL := "https://www.frontpages.com" + src
            log.Printf("Cover URL found: %s", coverURL)
            return coverURL, nil
        }
    }

    errMsg := "Image not found or missing 'src' attribute"
    log.Println(errMsg)
    return "", fmt.Errorf(errMsg)
}

func downloadImage(url string) error {
    log.Printf("Downloading image from URL: %s", url)

    var imageURL string
    if strings.HasPrefix(url, "https://www.frontpages.com") {
        // Fetch the cover URL for frontpages.com URLs
        var err error
        imageURL, err = getCoverURL(url)
        if err != nil {
            log.Printf("Error getting cover URL: %v", err)
            return err
        }
    } else {
        // Use the provided URL directly for other URLs
        imageURL = url
    }
    log.Printf("Image URL: %s", imageURL)

    res, err := http.Get(imageURL)
    if err != nil {
        log.Printf("Error fetching image: %v", err)
        return err
    }
    defer res.Body.Close()

    if res.StatusCode != http.StatusOK {
        log.Printf("Error fetching image: server returned non-200 status code: %d", res.StatusCode)
        return fmt.Errorf("server returned non-200 status code: %d", res.StatusCode)
    }

    cacheDir := "cache"
    if err := os.MkdirAll(cacheDir, os.ModePerm); err != nil {
        log.Printf("Error creating cache directory: %v", err)
        return err
    }

    // Generate the cache filename
    fileName := generateCacheFilename(url)

    cacheFile := filepath.Join(cacheDir, fileName)

    // Convert PDF to image if the URL points to a PDF
    if strings.HasSuffix(strings.ToLower(imageURL), ".pdf") {
        log.Printf("Converting PDF to image: %s", imageURL)
        imgData, err := convertPDFToImage(res.Body)
        if err != nil {
            log.Printf("Error converting PDF to image: %v", err)
            return err
        }
        if err := ioutil.WriteFile(cacheFile, imgData, 0644); err != nil {
            log.Printf("Error writing image data to file: %v", err)
            return err
        }
    } else {
        // Save the image directly if it's not a PDF
        file, err := os.Create(cacheFile)
        if err != nil {
            log.Printf("Error creating cache file: %v", err)
            return err
        }
        defer file.Close()

        log.Printf("Saving image to cache file: %s", cacheFile)
        imgData, err := ioutil.ReadAll(res.Body)
        if err != nil {
            log.Printf("Error reading image data: %v", err)
            return err
        }

        _, err = file.Write(imgData)
        if err != nil {
            log.Printf("Error writing image data to file: %v", err)
            return err
        }
    }

    log.Printf("Image downloaded and saved successfully")
    return nil
}

func convertPDFToImage(pdfData io.Reader) ([]byte, error) {
    tmpFile, err := ioutil.TempFile("", "temp-*.pdf")
    if err != nil {
        return nil, err
    }
    defer os.Remove(tmpFile.Name())

    _, err = io.Copy(tmpFile, pdfData)
    if err != nil {
        return nil, err
    }
    tmpFile.Close()

    doc, err := fitz.New(tmpFile.Name())
    if err != nil {
        return nil, err
    }
    defer doc.Close()

    img, err := doc.Image(0)
    if err != nil {
        return nil, err
    }

    var buf bytes.Buffer
    err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
    if err != nil {
        return nil, err
    }

    return buf.Bytes(), nil
}

// homeHandler is the HTTP handler for the home page.
func homeHandler(w http.ResponseWriter, r *http.Request) {
    log.Println("Serving home page")
    config, err := loadConfig("config.yaml")
    if err != nil {
        log.Printf("Error loading configuration: %v", err)
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
        return
    }



    // Get a random URL from the list
    rand.Seed(time.Now().UnixNano())
    currentURL := config.PDFURLs[rand.Intn(len(config.PDFURLs))]
    // Generate the cache filename
    fileName := generateCacheFilename(currentURL)


    cacheFile := filepath.Join("cache", fileName)

    if _, err := os.Stat(cacheFile); os.IsNotExist(err) || time.Since(getFileModTime(cacheFile)) > config.CacheTime {
        log.Printf("Image not in cache or cache expired, downloading new image")
        err := downloadImage(currentURL)
        if err != nil {
            log.Printf("Error downloading image: %v", err)
            http.Error(w, "No image available", http.StatusInternalServerError)
            return
        }
    } else {
        log.Printf("Using cached image: %s", cacheFile)
    }

    tmpl := template.Must(template.ParseFiles("templates/home_with_image.html"))
    data := struct {
        ImageURL string
    }{
        ImageURL: "/" + cacheFile,
    }
    err = tmpl.Execute(w, data)
    if err != nil {
        log.Printf("Error executing template: %v", err)
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
    }
}

// getFileModTime returns the modification time of the specified file.
func getFileModTime(file string) time.Time {
    info, err := os.Stat(file)
    if err != nil {
        log.Printf("Error getting file modification time: %v", err)
        return time.Time{}
    }
    return info.ModTime()
}

// main sets up the HTTP server and its routes.
func main() {
    http.HandleFunc("/", homeHandler)
    http.Handle("/cache/", http.StripPrefix("/cache/", http.FileServer(http.Dir("cache"))))

    log.Println("Server started on http://localhost:8080")
    if err := http.ListenAndServe(":8080", nil); err != nil {
        log.Fatalf("Error starting server: %v", err)
    }
}
