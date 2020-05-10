package main

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	args := os.Args[1:]
	structure := make(map[string]string)
	pathRewrites := make(map[string]string)
	basedir := "site"
	/**
	@fixme need to change this into a proper options based thing
	*/
	if len(args) > 0 {
		basedir = args[0]
	}
	if _, err := os.Stat(basedir + "/index.htm"); os.IsNotExist(err) {
		if _, err := os.Stat(basedir + "/index.html"); os.IsNotExist(err) {
			os.Stderr.WriteString("No index.htm or index.html file found")
			os.Exit(1)

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

	f, err := os.Create("webserver_main.go")
	f.WriteString(`package main
		import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
)
`)
	handleFunc := make(map[string]string) // key is full path, value is uniqueid
	// need to include "encoding/base64" when generating code
	for k, v := range structure {
		fileContents, _ := ioutil.ReadFile(k)
		uniqueid := "x" + fmt.Sprintf("%x", md5.Sum([]byte(k)))
		f.WriteString(`func ` + uniqueid + `(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "` + v + `")
	sDec, _ := base64.StdEncoding.DecodeString("` + base64.StdEncoding.EncodeToString(fileContents) + `")
	fmt.Fprint(w, string(sDec))
}

`)
		handleFunc[pathRewrites[k]] = `http.HandleFunc("/` + pathRewrites[k] + `", ` + uniqueid + `)`
		//if pathRewrites[k] == "index.htm" || pathRewrites[k] == "index.html" {
		//	handleFunc["DEFAULT"] = `http.HandleFunc("/", ` + uniqueid + `)`
		//}
		if strings.HasSuffix(pathRewrites[k], "index.htm") {
			handleFunc["DEFAULT"+pathRewrites[k]] = `http.HandleFunc("` + strings.Replace(pathRewrites[k], "index.htm", "", 1) + `", ` + uniqueid + `)`
		}
		if strings.HasSuffix(pathRewrites[k], "index.html") {
			handleFunc["DEFAULT"+pathRewrites[k]] = `http.HandleFunc("` + strings.Replace(pathRewrites[k], "index.html", "", 1) + `", ` + uniqueid + `)`
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
