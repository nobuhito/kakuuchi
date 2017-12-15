package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/search"
)

type User struct {
	DisplayName string `json:"displayName"`
	PhotoURL    string `json:"photoURL"`
	UID         string `json:"uid"`
}

type IndexData struct {
	Title       string
	Description string
	Message     search.HTML
	LastUpdated time.Time
}

const firestoreAccountFile = "firebase.json"
const firestoreProjectID = "kakuuchi-app"

const storageAccountFile = "appengine.json"
const storageProjectID = "fine-craft-188911.appspot.com"

const indexName = "kakuuchi"
const sandboxName = "Sandbox"

func init() {
	http.HandleFunc("/cron/createIndex/", createIndex)
	http.HandleFunc("/cron/initIndex/", initIndex)
	http.HandleFunc("/search/", searchIndex)
	http.HandleFunc("/cron/createArchive/", createArchive)
}

func writeLogIfError(ctx context.Context, err error) {
	if err != nil {
		log.Errorf(ctx, "Err: %s", err.Error())
	}
}

func getNewFirestoreClient(ctx context.Context) (*firestore.Client, error) {
	return firestore.NewClient(ctx, firestoreProjectID, option.WithServiceAccountFile(firestoreAccountFile))
}

func getNewStorageClient(ctx context.Context) (*storage.Client, error) {
	return storage.NewClient(ctx, option.WithServiceAccountFile(storageAccountFile))
}

func returnJSON(w http.ResponseWriter, jsonData []byte) {
	w.Header().Set("Content-type", "application/json")
	w.Write(jsonData)
}

func searchIndex(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	index, err := search.Open(indexName)
	writeLogIfError(ctx, err)

	query := r.FormValue("word")

	var counters []string
	for c := index.Search(ctx, query, nil); ; {
		var d IndexData
		id, err := c.Next(&d)
		if err == search.Done {
			break
		}
		writeLogIfError(ctx, err)

		counters = append(counters, id)
	}

	jsonData, err := json.Marshal(counters)
	writeLogIfError(ctx, err)
	returnJSON(w, jsonData)
}

func createArchive(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	client, err := getNewFirestoreClient(ctx)
	writeLogIfError(ctx, err)
	defer client.Close()

	options := CounterOptions{
		"isArchived": true,
		"isIndexed":  true,
		"link":       false,
	}
	counters, err := CountersFromFirestore(ctx, client, options)
	writeLogIfError(ctx, err)

	for _, counter := range counters {
		if counter.Name == sandboxName {
			continue
		}

		err = counter.AppendMessages(ctx)
		writeLogIfError(ctx, err)

		link, err := counter.SaveToStorage(ctx)
		writeLogIfError(ctx, err)

		_, err = counter.ref.Set(ctx, map[string]interface{}{
			"link": link,
		}, firestore.MergeAll)
		writeLogIfError(ctx, err)

		err = counter.DeleteAllMessages(ctx)
		writeLogIfError(ctx, err)
	}
}

func initIndex(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	is, _ := search.Open(indexName)
	var d IndexData
	d.Title = "test"
	d.Description = "test"
	_, _ = is.Put(ctx, "", &d)
}

func createIndex(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	client, err := getNewFirestoreClient(ctx)
	writeLogIfError(ctx, err)
	defer client.Close()

	options := CounterOptions{
		"isIndexed": false,
		"link":      false,
	}
	counters, err := CountersFromFirestore(ctx, client, options)
	writeLogIfError(ctx, err)

	indexSearch, err := search.Open(indexName)
	for _, counter := range counters {
		if counter.Name == sandboxName {
			continue
		}

		err := counter.AppendMessages(ctx)
		writeLogIfError(ctx, err)

		var d IndexData
		d.Title = counter.Name
		d.Description = counter.Description
		d.LastUpdated = counter.LastUpdated
		d.Message = search.HTML(strings.Join(counter.Messages.Values(), "<br>"))

		_, err = indexSearch.Put(ctx, counter.ref.ID, &d)

		counter.ref.Set(ctx, map[string]interface{}{
			"isIndexed": true,
		}, firestore.MergeAll)
	}

}
