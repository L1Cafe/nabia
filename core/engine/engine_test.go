package engine

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestFileSavingAndLoading(t *testing.T) {
	location := "filesaving.db"
	exists, err := checkOrCreateFile(location)
	if err != nil {
		t.Fatalf("failed to check or create file: %s", err) // Unknown error
	} else {
		if exists {
			if err := os.Remove(location); err != nil {
				t.Fatalf("failed to remove test.db: %s", err) // Unknown error
			}
		}
	}
	os.Remove(location)
	nabiaDB, err := NewNabiaDB(location)
	if err != nil {
		t.Fatalf("failed to create NabiaDB: %s", err) // Unknown error
	}
	defer os.Remove(location)
	value_a, _ := NewNabiaRecord("Value_A")
	if err := nabiaDB.Write("A", value_a); err != nil { // Failure when writing a value
		t.Errorf("failed to write to NabiaDB: %s", err) // Unknown error
	}
	if err := nabiaDB.saveToFile(location); err != nil {
		t.Fatalf("failed to save NabiaDB to file: %s", err) // Unknown error
	}
	nabiaDB, err = loadFromFile(location)
	if err != nil {
		t.Fatalf("failed to load NabiaDB from file: %s", err) // Unknown error
	}
	exists, err = checkOrCreateFile(location)
	if err != nil {
		t.Fatalf("failed to check or create file: %s", err)
	} else {
		if !exists {
			t.Errorf("file should exist: %s", err)
		}
	}
	if err := os.Remove(location); err != nil { // Deleting DB from disk
		t.Fatalf("failed to remove test.db: %s", err)
	}
	_, err = loadFromFile(location)
	if !strings.Contains(err.Error(), "no such file or directory") { // Attempting to read a file that doesn't exist should never succeed
		t.Errorf("should not succeed when attempting to load a non-existant file: %s", err)
	}
	if err := nabiaDB.saveToFile(location); err != nil { // Attempting to save after deletion
		t.Fatalf("failed to save NabiaDB to file: %s", err)
	}
	nabiaDB, err = loadFromFile(location) // Attempting to load the database once again
	if err != nil {
		t.Fatalf("failed to load NabiaDB from file: %s", err) // Unknown error
	}
	nr, err := nabiaDB.Read("A") // Attempting to read the value saved earlier
	if err != nil {
		t.Fatalf("failed to read from NabiaDB: %s", err) // Unknown error
	} else {
		expectedData := []byte("Value_A")
		if !bytes.Equal(nr, expectedData) { //TODO fix this ???
			t.Errorf("failed to read the correct value from NabiaDB: %s", err)
		}
	}
	nr, err = nabiaDB.Read("B")
	if err == nil {
		t.Error("should not succeed when attempting to read a non-existent key")
	}
	if err := os.Remove(location); err != nil { // Final DB deletion
		t.Fatalf("failed to remove test.db: %s", err)
	}
}

