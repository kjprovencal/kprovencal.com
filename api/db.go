package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
)

var (
	ErrEventExists  = errors.New("event already exists")
	ErrRSVPNotFound = errors.New("rsvp not found")
)

func openDB(path string) (*badger.DB, error) {
	opts := badger.DefaultOptions(path).
		WithLogger(nil).
		WithNumVersionsToKeep(1)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	return db, nil
}

type Event struct {
	Slug      string    `json:"slug"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	Published bool      `json:"published"`
}

type ContactRow struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Message   string    `json:"message"`
}

type RSVPRow struct {
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	GuestCount int       `json:"guest_count"`
	Meals      []string  `json:"meals"`
	Notes      string    `json:"notes"`
}

type RsvpAbandonRow struct {
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	GuestCount int       `json:"guest_count"` // -1 when party size was not chosen
	Meals      []string  `json:"meals"`
	Notes      string    `json:"notes"`
	Reason     string    `json:"reason"`
}

type ClientErrorRow struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Kind      string    `json:"kind"`
	Message   string    `json:"message"`
	Page      string    `json:"page"`
	Source    string    `json:"source,omitempty"`
	Line      int       `json:"line,omitempty"`
	Column    int       `json:"column,omitempty"`
	Stack     string    `json:"stack,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
}

func eventKey(slug string) []byte {
	return []byte("event/" + strings.ToLower(strings.TrimSpace(slug)))
}

func randomID(nbytes int) (string, error) {
	b := make([]byte, nbytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func tsKeyPrefix(prefix string, t time.Time) string {
	// Left-pad so lexicographic key order matches timestamp order.
	return fmt.Sprintf("%s/%019d/", prefix, t.UTC().UnixNano())
}

func contactKey(t time.Time, id string) string {
	return tsKeyPrefix("contact", t) + id
}

func rsvpKey(t time.Time, id string) string {
	return tsKeyPrefix("rsvp", t) + id
}

func clientErrorKey(t time.Time, id string) string {
	return tsKeyPrefix("client-error", t) + id
}

func rsvpAbandonKey(t time.Time, id string) string {
	return tsKeyPrefix("rsvp-abandon", t) + id
}

type eventStored struct {
	Slug      string    `json:"slug"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	Published bool      `json:"published"`
}

type contactStored struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Message   string    `json:"message"`
}

type rsvpStored struct {
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	GuestCount int       `json:"guest_count"`
	Meals      []string  `json:"meals"`
	Notes      string    `json:"notes"`
}

type rsvpAbandonStored struct {
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	GuestCount int       `json:"guest_count"`
	Meals      []string  `json:"meals"`
	Notes      string    `json:"notes"`
	Reason     string    `json:"reason"`
}

type clientErrorStored struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Kind      string    `json:"kind"`
	Message   string    `json:"message"`
	Page      string    `json:"page"`
	Source    string    `json:"source,omitempty"`
	Line      int       `json:"line,omitempty"`
	Column    int       `json:"column,omitempty"`
	Stack     string    `json:"stack,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
}

func insertEvent(db *badger.DB, slug, title string, published bool) error {
	key := eventKey(slug)
	createdAt := time.Now().UTC()
	st := eventStored{
		Slug:      strings.ToLower(slug),
		Title:     title,
		CreatedAt: createdAt,
		Published: published,
	}
	blob, err := json.Marshal(st)
	if err != nil {
		return err
	}

	return db.Update(func(tx *badger.Txn) error {
		_, err := tx.Get(key)
		if err == nil {
			return ErrEventExists
		}
		if !errors.Is(err, badger.ErrKeyNotFound) {
			return err
		}
		return tx.Set(key, blob)
	})
}

func insertContact(db *badger.DB, name, email, message string) error {
	now := time.Now().UTC()
	suffix, err := randomID(8)
	if err != nil {
		suffix = fmt.Sprintf("%d", now.UnixNano())
	}
	key := []byte(contactKey(now, suffix))

	st := contactStored{
		ID:        suffix,
		CreatedAt: now,
		Name:      name,
		Email:     email,
		Message:   message,
	}
	blob, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return db.Update(func(tx *badger.Txn) error {
		return tx.Set(key, blob)
	})
}

func insertWeddingRSVP(
	db *badger.DB,
	name, email string,
	guestCount int,
	meals []string,
	notes string,
) error {
	now := time.Now().UTC()
	suffix, err := randomID(8)
	if err != nil {
		suffix = fmt.Sprintf("%d", now.UnixNano())
	}
	key := []byte(rsvpKey(now, suffix))

	st := rsvpStored{
		ID:         suffix,
		CreatedAt:  now,
		Name:       name,
		Email:      email,
		GuestCount: guestCount,
		Meals:      meals,
		Notes:      notes,
	}
	blob, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return db.Update(func(tx *badger.Txn) error {
		return tx.Set(key, blob)
	})
}

func findRSVPInTxn(tx *badger.Txn, id string) (key []byte, st rsvpStored, err error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, rsvpStored{}, ErrRSVPNotFound
	}
	prefix := []byte("rsvp/")
	itOpts := badger.DefaultIteratorOptions
	itOpts.Prefix = prefix
	itOpts.PrefetchValues = true

	iter := tx.NewIterator(itOpts)
	defer iter.Close()

	for iter.Rewind(); iter.Valid(); iter.Next() {
		item := iter.Item()
		var cur rsvpStored
		if err := item.Value(func(val []byte) error {
			return json.Unmarshal(val, &cur)
		}); err != nil {
			return nil, rsvpStored{}, err
		}
		if cur.ID == id {
			return append([]byte(nil), item.Key()...), cur, nil
		}
	}
	return nil, rsvpStored{}, ErrRSVPNotFound
}

func updateWeddingRSVP(
	db *badger.DB,
	id, name, email string,
	guestCount int,
	meals []string,
	notes string,
) error {
	return db.Update(func(tx *badger.Txn) error {
		key, st, err := findRSVPInTxn(tx, id)
		if err != nil {
			return err
		}
		st.Name = name
		st.Email = email
		st.GuestCount = guestCount
		st.Meals = meals
		st.Notes = notes
		blob, err := json.Marshal(st)
		if err != nil {
			return err
		}
		return tx.Set(key, blob)
	})
}

func deleteWeddingRSVP(db *badger.DB, id string) error {
	return db.Update(func(tx *badger.Txn) error {
		key, _, err := findRSVPInTxn(tx, id)
		if err != nil {
			return err
		}
		return tx.Delete(key)
	})
}

func listPublishedEvents(db *badger.DB) ([]Event, error) {
	evs, err := listAllEvents(db)
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0)
	for _, ev := range evs {
		if ev.Published {
			out = append(out, ev)
		}
	}
	return out, nil
}

