package main

import (
	"fmt"

	"gopkg.in/mgo.v2/bson"

	mgo "gopkg.in/mgo.v2"
)

type database interface {
	// LoadURL loads a URL that was saved previously.
	LoadURL(name string) (string, error)

	// SaveURL saves a URL keyed by a name to be loaded later.
	SaveURL(name string, url string) error
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

	dialed *mgo.Session
}

func (d *mongoDatabase) session() (*mgo.Session, error) {
	if d.dialed == nil {
		var err error
		d.dialed, err = mgo.Dial(d.URL)
		return d.dialed, err
	}
	return d.dialed.Copy(), nil
}

func (d *mongoDatabase) collection() (*mgo.Collection, error) {
	s, err := d.session()
	if err != nil {
		return nil, err
	}
	return s.DB("").C("shortURL"), nil
}

func (d *mongoDatabase) LoadURL(name string) (string, error) {
	c, err := d.collection()
	if err != nil {
		return "", err
	}
	var r bson.D
	c.FindId(name).One(&r)
	if len(r) == 0 {
		return "", NotFoundError{name}
	}
	url := r.Map()["url"]
	if s, ok := url.(string); ok {
		return s, nil
	}
	return "", fmt.Errorf("Name is used but with a weird url object: %#v", url)
}

func (d *mongoDatabase) SaveURL(name string, url string) error {
	c, err := d.collection()
	if err != nil {
		return err
	}
	c.Insert(bson.D{{"_id", name}, {"url", url}})
	return nil
}