func TestCRUD(t *testing.T) { // Create, Read, Update, Destroy

	var nabia_read NabiaRecord[string]
	var expected []byte
	expected_stats := dataActivity{reads: 0, writes: 0, size: 0}

	nabiaDB, err := NewNabiaDB("crud.db")
	if err != nil {
		t.Errorf("Failed to create NabiaDB: %s", err)
	}
	defer os.Remove("crud.db")

	if nabiaDB.Exists("A") {
		t.Error("Uninitialised database contains elements!")
	}
	atomic.AddInt64(&expected_stats.reads, 1)
	//CREATE
	s, err := NewNabiaRecord("Value_A")
	if err != nil {
		t.Errorf("error when creating a record")
	}
	nabiaDB.Write("A", *s)
	atomic.AddInt64(&expected_stats.reads, 1)
	atomic.AddInt64(&expected_stats.writes, 1)
	atomic.AddInt64(&expected_stats.size, 1)
	if !nabiaDB.Exists("A") {
		t.Error("Database is not writing items correctly!")
	}
	atomic.AddInt64(&expected_stats.reads, 1)
	//READ
	nabia_read, err = nabiaDB.Read("A")
	atomic.AddInt64(&expected_stats.reads, 1)
	if err != nil {
		t.Errorf("\"Read\" returns an unexpected error:\n%q", err.Error())
	}
	expected = []byte("Value_A")
	for i, e := range nabia_read.RawData {
		if e != expected[i] {
			t.Errorf("\"Read\" returns unexpected data or ContentType!\nGot %q, expected %q", nabia_read, expected)
		}
	}
	//UPDATE
	s1 := NewNabiaRecord([]byte("Modified value"), "application/json; charset=UTF-8")
	nabiaDB.Write("A", *s1)
	atomic.AddInt64(&expected_stats.reads, 1)
	atomic.AddInt64(&expected_stats.writes, 1)
	if !nabiaDB.Exists("A") {
		t.Errorf("Overwritten item doesn't exist!")
	}
	atomic.AddInt64(&expected_stats.reads, 1)
	nabia_read, err = nabiaDB.Read("A")
	if err != nil {
		t.Errorf("\"Read\" returns an unexpected error:\n%q", err.Error())
	}
	atomic.AddInt64(&expected_stats.reads, 1)
	expected = []byte("Modified value")
	expected_content_type = "application/json; charset=UTF-8"
	for i, e := range nabia_read.RawData {
		if e != expected[i] || nabia_read.ContentType != expected_content_type {
			t.Errorf("\"Write\" on an existing item saves unexpected data or ContentType!\nGot %q, expected %q", nabia_read, expected)
		}
	}
	//DESTROY
	if !nabiaDB.Exists("A") {
		t.Error("Can't destroy item because it doesn't exist!")
	}
	atomic.AddInt64(&expected_stats.reads, 1)
	nabiaDB.Destroy("A")
	atomic.AddInt64(&expected_stats.reads, 1)
	atomic.AddInt64(&expected_stats.writes, 1)
	atomic.AddInt64(&expected_stats.size, -1)
	if nabiaDB.Exists("A") {
		t.Error("\"Destroy\" isn't working!\nDeleted item still exists in DB.")
	}
	atomic.AddInt64(&expected_stats.reads, 1)

	// Test for unknown ContentType
	s2, err := NewNabiaRecord([]byte("Unknown ContentType Value"))
	if err := nabiaDB.Write("B", *s2); err != nil {
		t.Errorf("\"Write\" returns an unexpected error:\n%q", err.Error())
	}
	atomic.AddInt64(&expected_stats.reads, 1)
	atomic.AddInt64(&expected_stats.writes, 1)
	atomic.AddInt64(&expected_stats.size, 1)
	nabia_read, err = nabiaDB.Read("B")
	if err != nil {
		t.Errorf("\"Read\" returns an unexpected error:\n%q", err.Error())
	}
	atomic.AddInt64(&expected_stats.reads, 1)

	// Test for non-existent item
	nabiaDB.Destroy("C")
	atomic.AddInt64(&expected_stats.reads, 1)
	atomic.AddInt64(&expected_stats.writes, 1)
	if nabiaDB.Exists("C") {
		t.Error("\"Destroy\" isn't working!\nNon-existent item appears to exist in DB.")
	}
	atomic.AddInt64(&expected_stats.reads, 1)

	// Test for incorrect key
	incorrect_key := nabiaDB.Write("", *s) // This should not be allowed
	if !strings.Contains(incorrect_key.Error(), "key cannot be empty") {
		t.Error("Empty key should not be allowed")
	}

	// Test for incorrect values
	incorrect_value1 := nabiaDB.Write("/A", NabiaRecord{}) // This should not be allowed
	if !strings.Contains(incorrect_value1.Error(), "value cannot be nil") {
		t.Error("Empty NabiaRecord should not be allowed")
	}
	incorrect_value2 := nabiaDB.Write("/A", NabiaRecord{nil, "application/json; charset=UTF-8"}) // This should not be allowed
	if !strings.Contains(incorrect_value2.Error(), "value cannot be nil") {
		t.Error("nil NabiaRecord RawData should not be allowed")
	}
	incorrect_value3 := nabiaDB.Write("/A", NabiaRecord{[]byte("Value_A"), ""}) // This should not be allowed
	if !strings.Contains(incorrect_value3.Error(), "Content-Type cannot be empty") {
		t.Error("Empty NabiaRecord ContentType should not be allowed")
	}
	if !reflect.DeepEqual(nabiaDB.internals.metrics.dataActivity, expected_stats) {
		t.Errorf("Stats are not as expected.\nExpected: %+v\nGot: %+v", expected_stats, nabiaDB.internals.metrics.dataActivity)
	}

	// TODO move this to a separate function

}

