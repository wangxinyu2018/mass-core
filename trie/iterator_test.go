package trie

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	"github.com/ylpool/mass-core/trie/common"
	"github.com/ylpool/mass-core/trie/massdb/memorydb"
)

func TestIterator(t *testing.T) {
	trie := newEmpty()
	vals := []struct{ k, v string }{
		{"do", "verb"},
		{"ether", "wookiedoo"},
		{"horse", "stallion"},
		{"shaman", "horse"},
		{"doge", "coin"},
		{"dog", "puppy"},
		{"somethingveryoddindeedthis is", "myothernodedata"},
	}
	all := make(map[string]string)
	for _, val := range vals {
		all[val.k] = val.v
		trie.Update([]byte(val.k), []byte(val.v))
	}
	trie.Commit()

	found := make(map[string]string)
	it := NewIterator(trie.NodeIterator(nil))
	for it.Next() {
		found[string(it.Key)] = string(it.Value)
	}

	for k, v := range all {
		if found[k] != v {
			t.Errorf("iterator value mismatch for %s: got %q want %q", k, found[k], v)
		}
	}
}

type kv struct {
	k, v []byte
	t    bool
}

func TestIteratorLargeData(t *testing.T) {
	trie := newEmpty()
	vals := make(map[string]*kv)

	for i := byte(0); i < 255; i++ {
		value := &kv{common.LeftPadBytes([]byte{i}, 32), []byte{i}, false}
		value2 := &kv{common.LeftPadBytes([]byte{10, i}, 32), []byte{i}, false}
		trie.Update(value.k, value.v)
		trie.Update(value2.k, value2.v)
		vals[string(value.k)] = value
		vals[string(value2.k)] = value2
	}

	it := NewIterator(trie.NodeIterator(nil))
	for it.Next() {
		vals[string(it.Key)].t = true
	}

	var untouched []*kv
	for _, value := range vals {
		if !value.t {
			untouched = append(untouched, value)
		}
	}

	if len(untouched) > 0 {
		t.Errorf("Missed %d nodes", len(untouched))
		for _, value := range untouched {
			t.Error(value)
		}
	}
}

// // Tests that the node iterator indeed walks over the entire database contents.
// func TestNodeIteratorCoverage(t *testing.T) {
// 	// Create some arbitrary test trie to iterate
// 	db, trie, _ := makeTestTrie()

// 	// Gather all the node hashes found by the iterator
// 	hashes := make(map[common.Hash]struct{})
// 	for it := trie.NodeIterator(nil); it.Next(true); {
// 		if it.Hash() != (common.Hash{}) {
// 			hashes[it.Hash()] = struct{}{}
// 		}
// 	}
// 	// Cross check the hashes and the database itself
// 	for hash := range hashes {
// 		if _, err := db.Node(hash); err != nil {
// 			t.Errorf("failed to retrieve reported node %x: %v", hash, err)
// 		}
// 	}
// 	for hash, obj := range db.dirties {
// 		if obj != nil && hash != (common.Hash{}) {
// 			if _, ok := hashes[hash]; !ok {
// 				t.Errorf("state entry not reported %x", hash)
// 			}
// 		}
// 	}
// 	it := db.diskdb.NewIterator(nil, nil)
// 	for it.Next() {
// 		key := it.Key()
// 		if _, ok := hashes[common.BytesToHash(key)]; !ok {
// 			t.Errorf("state entry not reported %x", key)
// 		}
// 	}
// 	it.Release()
// }

// // makeTestTrie create a sample test trie to test node-wise reconstruction.
// func makeTestTrie() (*Database, *SecureTrie, map[string][]byte) {
// 	// Create an empty trie
// 	triedb := NewDatabase(memorydb.New())
// 	trie, _ := NewSecure(common.Hash{}, triedb)

// 	// Fill it with some arbitrary data
// 	content := make(map[string][]byte)
// 	for i := byte(0); i < 255; i++ {
// 		// Map the same data under multiple keys
// 		key, val := common.LeftPadBytes([]byte{1, i}, 32), []byte{i}
// 		content[string(key)] = val
// 		trie.Update(key, val)

// 		key, val = common.LeftPadBytes([]byte{2, i}, 32), []byte{i}
// 		content[string(key)] = val
// 		trie.Update(key, val)

// 		// Add some other data to inflate the trie
// 		for j := byte(3); j < 13; j++ {
// 			key, val = common.LeftPadBytes([]byte{j, i}, 32), []byte{j, i}
// 			content[string(key)] = val
// 			trie.Update(key, val)
// 		}
// 	}
// 	trie.Commit(nil)

// 	// Return the generated trie
// 	return triedb, trie, content
// }

type kvs struct{ k, v string }

var testdata1 = []kvs{
	{"barb", "ba"},
	{"bard", "bc"},
	{"bars", "bb"},
	{"bar", "b"},
	{"fab", "z"},
	{"food", "ab"},
	{"foos", "aa"},
	{"foo", "a"},
}

var testdata2 = []kvs{
	{"aardvark", "c"},
	{"bar", "b"},
	{"barb", "bd"},
	{"bars", "be"},
	{"fab", "z"},
	{"foo", "a"},
	{"foos", "aa"},
	{"food", "ab"},
	{"jars", "d"},
}

func TestIteratorSeek(t *testing.T) {
	trie := newEmpty()
	for _, val := range testdata1 {
		trie.Update([]byte(val.k), []byte(val.v))
	}

	// Seek to the middle.
	it := NewIterator(trie.NodeIterator([]byte("fab")))
	if err := checkIteratorOrder(testdata1[4:], it); err != nil {
		t.Fatal(err)
	}

	// Seek to a non-existent key.
	it = NewIterator(trie.NodeIterator([]byte("barc")))
	if err := checkIteratorOrder(testdata1[1:], it); err != nil {
		t.Fatal(err)
	}

	// Seek beyond the end.
	it = NewIterator(trie.NodeIterator([]byte("z")))
	if err := checkIteratorOrder(nil, it); err != nil {
		t.Fatal(err)
	}
}

