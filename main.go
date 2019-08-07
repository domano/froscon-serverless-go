package main

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/gcsblob"
	_ "gocloud.dev/blob/memblob"
	_ "gocloud.dev/blob/s3blob"
	"gocloud.dev/gcerrors"
	"gocloud.dev/requestlog"
	"gocloud.dev/server"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
)

var tpl *template.Template

const maxSize = 10E7

func main() {
	bucketPath, port := readEnv()
	tpl, _ = template.New("index").Parse(html)

	// Open Gocloud bucket
	bucket, err := blob.OpenBucket(context.Background(), bucketPath)
	if err != nil {
		log.Fatal(err)
	}

	// Router for client requests
	r := mux.NewRouter()
	r.Methods(http.MethodGet).Path("/").HandlerFunc(listHandler(bucket))
	r.Methods(http.MethodGet).Path("/{image}").HandlerFunc(getFileHandler(bucket))
	r.Methods(http.MethodPost).Path("/").HandlerFunc(postFileHandler(bucket))

	// Gocloud server with Logger
	srv := server.New(r,
		&server.Options{
			RequestLogger: requestlog.NewNCSALogger(os.Stdout, nil),
		},
	)

	// Die and log if the server throws an error
	log.Fatal(srv.ListenAndServe(fmt.Sprintf(":%s", port)))
}

// Read Bucket Path and Port
func readEnv() (bucketPath string, port string) {
	bucketPath = os.Getenv("BUCKET_URI")
	if bucketPath == "" {
		log.Fatalln("No Bucket URI set.")
	}
	port = os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return bucketPath, port
}

// HTTP Handler used for listing the images using the provided HTML-Template
func listHandler(bucket *blob.Bucket) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list := bucket.List(nil)
		var images []string
		for {
			item, err := list.Next(context.Background())
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatal(err)
			}
			images = append(images, item.Key)
		}

		err := tpl.Execute(w, struct{ Images []string }{images})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		}
	}
}

// HTTP-Handler used to return single File from the blob storage
func getFileHandler(bucket *blob.Bucket) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		br, err := bucket.NewReader(context.Background(), r.URL.Path[1:], nil)
		if err != nil {
			if gcerrors.Code(err) == gcerrors.NotFound {
				w.WriteHeader(http.StatusNotFound)
				return
			} else {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
		defer br.Close()
		_, err = io.Copy(w, br)
		if err != nil {
			log.Println(err)
		}
	}
}

// HTTP-Handler used to accept image uploads and persist them in the blob storage
func postFileHandler(bucket *blob.Bucket) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		r.ParseMultipartForm(maxSize)
		// Get the form file with the key myFile and header information
		file, header, err := r.FormFile("myFile")
		if err != nil {
			log.Println(err)
			return
		}
		defer file.Close()
		if header.Size > maxSize {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("File too big"))
		}
		err = writeFile(bucket, header.Filename, file)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte("Could not write file"))
			log.Println(err)
		}

		listHandler(bucket)(w, r)
	}
}

func writeFile(bucket *blob.Bucket, name string, file io.Reader) error {
	bw, err := bucket.NewWriter(context.Background(), name, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer bw.Close()
	_, err = io.Copy(bw, file)
	return err
}
