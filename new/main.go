package main

import (
	"archive/tar"
	"compress/gzip"
	"database/sql"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

func main() {
	connectToDB()

	http.HandleFunc("/", uploadPage)
	http.HandleFunc("/validate", validateHandler)
	http.HandleFunc("/submit", submitHandler)

	log.Println("✅ Server started at :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func connectToDB() {
	dbUser := "root"
	dbPassword := "dbuser123"
	dbHost := "mysql-db"
	dbPort := "3306"
	dbName := "helmTemplate"
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", dbUser, dbPassword, dbHost, dbPort, dbName)

	for i := 1; i <= 10; i++ {
		var err error
		db, err = sql.Open("mysql", dsn)
		if err != nil {
			log.Printf("❌ Attempt %d: sql.Open error: %v", i, err)
		} else {
			err = db.Ping()
			if err == nil {
				log.Println("✅ Connected to MySQL database")
				return
			}
			log.Printf("❌ Attempt %d: db.Ping error: %v", i, err)
		}
		time.Sleep(3 * time.Second)
	}
	log.Println("❌ Could not connect to DB after retries")
	db = nil
}

func uploadPage(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/index.html"))
	tmpl.Execute(w, nil)
}

func validateHandler(w http.ResponseWriter, r *http.Request) {
	chartPath, _, err := processChartUpload(r)
	if err != nil {
		http.Error(w, "Validation failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	output, err := runHelmTemplate(chartPath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, output)
		return
	}

	fmt.Fprint(w, output)
}

func submitHandler(w http.ResponseWriter, r *http.Request) {
	if db == nil {
		http.Error(w, "Database not connected. Cannot store metadata.", http.StatusServiceUnavailable)
		return
	}

	chartPath, filename, err := processChartUpload(r)
	if err != nil {
		http.Error(w, "Upload failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	output, err := runHelmTemplate(chartPath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, output)
		return
	}

	chartName := filepath.Base(chartPath)
	storeChartMetadata(chartName, filename)

	fmt.Fprintf(w, "✅ Chart stored successfully!\n\nHelm Output:\n\n%s", output)
}

func processChartUpload(r *http.Request) (string, string, error) {
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		return "", "", err
	}

	file, handler, err := r.FormFile("chart")
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	uploadDir := "uploads"
	os.MkdirAll(uploadDir, 0755)

	uploadPath := filepath.Join(uploadDir, handler.Filename)
	out, err := os.Create(uploadPath)
	if err != nil {
		return "", "", err
	}
	defer out.Close()
	_, err = io.Copy(out, file)
	if err != nil {
		return "", "", err
	}

	baseName := strings.TrimSuffix(handler.Filename, filepath.Ext(handler.Filename))
	tempDir := filepath.Join("charts", baseName)
	os.MkdirAll(tempDir, 0755)

	err = untar(uploadPath, tempDir)
	if err != nil {
		return "", "", err
	}

	chartPath, err := findChartYAML(tempDir)
	if err != nil {
		return "", "", err
	}

	return chartPath, handler.Filename, nil
}

func untar(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	var tr *tar.Reader
	if strings.HasSuffix(src, ".tar.gz") || strings.HasSuffix(src, ".tgz") {
		gzr, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("gzip error: %w", err)
		}
		defer gzr.Close()
		tr = tar.NewReader(gzr)
	} else if strings.HasSuffix(src, ".tar") {
		tr = tar.NewReader(f)
	} else {
		return fmt.Errorf("unsupported file type: must be .tar or .tar.gz")
	}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dest, hdr.Name)
		if hdr.Typeflag == tar.TypeDir {
			os.MkdirAll(target, 0755)
		} else {
			os.MkdirAll(filepath.Dir(target), 0755)
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			_, err = io.Copy(outFile, tr)
			outFile.Close()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func findChartYAML(basePath string) (string, error) {
	var chartPath string
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name() == "Chart.yaml" {
			chartPath = filepath.Dir(path)
			return io.EOF
		}
		return nil
	})
	if err != nil && err != io.EOF {
		return "", err
	}
	if chartPath == "" {
		return "", fmt.Errorf("Chart.yaml not found")
	}
	return chartPath, nil
}

func runHelmTemplate(chartDir string) (string, error) {
	cmd := exec.Command("helm", "template", chartDir)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func storeChartMetadata(name, filename string) {
	if db == nil {
		log.Println("⚠️ DB not connected; skipping insert.")
		return
	}

	_, err := db.Exec("INSERT INTO charts (name, filename) VALUES (?, ?)", name, filename)
	if err != nil {
		log.Println("DB insert error:", err)
	}
}

