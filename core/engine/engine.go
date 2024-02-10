package engine

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"regexp"
	"sync"
)

type path = string // The key
type ContentType = string
type NabiaRecord struct {
	RawData     []byte
	ContentType ContentType // "Content-Type" https://datatracker.ietf.org/doc/html/rfc2616/#section-14.17
}

func NewNabiaString(s string) *NabiaRecord {
	return &NabiaRecord{RawData: []byte(s), ContentType: "text/plain; charset=UTF-8"}
}

func NewNabiaRecord(data []byte, ct ContentType) *NabiaRecord {
	return &NabiaRecord{RawData: data, ContentType: ct}
}

type NabiaDB struct {
	Records  sync.Map
	location string
}

// checkOrCreateDB checks if the file exists, and if it doesn't, it creates it.
// The first boolean indicates whether the file already existed, and the second
// boolean indicates whether an error occurred.
func checkOrCreateFile(location string) (bool, error) {
	// Attempt to open the file in read-only mode to check if it exists.
	if _, err := os.Stat(location); err == nil {
		// The file exists.
		return true, nil
	} else if os.IsNotExist(err) {
		// The file does not exist, attempt to create it.
		file, err := os.Create(location)
		if err != nil {
			// Failed to create the file, return the error.
			return false, err
		}
		// Successfully created the file, close it.
		defer file.Close()
		return false, nil
	} else {
		// Some other error occurred when checking the file, return it.
		return false, err
	}
}

func NewNabiaDB(location string) (*NabiaDB, error) {
	exists, err := checkOrCreateFile(location)
	if err != nil {
		return nil, err
	}
	ndb := &NabiaDB{Records: sync.Map{}, location: location}
	if exists {
		ndb.loadFromFile(location)
	} else {
		if err := ndb.saveToFile(location); err != nil {
			log.Fatalf("Failed to save to file: %s", err)
		}
	}
	return ndb, nil
}

// Below are the DB primitives.

// Exists checks if the key name provided exists in the Nabia map. It locks
// to read and unlocks immediately after.
func (ns *NabiaDB) Exists(key string) bool {
	_, ok := ns.Records.Load(key)
	return ok
}

// Read takes a key name and attempts to pull the data from the Nabia DB map.
// Returns a NabiaRecord (if found) and an error (if not found). Callers must
// always check the error returned in the second parameter, as the result cannot
// be used if the "error" field is not nil. This function is safe to call even
// with empty data, because the method applies a mutex.
func (ns *NabiaDB) Read(key string) (NabiaRecord, error) {
	if value, ok := ns.Records.Load(key); ok {
		record := value.(*NabiaRecord)
		return *record, nil
	} else {
		return NabiaRecord{}, fmt.Errorf("key '%s' doesn't exist", key)
	}
}

// Write takes the key and a value of NabiaRecord datatype and places it on the
// database, potentially overwriting whatever was there before, because Write
// has no data safety features preventing the overwriting of data.
func (ns *NabiaDB) Write(key string, value NabiaRecord) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	if value.RawData == nil {
		return fmt.Errorf("value cannot be nil")
	}
	if value.ContentType == "" {
		return fmt.Errorf("Content-Type cannot be empty")
	}
	pattern := `^[a-zA-Z0-9]+/[a-zA-Z0-9]+`
	r := regexp.MustCompile(pattern)
	if !r.MatchString(value.ContentType) {
		return fmt.Errorf("Content-Type is not valid")
	} else {
		ns.Records.Store(key, &value)
	}
	return nil
}

// Destroy takes a key and removes it from the map. This method doesn't have
// existence-checking logic. It is safe to use on empty data, it simply doesn't
// do anything if the record doesn't exist.
func (ns *NabiaDB) Destroy(key string) {
	ns.Records.Delete(key)
}

func (ns *NabiaDB) Stop() {
	return
}

func (ns *NabiaDB) saveToFile(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Use a buffered writer for better performance
	writer := bufio.NewWriter(file)
	defer writer.Flush()

	encoder := gob.NewEncoder(writer)

	// Convert sync.Map to a regular map for encoding
	data := make(map[string]*NabiaRecord)
	ns.Records.Range(func(key, value interface{}) bool {
		data[key.(string)] = value.(*NabiaRecord)
		return true
	})

	// Encode the map
	return encoder.Encode(data)
}

func (ns *NabiaDB) loadFromFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Use a buffered reader for better performance
	reader := bufio.NewReader(file)
	decoder := gob.NewDecoder(reader)

	// Decode the map
	data := make(map[string]*NabiaRecord)
	if err := decoder.Decode(&data); err != nil {
		return err
	}

	// Convert the regular map back to a sync.Map
	for key, value := range data {
		ns.Records.Store(key, value)
	}

	return nil
}