func listAllEvents(db *badger.DB) ([]Event, error) {
	evs := make([]Event, 0)
	prefix := []byte("event/")
	itOpts := badger.DefaultIteratorOptions
	itOpts.Prefix = prefix
	itOpts.PrefetchValues = true
	itOpts.Reverse = false

	txn := db.NewTransaction(false)
	defer txn.Discard()

	iter := txn.NewIterator(itOpts)
	defer iter.Close()

	for iter.Rewind(); iter.Valid(); iter.Next() {
		item := iter.Item()
		var st eventStored
		if err := item.Value(func(val []byte) error {
			return json.Unmarshal(val, &st)
		}); err == nil {
			evs = append(evs, Event{
				Slug:      st.Slug,
				Title:     st.Title,
				CreatedAt: st.CreatedAt,
				Published: st.Published,
			})
		}
	}

	sort.Slice(evs, func(i, j int) bool {
		return evs[i].CreatedAt.After(evs[j].CreatedAt)
	})
	return evs, nil
}

func listContacts(db *badger.DB, limit int) ([]ContactRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	prefix := []byte("contact/")
	itOpts := badger.DefaultIteratorOptions
	itOpts.Prefix = prefix
	// Badger v4: Reverse+Prefix iteration returns no keys (prefix check skipped in reverse).
	itOpts.Reverse = false
	itOpts.PrefetchValues = true

	txn := db.NewTransaction(false)
	defer txn.Discard()

	iter := txn.NewIterator(itOpts)
	defer iter.Close()

	out := make([]ContactRow, 0)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		item := iter.Item()
		var st contactStored
		if err := item.Value(func(val []byte) error {
			return json.Unmarshal(val, &st)
		}); err != nil {
			return nil, err
		}
		out = append(out, ContactRow{
			ID:        st.ID,
			CreatedAt: st.CreatedAt,
			Name:      st.Name,
			Email:     st.Email,
			Message:   st.Message,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func listRSVPs(db *badger.DB, limit int) ([]RSVPRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	prefix := []byte("rsvp/")
	itOpts := badger.DefaultIteratorOptions
	itOpts.Prefix = prefix
	itOpts.Reverse = false
	itOpts.PrefetchValues = true

	txn := db.NewTransaction(false)
	defer txn.Discard()

	iter := txn.NewIterator(itOpts)
	defer iter.Close()

	out := make([]RSVPRow, 0)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		item := iter.Item()
		var st rsvpStored
		if err := item.Value(func(val []byte) error {
			return json.Unmarshal(val, &st)
		}); err != nil {
			return nil, err
		}
		out = append(out, RSVPRow{
			ID:         st.ID,
			CreatedAt:  st.CreatedAt,
			Name:       st.Name,
			Email:      st.Email,
			GuestCount: st.GuestCount,
			Meals:      st.Meals,
			Notes:      st.Notes,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func insertClientError(
	db *badger.DB,
	kind, message, page, source string,
	line, column int,
	stack, userAgent string,
) error {
	now := time.Now().UTC()
	suffix, err := randomID(8)
	if err != nil {
		suffix = fmt.Sprintf("%d", now.UnixNano())
	}
	key := []byte(clientErrorKey(now, suffix))

	st := clientErrorStored{
		ID:        suffix,
		CreatedAt: now,
		Kind:      kind,
		Message:   message,
		Page:      page,
		Source:    source,
		Line:      line,
		Column:    column,
		Stack:     stack,
		UserAgent: userAgent,
	}
	blob, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return db.Update(func(tx *badger.Txn) error {
		return tx.Set(key, blob)
	})
}

func listClientErrors(db *badger.DB, limit int) ([]ClientErrorRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	prefix := []byte("client-error/")
	itOpts := badger.DefaultIteratorOptions
	itOpts.Prefix = prefix
	itOpts.Reverse = false
	itOpts.PrefetchValues = true

	txn := db.NewTransaction(false)
	defer txn.Discard()

	iter := txn.NewIterator(itOpts)
	defer iter.Close()

	out := make([]ClientErrorRow, 0)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		item := iter.Item()
		var st clientErrorStored
		if err := item.Value(func(val []byte) error {
			return json.Unmarshal(val, &st)
		}); err != nil {
			return nil, err
		}
		out = append(out, ClientErrorRow{
			ID:        st.ID,
			CreatedAt: st.CreatedAt,
			Kind:      st.Kind,
			Message:   st.Message,
			Page:      st.Page,
			Source:    st.Source,
			Line:      st.Line,
			Column:    st.Column,
			Stack:     st.Stack,
			UserAgent: st.UserAgent,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func insertRsvpAbandon(
	db *badger.DB,
	name, email string,
	guestCount int,
	meals []string,
	notes, reason string,
) error {
	now := time.Now().UTC()
	suffix, err := randomID(8)
	if err != nil {
		suffix = fmt.Sprintf("%d", now.UnixNano())
	}
	key := []byte(rsvpAbandonKey(now, suffix))

	st := rsvpAbandonStored{
		ID:         suffix,
		CreatedAt:  now,
		Name:       name,
		Email:      email,
		GuestCount: guestCount,
		Meals:      meals,
		Notes:      notes,
		Reason:     reason,
	}
	blob, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return db.Update(func(tx *badger.Txn) error {
		return tx.Set(key, blob)
	})
}

func listRsvpAbandons(db *badger.DB, limit int) ([]RsvpAbandonRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	prefix := []byte("rsvp-abandon/")
	itOpts := badger.DefaultIteratorOptions
	itOpts.Prefix = prefix
	itOpts.Reverse = false
	itOpts.PrefetchValues = true

	txn := db.NewTransaction(false)
	defer txn.Discard()

	iter := txn.NewIterator(itOpts)
	defer iter.Close()

	out := make([]RsvpAbandonRow, 0)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		item := iter.Item()
		var st rsvpAbandonStored
		if err := item.Value(func(val []byte) error {
			return json.Unmarshal(val, &st)
		}); err != nil {
			return nil, err
		}
		out = append(out, RsvpAbandonRow{
			ID:         st.ID,
			CreatedAt:  st.CreatedAt,
			Name:       st.Name,
			Email:      st.Email,
			GuestCount: st.GuestCount,
			Meals:      st.Meals,
			Notes:      st.Notes,
			Reason:     st.Reason,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