func checkIteratorOrder(want []kvs, it *Iterator) error {
	for it.Next() {
		if len(want) == 0 {
			return fmt.Errorf("didn't expect any more values, got key %q", it.Key)
		}
		if !bytes.Equal(it.Key, []byte(want[0].k)) {
			return fmt.Errorf("wrong key: got %q, want %q", it.Key, want[0].k)
		}
		want = want[1:]
	}
	if len(want) > 0 {
		return fmt.Errorf("iterator ended early, want key %q", want[0])
	}
	return nil
}

func TestIteratorNoDups(t *testing.T) {
	var tr Trie
	for _, val := range testdata1 {
		tr.Update([]byte(val.k), []byte(val.v))
	}
	checkIteratorNoDups(t, tr.NodeIterator(nil), nil)
}

// This test checks that nodeIterator.Next can be retried after inserting missing trie nodes.
func TestIteratorContinueAfterErrorDisk(t *testing.T) { testIteratorContinueAfterError(t) }

// func TestIteratorContinueAfterErrorMemonly(t *testing.T) { testIteratorContinueAfterError(t, true) }

func testIteratorContinueAfterError(t *testing.T) {
	diskdb := memorydb.New()
	triedb := NewDatabase(diskdb)

	tr, _ := New(common.Hash{}, triedb)
	for _, val := range testdata1 {
		tr.Update([]byte(val.k), []byte(val.v))
	}
	tr.Commit()
	wantNodeCount := checkIteratorNoDups(t, tr.NodeIterator(nil), nil)

	var (
		diskKeys [][]byte
		// memKeys  []common.Hash
	)
	it := diskdb.NewIterator(nil, nil)
	for it.Next() {
		diskKeys = append(diskKeys, it.Key())
	}
	it.Release()
	for i := 0; i < 20; i++ {
		// Create trie that will load all nodes from DB.
		tr, _ := New(tr.Hash(), triedb)

		// Remove a random node from the database. It can't be the root node
		// because that one is already loaded.
		var (
			rkey common.Hash
			rval []byte
		)
		for {
			copy(rkey[:], diskKeys[rand.Intn(len(diskKeys))])
			if rkey != tr.Hash() {
				break
			}
		}
		rval, _ = diskdb.Get(rkey[:])
		diskdb.Delete(rkey[:])
		// Iterate until the error is hit.
		seen := make(map[string]bool)
		it := tr.NodeIterator(nil)
		checkIteratorNoDups(t, it, seen)
		missing, ok := it.Error().(*MissingNodeError)
		if !ok || missing.NodeHash != rkey {
			t.Fatal("didn't hit missing node, got", it.Error())
		}

		// Add the node back and continue iteration.
		diskdb.Put(rkey[:], rval)
		checkIteratorNoDups(t, it, seen)
		if it.Error() != nil {
			t.Fatal("unexpected error", it.Error())
		}
		if len(seen) != wantNodeCount {
			t.Fatal("wrong node iteration count, got", len(seen), "want", wantNodeCount)
		}
	}
}

// TODO:
// // Similar to the test above, this one checks that failure to create nodeIterator at a
// // certain key prefix behaves correctly when Next is called. The expectation is that Next
// // should retry seeking before returning true for the first time.
// func TestIteratorContinueAfterSeekErrorDisk(t *testing.T) {
// 	testIteratorContinueAfterSeekError(t)
// }

// func testIteratorContinueAfterSeekError(t *testing.T) {
// 	// Commit test trie to db, then remove the node containing "bars".
// 	diskdb := memorydb.New()
// 	triedb := NewDatabase(diskdb)

// 	ctr, _ := New(common.Hash{}, triedb)
// 	for _, val := range testdata1 {
// 		ctr.Update([]byte(val.k), []byte(val.v))
// 	}
// 	root, _ := ctr.Commit()
// 	barNodeHash := common.HexToHash("05041990364eb72fcb1127652ce40d8bab765f2bfe53225b1170d276cc101c2e")
// 	var (
// 		barNodeBlob []byte
// 		// barNodeObj  *cachedNode
// 	)
// 	barNodeBlob, _ = diskdb.Get(barNodeHash[:])
// 	diskdb.Delete(barNodeHash[:])
// 	// Create a new iterator that seeks to "bars". Seeking can't proceed because
// 	// the node is missing.
// 	tr, _ := New(root, triedb)
// 	it := tr.NodeIterator([]byte("bars"))
// 	missing, ok := it.Error().(*MissingNodeError)
// 	if !ok {
// 		t.Fatal("want MissingNodeError, got", it.Error())
// 	} else if missing.NodeHash != barNodeHash {
// 		t.Fatal("wrong node missing")
// 	}
// 	// Reinsert the missing node.
// 	diskdb.Put(barNodeHash[:], barNodeBlob)
// 	// Check that iteration produces the right set of values.
// 	if err := checkIteratorOrder(testdata1[2:], NewIterator(it)); err != nil {
// 		t.Fatal(err)
// 	}
// }

func checkIteratorNoDups(t *testing.T, it NodeIterator, seen map[string]bool) int {
	if seen == nil {
		seen = make(map[string]bool)
	}
	for it.Next(true) {
		if seen[string(it.Path())] {
			t.Fatalf("iterator visited node path %x twice", it.Path())
		}
		seen[string(it.Path())] = true
	}
	return len(seen)
}
