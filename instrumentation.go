package main

import (
	"bufio"
	"bytes"
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
	suffix      string
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

	var funcCover = FuncCover{}
	var mainFile = ""
	var instrumented = make(map[string][]byte)

	for src, content := range h.parsedFiles {
		temp, flag, err := SaveFuncs(src, content)
		if err != nil {
			return nil, err
		}
		funcCover.FuncBlocks = append(funcCover.FuncBlocks, temp...)
		if flag == true {
			mainFile = src
		}
	}

	var currentIndex = 0

	for src, content := range h.parsedFiles {

		buf := new(bytes.Buffer)

		index, err := addCounters(buf, content, h.suffix, h.fileName, currentIndex)
		currentIndex = index

		if err != nil {
			return nil, err
		}

		if mainFile != src {
			instrumented[src] = buf.Bytes()
			continue
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
		declCover(buf, h.suffix, h.fileName, h.period, funcCover)

		instrumented[src] = buf.Bytes()
	}

	return instrumented, nil
}

func (h *packageInstrumentation) WriteInstrumented(dir string, instrumented map[string][]byte) error {

	path := dir
	os.MkdirAll(path, os.ModePerm)

	for src, content := range instrumented {
		filePath := filepath.Join(path, filepath.Base(src))
		fd, err := os.Create(filePath)
		if err != nil {
			return err
		}
		w := bufio.NewWriter(fd)
		fmt.Fprintf(w, "%s", content)
		w.Flush()
		fd.Close()
	}
	return nil
}
