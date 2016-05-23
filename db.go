package main

import (
	"fmt"

	"gopkg.in/mgo.v2/bson"

	mgo "gopkg.in/mgo.v2"
)

// A namedURL is a URL associated with its short name.
type namedURL struct {
	// Name is the short name of the URL. It cannot contain /?# nor be "_".
	Name string `json:"name"`
	// URL is the long URL that is shortened. It must be a valid URL.
	URL string `json:"url"`
	// Email of users that are allowed to modify this association.
	Owners []string `json:"owners"`
}

type database interface {
	// ListURLs list all URLs that were saved or at least the 100 first ones.
	ListURLs() ([]namedURL, error)

	// LoadURL loads a URL that was saved previously.
	LoadURL(name string) (string, error)

	// SaveURL saves a URL keyed by a name to be loaded later.
	SaveURL(name string, url string, owners []string) error

	// DeleteURL deletes a URL keyed by a name only if it's owned by the given
	// user.
	DeleteURL(name string, user string) error
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

func (d *mongoDatabase) ListURLs() (urls []namedURL, err error) {
	c, err := d.collection()
	if err != nil {
		return nil, err
	}
	iter := c.Find(nil).Limit(100).Sort("_id").Iter()
	var result bson.D
	for iter.Next(&result) {
		m := result.Map()
		var url namedURL
		var ok bool
		if url.Name, ok = m["_id"].(string); !ok {
			// Just skip it if you cannot retrieve the info.
			continue
		}
		if url.URL, ok = m["url"].(string); !ok {
			// Just skip it if you cannot retrieve the info.
			continue
		}
		url.Owners = []string{}
		if owners, ok := m["owners"].([]interface{}); ok && len(owners) > 0 {
			for _, owner := range owners {
				if ownerString, ok := owner.(string); ok {
					url.Owners = append(url.Owners, ownerString)
				}
			}
		}
		urls = append(urls, url)
	}
	return urls, iter.Close()
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

func (d *mongoDatabase) SaveURL(name string, url string, owners []string) error {
	c, err := d.collection()
	if err != nil {
		return err
	}
	c.Insert(bson.D{{"_id", name}, {"url", url}, {"owners", owners}})
	return nil
}

func (d *mongoDatabase) DeleteURL(name string, user string) error {
	c, err := d.collection()
	if err != nil {
		return err
	}
	return c.Remove(bson.D{{"_id", name}, {"owners", user}})
}
