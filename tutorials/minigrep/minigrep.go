package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/mariomac/pipes/pipe"
)

// FileLine stores a line match.
type FileLine struct {
	// FileName of the file where the match is found.
	// Might be empty if minigrep is reading from the standard input.
	FileName string
	// Actual line that matches the user-provided regular expression
	Line string
}

// MiniGrepNodes enumerates and connects the nodes that have a role
// in minigrep.
type MiniGrepNodes struct {
	fileFinder  pipe.Start[*os.File]
	fileScanner pipe.Middle[*os.File, FileLine]
	matchFilter pipe.Middle[FileLine, FileLine]
	printer     pipe.Final[FileLine]
}

// Connect describes the data flow across de different nodes in the minigrep pipeline.
func (n *MiniGrepNodes) Connect() {
	n.fileFinder.SendTo(n.fileScanner)
	n.fileScanner.SendTo(n.matchFilter)
	n.matchFilter.SendTo(n.printer)
}

// auxiliary functions that return pointers to the different fields of the MiniGrepNodes
// struct. They are grouped here for code conciseness in the AddStart, AddMiddle, etc... function invocations.
func finderPtr(f *MiniGrepNodes) *pipe.Start[*os.File]             { return &f.fileFinder }
func scannerPtr(f *MiniGrepNodes) *pipe.Middle[*os.File, FileLine] { return &f.fileScanner }
func matcherPtr(f *MiniGrepNodes) *pipe.Middle[FileLine, FileLine] { return &f.matchFilter }
func printerPtr(f *MiniGrepNodes) *pipe.Final[FileLine]            { return &f.printer }

// FileFinder opens the files passed as argument and forwards them to the next pipeline stage.
// If the file is not found or can't be opened, it just prints a message in the standard error.
func FileFinder(files []string) pipe.StartFunc[*os.File] {
	return func(out chan<- *os.File) {
		// if no file patterns are provided, minigrep filters standard input
		if len(files) == 0 {
			out <- os.Stdin
		}
		for _, fname := range files {
			if handler, err := os.Open(fname); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s\n", fname, err.Error())
			} else {
				out <- handler
			}
		}
	}
}

// FileScanner reads all the lines of each received file, and forwards them as FileLine
// instances to the next pipeline stage.
func FileScanner(in <-chan *os.File, out chan<- FileLine) {
	for f := range in {
		var fileName string
		if f != os.Stdin {
			fileName = f.Name()
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			out <- FileLine{
				FileName: fileName,
				Line:     scanner.Text(),
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "error reading %s: %s\n", f.Name(), err.Error())
		}
		if f != os.Stdin {
			f.Close()
		}
	}
}

// MatchFilterProvider instantiates a function that filters out the received FileLine
// instances that do not match the provided regexPattern.
// If the provided pattern can't compile as a Golang standard regular expression,
// this provider returns an error, aborting the minigrep pipeline creation.
func MatchFilterProvider(regexPattern string) pipe.MiddleProvider[FileLine, FileLine] {
	return func() (pipe.MiddleFunc[FileLine, FileLine], error) {
		matcher, err := regexp.Compile(regexPattern)
		if err != nil {
			return nil, fmt.Errorf("can't parse pattern %q as regular expression: %w", regexPattern, err)
		}
		return func(in <-chan FileLine, out chan<- FileLine) {
			for line := range in {
				if matcher.MatchString(line.Line) {
					out <- line
				}
			}
		}, nil
	}
}

// Printer just prints each received FileLine to the standard output.
func Printer(in <-chan FileLine) {
	for l := range in {
		if l.FileName != "" {
			fmt.Printf("%s:", l.FileName)
		}
		fmt.Printf("%s\n", l.Line)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: minigrep pattern [file ...]")
		os.Exit(2)
	}

	// Create the graph builder, and add the nodes.
	// AddStart, AddMiddle, AddFinal let specifying nodes that can't report
	// any error.
	// AddMiddleProvider specifies a node that can return an error and interrupt
	// the pipeline creation, if the user provides a wrong regular expression pattern.
	builder := pipe.NewBuilder(&MiniGrepNodes{})
	pipe.AddStart(builder, finderPtr, FileFinder(os.Args[2:]))
	pipe.AddMiddle(builder, scannerPtr, FileScanner)
	pipe.AddFinal(builder, printerPtr, Printer)
	pipe.AddMiddleProvider(builder, matcherPtr, MatchFilterProvider(os.Args[1]))

	runner, err := builder.Build()
	if err != nil {
		log.Fatal("minigrep:", err.Error())
	}

	// start the pipeline runner in background
	runner.Start()

	// wait until the pipeline has processed all the input
	<-runner.Done()
}
