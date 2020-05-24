package main

import (
	"crypto/md5"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	structure := make(map[string]string)
	pathRewrites := make(map[string]string)
	var namespace = ""
	var outFile = ""
	var basedir = ""
	required := []string{"o", "src"}

	flag.StringVar(&outFile, "o", "", "Name of the output file containing the code (required)")
	flag.StringVar(&basedir, "src", "", "Location of the webpages to compile (required)")
	flag.StringVar(&namespace, "pkg", "main", "Package name to generate the code under (Defaults to main)")
	flag.Parse()

	seen := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { seen[f.Name] = true })
	for _, req := range required {
		if !seen[req] {
			// or possibly use `log.Fatalf` instead of:
			fmt.Fprintf(os.Stderr, "missing required -%s - refer to "+os.Args[0]+" --help for more information\n", req)
			os.Exit(1) // the same exit code flag.Parse uses
		}
	}

	if _, err := os.Stat(basedir + "/index.htm"); os.IsNotExist(err) {
		if _, err := os.Stat(basedir + "/index.html"); os.IsNotExist(err) {
			os.Stderr.WriteString("No index.htm or index.html file found")
			os.Exit(2)

		}
	}
	separator := fmt.Sprintf("%c", os.PathSeparator)
	err := filepath.Walk(basedir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			// Open File
			f, err := os.Open(path)
			if err == nil {

				defer f.Close()

				// Get the content
				contentType, err := GetFileContentType(f)
				if err == nil {
					extension := filepath.Ext(path)
					switch extension {
					case ".js":
						contentType = strings.Replace(contentType, "text/plain", "text/javascript", 1)
					case ".css":
						contentType = strings.Replace(contentType, "text/plain", "text/css", 1)
					}
					/**
					Sort this out - the path stuff needs to be cleaned up a lot more, especially cleaning up the stuff, etc.
					*/
					pathReplacement := strings.Replace(basedir+separator, separator+separator, separator, 1)
					structure[path] = contentType
					pathRewrites[path] = strings.Replace(strings.Replace(path, pathReplacement, "", 1), separator, "/", 999)
					//fmt.Println(path + " : " + contentType + " rewritten to " + strings.Replace(strings.Replace(path, pathReplacement, "", 1), separator, "/", 999))
				}

			}
			return nil
		})
	if err != nil {
		log.Println(err)
	}

	/**
	Start working through the files and compile them into a package
	*/

	f, err := os.Create(outFile)
	f.WriteString(`package ` + namespace + `
		import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
)
`)
	handleFunc := make(map[string]string) // key is full path, value is uniqueid
	// need to include "encoding/base64" when generating code
	for k, v := range structure {
		fileContents, _ := ioutil.ReadFile(k)
		uniqueid := "x" + fmt.Sprintf("%x", md5.Sum([]byte(k)))
		f.WriteString(`func ` + uniqueid + `(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "` + v + `")
	w.Header().Set("Server", "dxn.pw compilatron")
	sDec, _ := base64.StdEncoding.DecodeString("` + base64.StdEncoding.EncodeToString(fileContents) + `")
	fmt.Fprint(w, string(sDec))
}

`)
		handleFunc[pathRewrites[k]] = `http.HandleFunc("/` + pathRewrites[k] + `", ` + uniqueid + `)`
		if strings.HasSuffix(pathRewrites[k], "index.htm") {
			handleFunc["DEFAULT"+pathRewrites[k]] = `http.HandleFunc("/` + strings.Replace(pathRewrites[k], "index.htm", "", 1) + `", ` + uniqueid + `)`
		}
		if strings.HasSuffix(pathRewrites[k], "index.html") {
			handleFunc["DEFAULT"+pathRewrites[k]] = `http.HandleFunc("/` + strings.Replace(pathRewrites[k], "index.html", "", 1) + `", ` + uniqueid + `)`
		}

	}

	/**
	  Write handlers for all the urls
	*/
	f.WriteString(`func main() {
`)
	for _, v := range handleFunc {
		f.WriteString(v + "\n")
	}
	f.WriteString(`
log.Fatal(http.ListenAndServe(":8080", nil))
}
`)
	f.Close()

}

func GetFileContentType(out *os.File) (string, error) {

	// Only the first 512 bytes are used to sniff the content type.
	buffer := make([]byte, 512)

	_, err := out.Read(buffer)
	if err != nil {
		return "", err
	}

	// Use the net/http package's handy DectectContentType function. Always returns a valid
	// content-type by returning "application/octet-stream" if no others seemed to match.
	contentType := http.DetectContentType(buffer)

	return contentType, nil
}
