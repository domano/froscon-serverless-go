package main

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/gcsblob"
	_ "gocloud.dev/blob/memblob"
	_ "gocloud.dev/blob/s3blob"
	"gocloud.dev/server"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
)

var tpl *template.Template

func main() {
	bucketPath := os.Getenv("BUCKET_URI")
	if bucketPath == "" {
		log.Fatalln("No Bucket URI set.")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	tpl, _ = template.New("index").Parse(html)

	bucket, err := blob.OpenBucket(context.Background(), bucketPath)
	if err != nil {
		log.Fatal(err)
	}
	r := mux.NewRouter()
	r.Methods(http.MethodGet).Path("/").HandlerFunc(listHandler(bucket))
	r.Methods(http.MethodGet).Path("/{image}").HandlerFunc(getFileHandler(bucket))
	r.Methods(http.MethodPost).Path("/").HandlerFunc(postFileHandler(bucket))
	srv := server.New(r, nil)

	log.Printf("Listening on port %s", port)
	log.Fatal(srv.ListenAndServe(fmt.Sprintf(":%s", port)))
}

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

func getFileHandler(bucket *blob.Bucket) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.URL.Path)
		br := readFile(bucket, r.URL.Path[1:])
		defer br.Close()
		_, err := io.Copy(w, br)
		if err != nil {
			log.Println(err)
		}
	}
}

func postFileHandler(bucket *blob.Bucket) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("File Upload Endpoint Hit")

		// Parse our multipart form, 10 << 20 specifies a maximum
		// upload of 10 MB files.
		r.ParseMultipartForm(10 << 20)
		// FormFile returns the first file for the given key `myFile`
		// it also returns the FileHeader so we can get the Filename,
		// the Header and the size of the file
		file, header, err := r.FormFile("myFile")
		if err != nil {
			log.Println(err)
			return
		}
		defer file.Close()
		if header.Size > 10E7 {
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

func readFile(bucket *blob.Bucket, name string) io.ReadCloser{
	br, err := bucket.NewReader(context.Background(), name, nil)
	if err != nil {
		log.Fatal(err)
	}
	return br
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
