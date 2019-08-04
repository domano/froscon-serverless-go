// Copyright 2018 The Go Cloud Development Kit Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// A simple "hello world" application using server.Server, to be run on
// Google App Engine (GAE).
package main

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/gcsblob"
	_ "gocloud.dev/blob/s3blob"
	"gocloud.dev/server"
	"io"
	"log"
	"net/http"
	"os"
)

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", handle)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := server.New(r, nil)
	bucket, err := blob.OpenBucket(context.Background(), "s3://froscon-serverless")
	if err != nil {
		log.Fatal(err)
	}
	list := bucket.List(nil)
	for {
		item, err := list.Next(context.Background())
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		log.Println(item.Key)
	}
	log.Printf("Listening on port %s", port)
	log.Fatal(srv.ListenAndServe(fmt.Sprintf(":%s", port)))
}

func handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	fmt.Fprint(w, "Hello world!")
}
