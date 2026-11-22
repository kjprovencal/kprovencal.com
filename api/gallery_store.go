package main

import (
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/dgraph-io/badger/v4"
)

var (
	ErrAlbumNotFound = errors.New("album not found")
	ErrPhotoNotFound = errors.New("photo not found")
)

type albumStored struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Token     string    `json:"token"`
	PinHash   string    `json:"pin_hash"`
	CreatedAt time.Time `json:"created_at"`
}

type photoStored struct {
	ID          string    `json:"id"`
	AlbumID     string    `json:"album_id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	OriginalKey string    `json:"original_key"`
	WebKey      string    `json:"web_key"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

func albumKey(id string) []byte {
	return []byte("album/" + id)
}

func albumTokenKey(token string) []byte {
	return []byte("album-token/" + token)
}

func photoKey(albumID, photoID string) []byte {
	return []byte("photo/" + albumID + "/" + photoID)
}

func createAlbum(db *badger.DB, title, pinHash string) (albumStored, error) {
	id, err := randomID(16)
	if err != nil {
		return albumStored{}, err
	}
	token, err := randomID(16)
	if err != nil {
		return albumStored{}, err
	}
	st := albumStored{
		ID:        id,
		Title:     title,
		Token:     token,
		PinHash:   pinHash,
		CreatedAt: time.Now().UTC(),
	}
	blob, err := json.Marshal(st)
	if err != nil {
		return albumStored{}, err
	}
	err = db.Update(func(tx *badger.Txn) error {
		if err := tx.Set(albumKey(id), blob); err != nil {
			return err
		}
		return tx.Set(albumTokenKey(token), []byte(id))
	})
	if err != nil {
		return albumStored{}, err
	}
	return st, nil
}

func getAlbum(db *badger.DB, id string) (albumStored, error) {
	var st albumStored
	err := db.View(func(tx *badger.Txn) error {
		item, err := tx.Get(albumKey(id))
		if errors.Is(err, badger.ErrKeyNotFound) {
			return ErrAlbumNotFound
		}
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &st)
		})
	})
	return st, err
}

func getAlbumByToken(db *badger.DB, token string) (albumStored, error) {
	var id string
	err := db.View(func(tx *badger.Txn) error {
		item, err := tx.Get(albumTokenKey(token))
		if errors.Is(err, badger.ErrKeyNotFound) {
			return ErrAlbumNotFound
		}
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			id = string(val)
			return nil
		})
	})
	if err != nil {
		return albumStored{}, err
	}
	return getAlbum(db, id)
}

func listAlbums(db *badger.DB) ([]albumStored, error) {
	prefix := []byte("album/")
	itOpts := badger.DefaultIteratorOptions
	itOpts.Prefix = prefix
	itOpts.PrefetchValues = true

	txn := db.NewTransaction(false)
	defer txn.Discard()

	iter := txn.NewIterator(itOpts)
	defer iter.Close()

	out := make([]albumStored, 0)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		var st albumStored
		if err := iter.Item().Value(func(val []byte) error {
			return json.Unmarshal(val, &st)
		}); err != nil {
			return nil, err
		}
		if st.ID == "" {
			continue
		}
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func updateAlbum(db *badger.DB, id, title, pinHash string, setTitle, setPin bool) (albumStored, error) {
	var updated albumStored
	err := db.Update(func(tx *badger.Txn) error {
		item, err := tx.Get(albumKey(id))
		if errors.Is(err, badger.ErrKeyNotFound) {
			return ErrAlbumNotFound
		}
		if err != nil {
			return err
		}
		var st albumStored
		if err := item.Value(func(val []byte) error {
			return json.Unmarshal(val, &st)
		}); err != nil {
			return err
		}
		if setTitle {
			st.Title = title
		}
		if setPin {
			st.PinHash = pinHash
		}
		blob, err := json.Marshal(st)
		if err != nil {
			return err
		}
		updated = st
		return tx.Set(albumKey(id), blob)
	})
	return updated, err
}

func countPhotos(db *badger.DB, albumID string) (int, error) {
	photos, err := listPhotos(db, albumID)
	if err != nil {
		return 0, err
	}
	return len(photos), nil
}

func listPhotos(db *badger.DB, albumID string) ([]photoStored, error) {
	prefix := []byte("photo/" + albumID + "/")
	itOpts := badger.DefaultIteratorOptions
	itOpts.Prefix = prefix
	itOpts.PrefetchValues = true

	txn := db.NewTransaction(false)
	defer txn.Discard()

	iter := txn.NewIterator(itOpts)
	defer iter.Close()

	out := make([]photoStored, 0)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		var st photoStored
		if err := iter.Item().Value(func(val []byte) error {
			return json.Unmarshal(val, &st)
		}); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func getPhoto(db *badger.DB, albumID, photoID string) (photoStored, error) {
	var st photoStored
	err := db.View(func(tx *badger.Txn) error {
		item, err := tx.Get(photoKey(albumID, photoID))
		if errors.Is(err, badger.ErrKeyNotFound) {
			return ErrPhotoNotFound
		}
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &st)
		})
	})
	return st, err
}

func insertPhotos(db *badger.DB, photos []photoStored) error {
	return db.Update(func(tx *badger.Txn) error {
		for _, photo := range photos {
			blob, err := json.Marshal(photo)
			if err != nil {
				return err
			}
			if err := tx.Set(photoKey(photo.AlbumID, photo.ID), blob); err != nil {
				return err
			}
		}
		return nil
	})
}

func markPhotoReady(db *badger.DB, albumID, photoID string, width, height int) (photoStored, error) {
	var updated photoStored
	err := db.Update(func(tx *badger.Txn) error {
		item, err := tx.Get(photoKey(albumID, photoID))
		if errors.Is(err, badger.ErrKeyNotFound) {
			return ErrPhotoNotFound
		}
		if err != nil {
			return err
		}
		var st photoStored
		if err := item.Value(func(val []byte) error {
			return json.Unmarshal(val, &st)
		}); err != nil {
			return err
		}
		st.Status = photoStatusReady
		st.Width = width
		st.Height = height
		blob, err := json.Marshal(st)
		if err != nil {
			return err
		}
		updated = st
		return tx.Set(photoKey(albumID, photoID), blob)
	})
	return updated, err
}

func deletePhotoRecord(db *badger.DB, albumID, photoID string) error {
	return db.Update(func(tx *badger.Txn) error {
		_, err := tx.Get(photoKey(albumID, photoID))
		if errors.Is(err, badger.ErrKeyNotFound) {
			return ErrPhotoNotFound
		}
		if err != nil {
			return err
		}
		return tx.Delete(photoKey(albumID, photoID))
	})
}

func deleteAlbumRecords(db *badger.DB, album albumStored, photos []photoStored) error {
	return db.Update(func(tx *badger.Txn) error {
		for _, photo := range photos {
			if err := tx.Delete(photoKey(album.ID, photo.ID)); err != nil {
				return err
			}
		}
		if err := tx.Delete(albumTokenKey(album.Token)); err != nil {
			return err
		}
		return tx.Delete(albumKey(album.ID))
	})
}
