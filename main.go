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
	"crypto/sha256"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const usageMessage = "" + `new usage: funccover [instrumentation flags] [arguments...]:
funccover generates an instrumented source code for function coverage
Generated source code can be built or ran normally to get the coverage data
Coverage data will be written to a file periodically while binary is running also when main ends

Currently funccover only works for single source file, source file path shall be given as an argument
`

func usage() {
	fmt.Fprintln(os.Stderr, usageMessage)
	fmt.Fprintln(os.Stderr, "Flags:")
	flag.PrintDefaults()
	os.Exit(2)
}

var (
	funcCoverPeriod = flag.Duration("period", 0, "period of the data collection ex.500ms\nif not given no periodical collection")
	direc           = flag.String("dir", "", "directory for instrumented package\nif not given /instrumented")
	outputFile      = flag.String("o", "cover.out", "file for coverage output")
)

type Options struct {
	output      string
	packageName string
	importCfg   string
}

var files []string

func main() {

	flag.Usage = usage
	flag.Parse()

	// Usage information when no arguments.
	if flag.NFlag() == 0 && flag.NArg() == 0 {
		flag.Usage()
	}

	err := parseFlags()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, `For usage information, run 'funccover' with no arguments and flags`)
		os.Exit(2)
	}

	args := flag.Args()

	if err != nil {
		usage()
		os.Exit(1)
	}

	if isCompile(args[0]) {
		args, err = instrumentPackage(args)
		if err != nil {
			panic(err)
		}
	} else if isLink(args[0]) {
		args, err = link(args)
		if err != nil {
			panic(err)
		}
	}

	executeCommand(args)

}

// parseFlags performs validations.
func parseFlags() error {

	if *funcCoverPeriod < 0 {
		return fmt.Errorf("-period: %s is not a valid period", *funcCoverPeriod)
	}

	if *outputFile == "" {
		return fmt.Errorf("output file name can not be empty")
	}

	if flag.NArg() == 0 {
		return fmt.Errorf("missing source file")
	}

	return nil
}

func getSources(args []string) ([]string, []string) {
	var sources []string
	var oth []string
	for _, arg := range args {
		if strings.HasSuffix(arg, ".go") {
			sources = append(sources, arg)
		} else {
			oth = append(oth, arg)
		}
	}
	return sources, oth
}

// forwardCommand runs the given command's argument list and exits the process
// with the exit code that was returned.
func executeCommand(args []string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func isCompile(command string) bool {
	command = filepath.Base(command)
	extension := filepath.Ext(command)
	if extension != "" {
		command = strings.TrimSuffix(command, extension)
	}
	if command == "compile" {
		return true
	}
	return false
}

func isLink(command string) bool {
	command = filepath.Base(command)
	extension := filepath.Ext(command)
	if extension != "" {
		command = strings.TrimSuffix(command, extension)
	}
	if command == "link" {
		return true
	}
	return false
}

func instrumentPackage(args []string) ([]string, error) {

	opt, err := getOptions(args[1:])
	if err != nil {
		return nil, err
	}

	if opt.output == "" || opt.packageName == "" || opt.importCfg == "" {
		return args, nil
	}

	pkg := opt.packageName
	buildDir := filepath.Dir(opt.output)
	if pkg != "main" {
		return args, nil
	}

	f, err := os.OpenFile(opt.importCfg, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}

	//
	if _, err = f.WriteString("packagefile covcollect=/Users/muratekici/Desktop/google/testdata/sample3/pkg.a"); err != nil {
		panic(err)
	}
	f.Close()

	file, err := os.Open(filepath.Join(buildDir, "importcfg"))
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	files, args = getSources(args)

	sum := sha256.Sum256([]byte(files[0]))
	uniqueHash := fmt.Sprintf("%x", sum[:6])

	if *direc == "" {
		*direc = buildDir
	}

	var instrumentation = packageInstrumentation{
		dir:      *direc,
		suffix:   uniqueHash,
		period:   *funcCoverPeriod,
		fileName: *outputFile,
	}

	for _, src := range files {
		res, _ := filepath.Abs(src)
		err := instrumentation.AddFile(res)
		if err != nil {
			panic(err)
		}
	}

	instrumented, err := instrumentation.Instrument()
	if err != nil {
		panic(err)
	}

	err = instrumentation.WriteInstrumented(instrumentation.dir, instrumented)

	if err != nil {
		panic(err)
	}

	for _, src := range files {
		filePath := filepath.Join(instrumentation.dir, filepath.Base(src))
		args = append(args, filePath)
	}

	fmt.Println("args: ", args)

	return args, nil
}

func link(args []string) ([]string, error) {

	opt, err := getOptions(args[1:])
	if err != nil {
		return nil, err
	}

	if opt.output == "" || opt.importCfg == "" {
		return args, nil
	}

	f, err := os.OpenFile(opt.importCfg, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	if _, err = f.WriteString("packagefile covcollect=/Users/muratekici/Desktop/google/testdata/sample3/pkg.a"); err != nil {
		panic(err)
	}
	f.Close()

	file, err := os.Open(opt.importCfg)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	b, err := ioutil.ReadAll(file)
	fmt.Println("yeni importcfg forl link -----> " + string(b))

	return args, nil
}
func getOptions(args []string) (Options, error) {

	n := len(args)
	index := 0
	var opt Options

	for index < n {
		if args[index][0] != '-' {
			break
		}
		if index+1 < n {
			index += parseOption(args[index], args[index+1], &opt)
		} else {
			index += parseOption(args[index], "", &opt)
		}
	}

	return opt, nil
}

func parseOption(arg1, arg2 string, opt *Options) int {
	splitted := strings.SplitN(arg1, "=", 2)
	if len(splitted) == 2 {
		if splitted[0] == "-o" {
			opt.output = splitted[2]
		} else if splitted[0] == "-p" {
			opt.packageName = splitted[2]
		} else if splitted[0] == "-importcfg" {
			opt.importCfg = splitted[2]
		}
		return 1
	} else if arg2 == "" || arg2[0] != '-' {
		if arg1 == "-o" {
			opt.output = arg2
		} else if arg1 == "-p" {
			opt.packageName = arg2
		} else if arg1 == "-importcfg" {
			opt.importCfg = arg2
		}
		return 2
	}
	return 1
}
