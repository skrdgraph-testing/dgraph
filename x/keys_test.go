/*
 * Copyright 2016-2022 Dgraph Labs, Inc. and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package x

import (
	"fmt"
	"math"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNameSpace(t *testing.T) {
	ns := uint64(133)
	attr := "name"
	nsAttr := NamespaceAttr(ns, attr)
	require.Equal(t, 8+len(attr), len(nsAttr))
	parsedAttr := ParseAttr(nsAttr)
	require.Equal(t, attr, parsedAttr)
}

func TestDataKey(t *testing.T) {
	var uid uint64

	// key with uid = 0 is invalid
	uid = 0
	key := DataKey(NamespaceAttr(GalaxyNamespace, "bad uid"), uid)
	_, err := Parse(key)
	require.Error(t, err)

	for uid = 1; uid < 1001; uid++ {
		// Use the uid to derive the attribute so it has variable length and the test
		// can verify that multiple sizes of attr work correctly.
		sattr := fmt.Sprintf("attr:%d", uid)
		key := DataKey(NamespaceAttr(GalaxyNamespace, sattr), uid)
		pk, err := Parse(key)
		require.NoError(t, err)

		require.True(t, pk.IsData())
		require.Equal(t, sattr, ParseAttr(pk.Attr))
		require.Equal(t, uid, pk.Uid)
		require.Equal(t, uint64(0), pk.StartUid)
	}

	keys := make([]string, 0, 1024)
	for uid = 1024; uid >= 1; uid-- {
		key := DataKey(NamespaceAttr(GalaxyNamespace, "testing.key"), uid)
		keys = append(keys, string(key))
	}
	// Test that sorting is as expected.
	sort.Strings(keys)
	require.True(t, sort.StringsAreSorted(keys))
	for i, key := range keys {
		exp := DataKey(NamespaceAttr(GalaxyNamespace, "testing.key"), uint64(i+1))
		require.Equal(t, string(exp), key)
	}
}

func TestParseDataKeyWithStartUid(t *testing.T) {
	var uid uint64
	startUid := uint64(math.MaxUint64)
	for uid = 1; uid < 1001; uid++ {
		sattr := fmt.Sprintf("attr:%d", uid)
		key := DataKey(NamespaceAttr(GalaxyNamespace, sattr), uid)
		key, err := SplitKey(key, startUid)
		require.NoError(t, err)
		pk, err := Parse(key)
		require.NoError(t, err)

		require.True(t, pk.IsData())
		require.Equal(t, sattr, ParseAttr(pk.Attr))
		require.Equal(t, uid, pk.Uid)
		require.Equal(t, pk.HasStartUid, true)
		require.Equal(t, startUid, pk.StartUid)
	}
}

func TestIndexKey(t *testing.T) {
	var uid uint64
	for uid = 0; uid < 1001; uid++ {
		sattr := fmt.Sprintf("attr:%d", uid)
		sterm := fmt.Sprintf("term:%d", uid)

		key := IndexKey(NamespaceAttr(GalaxyNamespace, sattr), sterm)
		pk, err := Parse(key)
		require.NoError(t, err)

		require.True(t, pk.IsIndex())
		require.Equal(t, sattr, ParseAttr(pk.Attr))
		require.Equal(t, sterm, pk.Term)
	}
}

func TestIndexKeyWithStartUid(t *testing.T) {
	var uid uint64
	startUid := uint64(math.MaxUint64)
	for uid = 0; uid < 1001; uid++ {
		sattr := fmt.Sprintf("attr:%d", uid)
		sterm := fmt.Sprintf("term:%d", uid)

		key := IndexKey(NamespaceAttr(GalaxyNamespace, sattr), sterm)
		key, err := SplitKey(key, startUid)
		require.NoError(t, err)
		pk, err := Parse(key)
		require.NoError(t, err)

		require.True(t, pk.IsIndex())
		require.Equal(t, sattr, ParseAttr(pk.Attr))
		require.Equal(t, sterm, pk.Term)
		require.Equal(t, pk.HasStartUid, true)
		require.Equal(t, startUid, pk.StartUid)
	}
}

func TestReverseKey(t *testing.T) {
	var uid uint64
	for uid = 1; uid < 1001; uid++ {
		sattr := fmt.Sprintf("attr:%d", uid)

		key := ReverseKey(NamespaceAttr(GalaxyNamespace, sattr), uid)
		pk, err := Parse(key)
		require.NoError(t, err)

		require.True(t, pk.IsReverse())
		require.Equal(t, sattr, ParseAttr(pk.Attr))
		require.Equal(t, uid, pk.Uid)
	}
}

func TestReverseKeyWithStartUid(t *testing.T) {
	var uid uint64
	startUid := uint64(math.MaxUint64)
	for uid = 1; uid < 1001; uid++ {
		sattr := fmt.Sprintf("attr:%d", uid)

		key := ReverseKey(NamespaceAttr(GalaxyNamespace, sattr), uid)
		key, err := SplitKey(key, startUid)
		require.NoError(t, err)
		pk, err := Parse(key)
		require.NoError(t, err)

		require.True(t, pk.IsReverse())
		require.Equal(t, sattr, ParseAttr(pk.Attr))
		require.Equal(t, uid, pk.Uid)
		require.Equal(t, pk.HasStartUid, true)
		require.Equal(t, startUid, pk.StartUid)
	}
}

func TestCountKey(t *testing.T) {
	var count uint32
	for count = 0; count < 1001; count++ {
		sattr := fmt.Sprintf("attr:%d", count)

		key := CountKey(NamespaceAttr(GalaxyNamespace, sattr), count, true)
		pk, err := Parse(key)
		require.NoError(t, err)

		require.True(t, pk.IsCountOrCountRev())
		require.Equal(t, sattr, ParseAttr(pk.Attr))
		require.Equal(t, count, pk.Count)
	}
}

func TestCountKeyWithStartUid(t *testing.T) {
	var count uint32
	startUid := uint64(math.MaxUint64)
	for count = 0; count < 1001; count++ {
		sattr := fmt.Sprintf("attr:%d", count)

		key := CountKey(NamespaceAttr(GalaxyNamespace, sattr), count, true)
		key, err := SplitKey(key, startUid)
		require.NoError(t, err)
		pk, err := Parse(key)
		require.NoError(t, err)

		require.True(t, pk.IsCountOrCountRev())
		require.Equal(t, sattr, ParseAttr(pk.Attr))
		require.Equal(t, count, pk.Count)
		require.Equal(t, pk.HasStartUid, true)
		require.Equal(t, startUid, pk.StartUid)
	}
}

func TestSchemaKey(t *testing.T) {
	var uid uint64
	for uid = 0; uid < 1001; uid++ {
		sattr := fmt.Sprintf("attr:%d", uid)

		key := SchemaKey(NamespaceAttr(GalaxyNamespace, sattr))
		pk, err := Parse(key)
		require.NoError(t, err)

		require.True(t, pk.IsSchema())
		require.Equal(t, sattr, ParseAttr(pk.Attr))
	}
}

func TestTypeKey(t *testing.T) {
	var uid uint64
	for uid = 0; uid < 1001; uid++ {
		sattr := fmt.Sprintf("attr:%d", uid)

		key := TypeKey(NamespaceAttr(GalaxyNamespace, sattr))
		pk, err := Parse(key)
		require.NoError(t, err)

		require.True(t, pk.IsType())
		require.Equal(t, sattr, ParseAttr(pk.Attr))
	}
}

func TestBadStartUid(t *testing.T) {
	testKey := func(key []byte) {
		key, err := SplitKey(key, 10)
		require.NoError(t, err)
		_, err = Parse(key)
		require.NoError(t, err)
		key = append(key, 0)
		_, err = Parse(key)
		require.Error(t, err)
	}

	key := DataKey(NamespaceAttr(GalaxyNamespace, "aa"), 1)
	testKey(key)

	key = ReverseKey(NamespaceAttr(GalaxyNamespace, "aa"), 1)
	testKey(key)

	key = CountKey(NamespaceAttr(GalaxyNamespace, "aa"), 0, false)
	testKey(key)

	key = CountKey(NamespaceAttr(GalaxyNamespace, "aa"), 0, true)
	testKey(key)
}

func TestBadKeys(t *testing.T) {
	// 0-len key
	key := []byte{}
	_, err := Parse(key)
	require.Error(t, err)

	// key of len < 3
	key = []byte{1}
	_, err = Parse(key)
	require.Error(t, err)

	key = []byte{1, 2}
	_, err = Parse(key)
	require.Error(t, err)

	// key of len < sz (key[1], key[2])
	key = []byte{1, 0x00, 0x04, 1, 2}
	_, err = Parse(key)
	require.Error(t, err)

	// key with uid = 0 is invalid
	uid := 0
	key = DataKey(NamespaceAttr(GalaxyNamespace, "bad uid"), uint64(uid))
	_, err = Parse(key)
	require.Error(t, err)
}
