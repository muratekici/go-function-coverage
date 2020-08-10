//Copyright 2020 Google LLC

//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//    https://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.

// This code implements a source file instrumentation function
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/tools/go/ast/astutil"
)

type Instrumenter interface {
	AddFile(path string) error
	Instrument() map[string][]byte
	WriteInstrumented(dir string, instrumented map[string][]byte)
}

type packageInstrumentation struct {
	fileName    string
	period      time.Duration
	fset        *token.FileSet
	parsedFiles map[string][]byte
	dir         string
}

func (h *packageInstrumentation) AddFile(src string) error {

	if h.fset == nil {
		h.fset = token.NewFileSet()
	}

	content, err := ioutil.ReadFile(src)
	if err != nil {
		panic(err)
	}

	if h.parsedFiles == nil {
		h.parsedFiles = make(map[string][]byte)
	}

	h.parsedFiles[src] = content
	return nil
}

func (h *packageInstrumentation) Instrument() (map[string][]byte, error) {

	var instrumented = make(map[string][]byte)

	var err error

	for src, content := range h.parsedFiles {

		var funcCover = FuncCover{}

		funcCover.FuncBlocks, err = SaveFuncs(src, content)
		if err != nil {
			return nil, err
		}

		sum := sha256.Sum256([]byte(src))
		uniqueHash := fmt.Sprintf("%x", sum[:6])

		buf := new(bytes.Buffer)
		err = addCounters(buf, content, uniqueHash, h.fileName+"_"+src)
		if err != nil {
			return nil, err
		}

		parsedFile, err := parser.ParseFile(h.fset, "", buf, parser.ParseComments)
		if err != nil {
			return nil, err
		}

		// Ensure necessary imports are inserted
		imports := []string{"covcollect"}
		for _, impr := range imports {
			astutil.AddImport(h.fset, parsedFile, impr)
		}

		importedContent := astToByte(h.fset, parsedFile)
		buf = new(bytes.Buffer)

		fmt.Fprintf(buf, "%s", importedContent)

		// Write necessary functions variables and an init function that calls periodical_retrieve_$hash() with w
		declCover(buf, uniqueHash, h.fileName+"_"+src, h.period, funcCover)

		instrumented[src] = buf.Bytes()
	}

	return instrumented, nil
}

func (h *packageInstrumentation) WriteInstrumented(instrumented map[string][]byte) error {

	path := h.dir
	os.MkdirAll(path, os.ModePerm)

	for src, content := range instrumented {
		if path != "" {
			filePath := filepath.Join(path, filepath.Base(src))
			fd, err := os.Create(filePath)
			if err != nil {
				return err
			}
			w := bufio.NewWriter(fd)
			fmt.Fprintf(w, "\\%s\n%s", src, content)
			w.Flush()
			fd.Close()
		} else {
			fmt.Printf("\\%s\n%s", src, content)
		}

	}
	return nil
}