func TestConcurrency(t *testing.T) {
	expected_stats := dataActivity{reads: 0, writes: 0, size: 0}
	nabiaDB, err := NewNabiaDB("concurrency.db")
	if err != nil {
		t.Errorf("Failed to create NabiaDB: %s", err)
	}
	defer os.Remove("concurrency.db")
	// Concurrency test with Destroy operation
	var wg sync.WaitGroup
	for i := 0; i < 1000000; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("Key_%d", i)
			value, err := NewNabiaRecord([]byte(fmt.Sprintf("Value_%d", i)))
			if err != nil {
				t.Errorf("error creating a random record")
			}
			operation := rand.Intn(3)
			switch operation {
			case 0:
				// Destroy before writing
				nabiaDB.Destroy(key)
				atomic.AddInt64(&expected_stats.reads, 1)
				atomic.AddInt64(&expected_stats.writes, 1)
				if nabiaDB.Exists(key) {
					t.Errorf("Destroy operation failed before writing for key: %s", key)
				}
				atomic.AddInt64(&expected_stats.reads, 1)
				nabiaDB.Write(key, *value)
				atomic.AddInt64(&expected_stats.reads, 1)
				atomic.AddInt64(&expected_stats.size, 1)
				atomic.AddInt64(&expected_stats.writes, 1)
			case 1:
				// Destroy after writing and verifying the value
				nabiaDB.Write(key, *value)
				atomic.AddInt64(&expected_stats.reads, 1)
				atomic.AddInt64(&expected_stats.writes, 1)
				atomic.AddInt64(&expected_stats.size, 1)
				readValue, err := nabiaDB.Read(key)
				if err != nil || !bytes.Equal(readValue.RawData, value.RawData) || readValue.ContentType != value.ContentType {
					t.Errorf("Write or Read operation failed for key: %s", key)
				}
				atomic.AddInt64(&expected_stats.reads, 1)
				nabiaDB.Destroy(key)
				atomic.AddInt64(&expected_stats.reads, 1)
				atomic.AddInt64(&expected_stats.writes, 1)
				atomic.AddInt64(&expected_stats.size, -1)
				if nabiaDB.Exists(key) {
					t.Errorf("Destroy operation failed after writing for key: %s", key)
				}
				atomic.AddInt64(&expected_stats.reads, 1)
			case 2:
				// Overwrite and check value again after checking value with first write
				nabiaDB.Write(key, *value) // first write
				atomic.AddInt64(&expected_stats.reads, 1)
				atomic.AddInt64(&expected_stats.writes, 1)
				atomic.AddInt64(&expected_stats.size, 1)
				readValue, err := nabiaDB.Read(key)
				atomic.AddInt64(&expected_stats.reads, 1)
				if err != nil || !bytes.Equal(readValue.RawData, value.RawData) || readValue.ContentType != value.ContentType {
					t.Errorf("First Write or Read operation failed for key: %s", key)
				}
				value2 := NewNabiaRecord([]byte(fmt.Sprintf("New_Value_%d", i)), "application/json; charset=UTF-8")
				nabiaDB.Write(key, *value2) // overwrite
				atomic.AddInt64(&expected_stats.reads, 1)
				atomic.AddInt64(&expected_stats.writes, 1)
				readValue2, err := nabiaDB.Read(key)
				atomic.AddInt64(&expected_stats.reads, 1)
				if err != nil || !bytes.Equal(readValue2.RawData, value2.RawData) || readValue2.ContentType != value2.ContentType {
					t.Errorf("Second Write or Read operation failed for key: %s", key)
				}
			}
		}(i)
	}
	wg.Wait()
	if !reflect.DeepEqual(nabiaDB.internals.metrics.dataActivity, expected_stats) {
		t.Errorf("Stats are not as expected.\nExpected: %+v\nGot: %+v", expected_stats, nabiaDB.internals.metrics.dataActivity)
	}

}
