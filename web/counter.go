package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
)

type Counter struct {
	Description string    `json:"description"`
	IsArchived  bool      `json:"isArchived"`
	IsIndexed   bool      `json:"isIndexed"`
	IsPrivate   bool      `json:"isPrivate"`
	LastUpdated time.Time `json:"lastUpdated"`
	Name        string    `json:"name"`
	Owner       User      `json:"owner"`
	Participant []string  `json:"participant"`
	Messages    Messages  `json:"messages"`
	ref         *firestore.DocumentRef
}

type CounterOptions map[string]bool

type Message struct {
	Value    string    `json:"value"`
	User     User      `json:"user"`
	Inserted time.Time `json:"inserted"`
}

type Messages []Message

func CountersFromFirestore(ctx context.Context, client *firestore.Client, options CounterOptions) ([]Counter, error) {
	var counters []Counter

	query := client.Collection("Counters").Query
	for key, value := range options {
		if key == "link" {
			if !value {
				query = query.Where(key, "==", "")
			}
		} else {
			query = query.Where(key, "==", value)
		}
	}

	docs, err := query.Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}

	for _, doc := range docs {
		var counter Counter
		err = doc.DataTo(&counter)
		if err != nil {
			return nil, err
		}

		counter.ref = doc.Ref
		counters = append(counters, counter)
	}

	return counters, nil
}

func (c *Counter) SaveToStorage(ctx context.Context) (string, error) {
	client, err := getNewStorageClient(ctx)
	if err != nil {
		return "", err
	}
	defer client.Close()

	filename := fmt.Sprintf("%s.json", c.ref.ID)

	w := client.Bucket(storageProjectID).Object(filename).NewWriter(ctx)
	w.ContentType = "application/json"
	w.ACL = []storage.ACLRule{{storage.AllUsers, storage.RoleReader}}

	data, err := json.Marshal(&c)
	if err != nil {
		return "", err
	}

	_, err = w.Write(data)
	if err != nil {
		return "", err
	}

	err = w.Close()
	if err != nil {
		return "", err
	}

	return w.Attrs().MediaLink, nil
}

func (c *Counter) DeleteAllMessages(ctx context.Context) error {
	docs, err := c.ref.Collection("messages").Documents(ctx).GetAll()
	if err != nil {
		return err
	}

	for _, doc := range docs {
		_, err = doc.Ref.Delete(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Counter) AppendMessages(ctx context.Context) error {
	docs, err := c.ref.Collection("messages").Documents(ctx).GetAll()
	if err != nil {
		return err
	}

	for _, doc := range docs {
		var message Message
		err = doc.DataTo(&message)
		if err != nil {
			return err
		}

		c.Messages = append(c.Messages, message)
	}

	return nil
}

func (messages Messages) Values() []string {
	var ret []string
	for _, message := range messages {
		ret = append(ret, message.Value)
	}
	return ret
}
