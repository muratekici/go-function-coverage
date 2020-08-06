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
	_ "io/ioutil"
	_ "log"
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
		executeCommand(args)
	} else if isLink(args[0]) {
		args, err = link(args)
		if args[1] != "-V=full" {
			executeCommand(args)
		} else {
			executeCommand(args)
		}
	} else {
		executeCommand(args)
	}

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

	///usr/local/go/pkg/tool/darwin_amd64/link
	pkgPath := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(args[0]))), filepath.Base(filepath.Dir(args[0])))
	fmt.Println(pkgPath)
	/// create importcfg for compiling covcollect
	covImports := "cov_importcfg"
	pathToCovCollect := os.Getenv("GOPATH") + "/src/github.com/muratekici/go-function-coverage/pkg/covcollect/covcollect.go "

	catArgs := []string{"bash", "-c", "cat >" + filepath.Join(buildDir, covImports) + ` << 'EOF'
packagefile fmt=` + pkgPath + `/fmt.a
packagefile bufio=` + pkgPath + `/bufio.a
packagefile os=` + pkgPath + `/os.a
packagefile time=` + pkgPath + `/time.a
EOF`}

	fmt.Println(catArgs)
	executeCommand(catArgs)

	covCollectArgs := []string{args[0], "-o", filepath.Join(buildDir, "cov_pkg.a"), "-p", "covcollect", "-complete", "-std", "-importcfg=" + filepath.Join(buildDir, covImports), "-pack", pathToCovCollect}
	fmt.Println(covCollectArgs)
	executeCommand(covCollectArgs)
	fmt.Println("compile cov bilmemne")

	f, err := os.OpenFile(opt.importCfg, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	f.WriteString("packagefile covcollect=" + filepath.Join(buildDir, "cov_pkg.a") + "\n")
	f.Close()

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

	buildDir := filepath.Dir(opt.importCfg)
	pkgPath := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(args[0]))), filepath.Base(filepath.Dir(args[0])))

	f.WriteString("packagefile covcollect=" + filepath.Join(buildDir, "cov_pkg.a") + "\n")
	f.WriteString("packagefile fmt=" + pkgPath + "/fmt.a\n")
	f.WriteString("packagefile bufio=" + pkgPath + "/bufio.a\n")
	f.WriteString("packagefile os=" + pkgPath + "/os.a\n")
	f.WriteString("packagefile time=" + pkgPath + "/time.a\n")
	f.WriteString("packagefile bytes=" + pkgPath + "/bytes.a\n")
	f.WriteString("packagefile errors=" + pkgPath + "/errors.a\n")
	f.WriteString("packagefile io=" + pkgPath + "/io.a\n")
	f.WriteString("packagefile unicode/utf8=" + pkgPath + "/unicode/utf8.a\n")
	f.WriteString("packagefile unicode=" + pkgPath + "/unicode.a\n")
	f.WriteString("packagefile strconv=" + pkgPath + "/strconv.a\n")
	f.WriteString("packagefile internal/fmtsort=" + pkgPath + "/internal/fmtsort.a\n")
	f.WriteString("packagefile reflect=" + pkgPath + "/reflect.a\n")
	f.WriteString("packagefile sync=" + pkgPath + "/sync.a\n")
	f.WriteString("packagefile math=" + pkgPath + "/math.a\n")
	f.WriteString("packagefile syscall=" + pkgPath + "/syscall.a\n")
	f.WriteString("packagefile internal/testlog=" + pkgPath + "/internal/testlog.a\n")
	f.WriteString("packagefile internal/oserror=" + pkgPath + "/internal/oserror.a\n")
	f.WriteString("packagefile internal/poll=" + pkgPath + "/internal/poll.a\n")
	f.WriteString("packagefile sync/atomic=" + pkgPath + "/sync/atomic.a\n")
	f.WriteString("packagefile internal/syscall/execenv=" + pkgPath + "/internal/syscall/execenv.a\n")
	f.WriteString("packagefile internal/syscall/unix=" + pkgPath + "/internal/syscall/unix.a\n")
	f.WriteString("packagefile internal/reflectlite=" + pkgPath + "/internal/reflectlite.a\n")
	f.WriteString("packagefile internal/syscall/unix=" + pkgPath + "/internal/syscall/unix.a\n")
	f.WriteString("packagefile internal/reflectlite=" + pkgPath + "/internal/reflectlite.a\n")
	f.WriteString("packagefile math/bits=" + pkgPath + "/math/bits.a\n")
	f.WriteString("packagefile sort=" + pkgPath + "/sort.a\n")
	f.WriteString("packagefile internal/race=" + pkgPath + "/internal/race.a\n")
	f.Close()

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
