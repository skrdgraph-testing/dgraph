/*
 * Copyright 2017-2022 Dgraph Labs, Inc. and Contributors
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

package chunker

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func bufioReader(str string) *bufio.Reader {
	return bufio.NewReader(strings.NewReader(str))
}

// Test that problems at the start of the JSON document are caught.
func TestJSONLoadStart(t *testing.T) {
	var tests = []struct {
		json string
		desc string
	}{
		{"[,]", "Illegal rune found \",\", expecting {"},
		{"[a]", "Illegal rune found \"a\", expecting {"},
		{"{}]", "JSON map is followed by an extraneous ]"},
		{"These are words.", "file is not JSON"},
		{"\x1f\x8b\x08\x08\x3e\xc7\x0a\x5c\x00\x03\x65\x6d\x70\x74\x79\x00", "file is binary"},
	}

	for _, test := range tests {
		chunker := NewChunker(JsonFormat, 1000)
		_, err := chunker.Chunk(bufioReader(test.json))
		require.True(t, err != nil && err != io.EOF, test.desc)
	}
}

func TestChunkJSONMapAndArray(t *testing.T) {
	tests := []struct {
		json   string
		chunks []string
	}{
		{`[]`, []string{"[]"}},
		{`[{}]`, []string{"[{}]"}},
		{`[{"user": "alice"}]`, []string{`[{"user":"alice"}]`}},
		{`[{"user": "alice", "age": 26}]`, []string{`[{"user":"alice","age":26}]`}},
		{`[{"user": "alice", "age": 26}, {"name": "bob"}]`, []string{`[{"user":"alice","age":26},{"name":"bob"}]`}},
	}

	for _, test := range tests {
		chunker := NewChunker(JsonFormat, 1000)
		r := bufioReader(test.json)
		var chunks []string
		for {
			chunkBuf, err := chunker.Chunk(r)
			if err != nil {
				require.Equal(t, io.EOF, err, "Received error for %s", test)
			}

			chunks = append(chunks, chunkBuf.String())

			if err == io.EOF {
				break
			}
		}

		require.Equal(t, test.chunks, chunks, "Got different chunks")
	}
}

// Test that problems at the start of the next chunk are caught.
func TestJSONLoadReadNext(t *testing.T) {
	var tests = []struct {
		json string
		desc string
	}{
		{"[,]", "no start of JSON map 1"},
		{"[ this is not really a json array ]", "no start of JSON map 2"},
		{"[{]", "malformed map"},
		{"[{}", "malformed array"},
	}
	for _, test := range tests {
		chunker := NewChunker(JsonFormat, 1000)
		reader := bufioReader(test.json)
		chunkBuf, err := chunker.Chunk(reader)
		if err == nil {
			err = chunker.Parse(chunkBuf)
			require.True(t, err != nil && err != io.EOF, test.desc)
		} else {
			require.True(t, err != io.EOF, test.desc)
		}
	}
}

// Test that loading first chunk succeeds. No need to test that loaded chunk is valid.
func TestJSONLoadSuccessFirst(t *testing.T) {
	var tests = []struct {
		json string
		expt string
		desc string
	}{
		{"[{}]", "[{}]", "empty map"},
		{`[{"closingDelimeter":"}"}]`, `[{"closingDelimeter":"}"}]`, "quoted closing brace"},
		{`[{"company":"dgraph"}]`, `[{"company":"dgraph"}]`, "simple, compact map"},
		{
			"[\n  {\n    \"company\" : \"dgraph\"\n  }\n]\n",
			"[{\"company\":\"dgraph\"}]",
			"simple, pretty map",
		},
		{
			`[{"professor":"Alastor \"Mad-Eye\" Moody"}]`,
			`[{"professor":"Alastor \"Mad-Eye\" Moody"}]`,
			"escaped balanced quotes",
		},
		{

			`[{"something{": "}something"}]`,
			`[{"something{":"}something"}]`,
			"escape quoted brackets",
		},
		{
			`[{"height":"6'0\""}]`,
			`[{"height":"6'0\""}]`,
			"escaped unbalanced quote",
		},
		{
			`[{"house":{"Hermione":"Gryffindor","Cedric":"Hufflepuff","Luna":"Ravenclaw","Draco":"Slytherin",}}]`,
			`[{"house":{"Hermione":"Gryffindor","Cedric":"Hufflepuff","Luna":"Ravenclaw","Draco":"Slytherin",}}]`,
			"nested braces",
		},
	}
	for _, test := range tests {
		chunker := NewChunker(JsonFormat, 1000)
		reader := bufioReader(test.json)
		json, err := chunker.Chunk(reader)
		if err == io.EOF {
			// pass
		} else {
			require.NoError(t, err, test.desc)
		}
		//fmt.Fprintf(os.Stderr, "err = %v, json = %v\n", err, json)
		require.Equal(t, test.expt, json.String(), test.desc)
	}
}

// Test that loading all chunks succeeds. No need to test that loaded chunk is valid.
func TestJSONLoadSuccessAll(t *testing.T) {
	var testDoc = `
[
	{},
	{
		"closingDelimeter" : "}"
	},
	{
		"company" : "dgraph",
		"age": 3
	},
	{
		"professor" : "Alastor \"Mad-Eye\" Moody",
		"height"    : "6'0\""
	},
	{
		"house" : {
			"Hermione" : "Gryffindor",
			"Cedric"   : "Hufflepuff",
			"Luna"     : "Ravenclaw",
			"Draco"    : "Slytherin"
		}
	}
]`
	var testChunks = []string{
		`{}`,
		`{
		"closingDelimeter" : "}"
	}`,
		`{
		"company" : "dgraph",
		"age": 3
	}`,
		`{
		"professor" : "Alastor \"Mad-Eye\" Moody",
		"height"    : "6'0\""
	}`,
		`{
		"house" : {
			"Hermione" : "Gryffindor",
			"Cedric"   : "Hufflepuff",
			"Luna"     : "Ravenclaw",
			"Draco"    : "Slytherin"
		}
	}`,
	}

	chunker := NewChunker(JsonFormat, 1000)
	reader := bufioReader(testDoc)

	var json *bytes.Buffer
	var idx int

	var err error
	for idx = 0; err == nil; idx++ {
		desc := fmt.Sprintf("reading chunk #%d", idx+1)
		json, err = chunker.Chunk(reader)
		//fmt.Fprintf(os.Stderr, "err = %v, json = %v\n", err, json)
		if err != io.EOF {
			require.NoError(t, err, desc)
			require.Equal(t, testChunks[idx], json.String(), desc)
		}
	}
	require.Equal(t, io.EOF, err, "end reading JSON document")
}

type isJsonDataTest struct {
	possibleJsonData string
	expected         bool
}

var isJsonDataTests = []isJsonDataTest{
	isJsonDataTest{`[{
	"id": 1,
	"first_name": "Jeanette",
	"last_name": "Penddreth",
	"email": "jpenddreth0@census.gov",
	"gender": "Female",
	"ip_address": "26.58.193.2"
  }, {
	"id": 2,
	"first_name": "Giavani",
	"last_name": "Frediani",
	"email": "gfrediani1@senate.gov",
	"gender": "Male",
	"ip_address": "229.179.4.212"
  }]
  `,
		true},
	isJsonDataTest{`Just a simple string`,
		false},
}

func TestIsJSONData(t *testing.T) {

	for _, jsonTestData := range isJsonDataTests {
		reader := bufioReader(jsonTestData.possibleJsonData)
		if output, error := IsJSONData(reader); output != jsonTestData.expected {
			fmt.Println(error)
			// t.Errorf("got: %t, wanted: %t", output, jsonTestData.expected)
			assert.Equal(t, output, jsonTestData.expected, "Actual result is different than Expected result")
		}
	}
}

func TestRdfChunker_Parse_Positive(t *testing.T) {
	reader := bufioReader(`<0x01> <name> "Alice" .
	<0x01> <dgraph.type> "Person" .

`)

	chunk := NewChunker(RdfFormat, 1024)
	chunkBuff, _ := chunk.Chunk(reader)
	got := chunk.Parse(chunkBuff)

	assert.Nil(t, got, "If the data passed has correct format, Pares() func returns <nil>")
}

func TestRdfChunker_Parse_Negative(t *testing.T) {

	expectedResult := `while parsing line " <name> \"Alice\" .\n": while lexing <name> "Alice" . at line 1 column 6: Invalid quote for non-object.`

	reader := bufioReader(` <name> "Alice" .
	<0x01> <dgraph.type> "Person" .

`)

	chunk := NewChunker(RdfFormat, 1024)
	chunkBuff, _ := chunk.Chunk(reader)
	got := chunk.Parse(chunkBuff)

	// In this test we are passing incorrect data, hence an error is expected in got. If we do not receive any error then assert.
	assert.Equal(t, expectedResult, got.Error(), "If the data passed has incorrect format, Pares() func returns Some error")
}

type client struct {
	Hostname string `json:"Hostname"`
	IP       string `json:"IP"`
}

type connection struct {
	Clients []*client `json:"Clients"`
}

func TestJsonFormat_Chunk(t *testing.T) {

	expectedResult := "EOF"

	var clients []*client

	for lineCount := 0; lineCount < 1e5+2; lineCount++ {
		clients = append(clients, &client{Hostname: strconv.Itoa(lineCount), IP: strconv.Itoa(lineCount) + ":" + strconv.Itoa(lineCount)})
	}

	res, err := json.MarshalIndent(connection{Clients: clients}, "", "  ")
	Indented_Json := bytes.NewBuffer(res).String()

	reader := bufioReader(Indented_Json)

	chunk := NewChunker(JsonFormat, -1)
	bytesBuff, err := chunk.Chunk(reader)

	assert := assert.New(t)
	assert.Equal(expectedResult, err.Error(), "If the data passed has JSON format, Chunk() func returns EOF as error")
	assert.True(bytesBuff.Len() > 1, "If the data passed has JSON format, Chunk() func returns *bufio.Reader")
}
func TestFileReader(t *testing.T) {
	//create sample files
	_, thisFile, _, _ := runtime.Caller(0)
	dir := "test-files"
	require.NoError(t, os.MkdirAll(dir, os.ModePerm))
	testFilesDir := filepath.Join(filepath.Dir(thisFile), "test-files")
	var expectedOutcomes [2]string

	file_data := []struct {
		filename string
		content  string
	}{
		{"test-1", "This is test file 1."},
		{"test-2", "This is test file 2."},
	}
	for i, data := range file_data {
		filePath := filepath.Join(testFilesDir, data.filename)
		f, err := os.Create(filePath)
		require.NoError(t, err)
		defer f.Close()
		_, err = f.WriteString(data.content)
		require.NoError(t, err)
		expectedOutcomes[i] = data.content
	}

	files, err := ioutil.ReadDir(testFilesDir)
	require.NoError(t, err)

	for i, file := range files {

		testfilename := filepath.Join(testFilesDir, file.Name())
		reader, cleanup := FileReader(testfilename, nil)

		bytes, err := ioutil.ReadAll(reader)

		require.NoError(t, err)
		contents := string(bytes)
		//compare file content with correct string
		require.Equal(t, contents, expectedOutcomes[i])
		cleanup()
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("Error removing direcotory: %s", err.Error())
	}

}

func TestDataFormat(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	dir := "test-files"
	require.NoError(t, os.MkdirAll(dir, os.ModePerm))
	testFilesDir := filepath.Join(filepath.Dir(thisFile), "test-files")
	expectedOutcomes := [5]InputFormat{2, 1, 0, 2, 1}

	file_data := [5]string{"test-1.json", "test-2.rdf", "test-3.txt", "test-4.json.gz", "test-5.rdf.gz"}
	for i, data := range file_data {
		filePath := filepath.Join(testFilesDir, data)
		format := DataFormat(filePath, "")

		require.Equal(t, format, expectedOutcomes[i])
	}

}
