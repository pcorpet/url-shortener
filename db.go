package main

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// A namedURL is a URL associated with its short name.
type namedURL struct {
	// Name is the short name of the URL. It cannot contain /?# nor be "_".
	Name string `json:"name" bson:"_id"`
	// URL is the long URL that is shortened. It must be a valid URL.
	URL string `json:"url" bson:"url"`
	// Email of users that are allowed to modify this association.
	Owners []string `json:"owners" bson: "owners"`
}

type database interface {
	// ListURLs list all URLs that were saved or at least the 5000 first ones.
	ListURLs(ctx context.Context) ([]namedURL, error)

	// LoadURL loads a URL that was saved previously.
	LoadURL(ctx context.Context, name string) (string, error)

	// SaveURL saves a URL keyed by a name to be loaded later.
	SaveURL(ctx context.Context, name string, url string, owners []string) error

	// DeleteURL deletes a URL keyed by a name only if it's owned by the given
	// user. If user is empty, doesn't check for ownership.
	DeleteURL(ctx context.Context, name string, user string) error
}

// A NotFoundError is triggered if a name does not resolve to an URL in the
// database.
type NotFoundError struct {
	Name string
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("no URL found with name %q", e.Name)
}

type mongoDatabase struct {
	// URL is the URL to connect to the MongoDB:
	//   [mongodb://][user:pass@]host1[:port1][,host2[:port2],...][/database][?options]
	URL string

	// Name of the DB to use.
	DBName string

	// Name of the collection to use.
	CollectionName string

	connected *mongo.Client
}

func (d *mongoDatabase) client(ctx context.Context) (*mongo.Client, error) {
	if d.connected == nil {
		var err error
		d.connected, err = mongo.Connect(ctx, options.Client().ApplyURI(d.URL))
		return d.connected, err
	}
	return d.connected, nil
}

func (d *mongoDatabase) collection(ctx context.Context) (*mongo.Collection, error) {
	c, err := d.client(ctx)
	if err != nil {
		return nil, err
	}
	return c.Database(d.DBName).Collection(d.CollectionName), nil
}

func (d *mongoDatabase) ListURLs(ctx context.Context) (urls []namedURL, err error) {
	c, err := d.collection(ctx)
	if err != nil {
		return nil, err
	}
	iter, err := c.Find(ctx, bson.D{}, options.Find().SetLimit(5000).SetSort(bson.D{{"_id", 1}}))
	if err != nil {
		return nil, err
	}
	for iter.Next(ctx) {
		var result namedURL
		if err := iter.Decode(&result); err != nil {
			// Just skip it if you cannot retrieve the info.
			continue
		}
		urls = append(urls, result)
	}
	return urls, iter.Close(ctx)
}

func (d *mongoDatabase) LoadURL(ctx context.Context, name string) (string, error) {
	c, err := d.collection(ctx)
	if err != nil {
		return "", err
	}
	var result namedURL
	err = c.FindOne(ctx, bson.D{{"_id", name}}).Decode(&result)
	if err == mongo.ErrNoDocuments {
		return "", NotFoundError{name}
	}
	if err != nil {
		return "", fmt.Errorf("Could not decode URL object for %v: %w", name, err)
	}
	return result.URL, nil
}

func (d *mongoDatabase) SaveURL(ctx context.Context, name string, url string, owners []string) error {
	c, err := d.collection(ctx)
	if err != nil {
		return err
	}
	_, err = c.InsertOne(ctx, bson.D{{"_id", name}, {"url", url}, {"owners", owners}})
	return err
}

func (d *mongoDatabase) DeleteURL(ctx context.Context, name string, user string) error {
	c, err := d.collection(ctx)
	if err != nil {
		return err
	}
	filter := bson.D{{"_id", name}}
	if user != "" {
		filter = append(filter, bson.E{"owners", user})
	}
	r, err := c.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if r.DeletedCount != 1 {
		return fmt.Errorf("The short URL does not exist: %#v", name)
	}
	return nil
}
