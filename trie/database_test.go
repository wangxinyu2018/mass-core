package trie

import (
	"testing"

	"github.com/wangxinyu2018/mass-core/trie/common"
	"github.com/wangxinyu2018/mass-core/trie/massdb/memorydb"
)

// Tests that the trie database returns a missing trie node error if attempting
// to retrieve the meta root.
func TestDatabaseMetarootFetch(t *testing.T) {
	db := NewDatabase(memorydb.New())
	if _, err := db.Node(common.Hash{}); err == nil {
		t.Fatalf("metaroot retrieval succeeded")
	}
}
