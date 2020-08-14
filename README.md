**This is not an officially supported Google product.**

# Go Function Coverage

The project aims to collect Go function-level coverage with low overhead for any
binary.

For context, the existing coverage in Go works only for tests, and only have the
line-coverage, which can be too inefficient to run in a production environment.

## Source Code Headers

Every file containing source code must include copyright and license
information. This includes any JS/CSS files that you might be serving out to
browsers. (This is to help well-intentioned people avoid accidental copying that
doesn't comply with the license.)

Apache header:

```
Copyright 2020 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```

## Overview

Go Function Coverage tool 'funccover' support generating instrumented source code files
so that running binary automatically collects the coverage data for functions.
    
'funccover' inserts a global function coverage variable that will keep the coverage data for functions 
to the given source code. Then it inserts necessary instructions to the top of each function 
(basic assignment instruction to global coverage variable). This way when a function starts executing, 
global coverage variable will keep the information that this function started execution some time. 

We also have to save that coverage information somewhere. Initially 'funccover' tool writes coverage information
to a file (RPC will be more useful in the future). Currently 'funccover' tool inserts 2 functions to the given
source code, one writes coverage data to a file, other calls it periodically. Period must be given as a flag to the tool.
Tool also inserts a defer call to the main to write coverage data after main function ends. So it is more general. 

## Quickstart

First you have to build a custom version of go forked from original source. It implements the -cover flag.

https://github.com/muratekici/go

Get the module from Github and install it into your $GOPATH/bin/
```bash
$ go get github.com/muratekici/go-function-coverage/tree/go-compile-integration/...
```

Add your _$GOPATH/bin_ into your _$PATH_ ([instructions](
https://github.com/golang/go/wiki/GOPATH))

## How To Use It

```bash
$ go tool compile -cover='period=? o=?' [other args...]
```

All sources given to funccover must be in the same package

### Flags

'funccover' has 2 flags. Each flag tells 'funccover' how it should instrument the source codes. 

#### period duration

period represents the period of the data collection, if it is not given periodical collection will be disabled. 

```bash
period=500ms
```

#### o string

o sets the name of the coverage output file name (default "cover.out").

```bash
o=coverage.out
```

### Example Usage

You have 2 source files named src.go and fun.go, both belongs to the same package. Normally binary runs with 2 arguments. You want to get the function coverage data for the binary to a file named cover.txt and since it is a long running code you want to get the coverage data every 1 minutes.

```bash
$ go tool compile cover='period=1m -o=cover.txt' -o=src.o src.go fun.go
$ go tool link  -o a src.o
.-o argument1 argument2
```

After you build the instrumented binary, you can run the binary normally (same way you run the binary for src.go) and coverage data will be written to cover.txt in following format:

```
path/to/original/source1:functionname1:line1:coverage1
path/to/original/source2:functionname2:line2:coverage2
path/to/original/source3:functionname3:line3:coverage3
...
```
