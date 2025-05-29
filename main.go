package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/mcncl/gotyper/internal/analyzer"
	"github.com/mcncl/gotyper/internal/errors"
	"github.com/mcncl/gotyper/internal/formatter"
	"github.com/mcncl/gotyper/internal/generator"
	"github.com/mcncl/gotyper/internal/models"
	"github.com/mcncl/gotyper/internal/parser"
)

// CLI defines the command-line interface
var CLI struct {
	Input       string `help:"Path to input JSON file. If not specified, reads from stdin." short:"i" type:"path"`
	Output      string `help:"Path to output Go file. If not specified, writes to stdout." short:"o" type:"path"`
	Package     string `help:"Package name for generated code." short:"p" default:"main"`
	RootName    string `help:"Name for the root struct." short:"r" default:"RootType"`
	Format      bool   `help:"Format the output code according to Go standards." short:"f" default:"true"`
	Debug       bool   `help:"Enable debug logging." short:"d"`
	Version     bool   `help:"Show version information." short:"v"`
	Interactive bool   `help:"Run in interactive mode, allowing direct JSON input with Ctrl+D to process." short:"I"`
}

// Context holds the runtime context
type Context struct {
	Debug bool
}

// Version information
const (
	Version = "0.1.0"
)

func main() {
	// Parse CLI arguments with Kong
	parser := kong.Must(&CLI,
		kong.Name("gotyper"),
		kong.Description("A tool to convert JSON to Go structs"),
		kong.UsageOnError(),
	)

	// Check if no arguments provided and set interactive mode by default
	if len(os.Args) == 1 {
		CLI.Interactive = true
	}

	// Parse the command line arguments
	ctx, err := parser.Parse(os.Args[1:])
	if err != nil {
		// If there's an error parsing arguments, the usage will already be shown by kong.UsageOnError()
		os.Exit(1)
	}

	// Show version and exit if requested
	if CLI.Version {
		fmt.Printf("gotyper version %s\n", Version)
		return
	}

	err = run(&Context{Debug: CLI.Debug})
	if err != nil {
		// Use our custom error handling to provide user-friendly error messages
		fmt.Fprintf(os.Stderr, "%s\n", errors.UserFriendlyError(err))
		
		// Show help on error
		fmt.Fprintf(os.Stderr, "\nFor help, run: gotyper --help\n")
		
		os.Exit(1)
	}

	// If we have a command with a Run function, call it
	if ctx.Command() != "" {
		err = ctx.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", errors.UserFriendlyError(err))
			os.Exit(1)
		}
	}
}

// run executes the main program logic
func run(ctx *Context) error {
	// 1. Parse JSON input
	ir, err := parseInput()
	if err != nil {
		// Error is already wrapped by parseInput
		return err
	}

	// 2. Analyze JSON structure
	analyzerInst := analyzer.NewAnalyzer()
	analysisResult, err := analyzerInst.Analyze(ir, CLI.RootName)
	if err != nil {
		return errors.NewAnalysisError("failed to analyze JSON structure", err)
	}

	// 3. Generate Go structs
	generatorInst := generator.NewGenerator()
	code, err := generatorInst.GenerateStructs(analysisResult, CLI.Package)
	if err != nil {
		return errors.NewGenerateError("failed to generate Go structs", err)
	}

	// 4. Format the code if requested
	if CLI.Format {
		formatterInst := formatter.NewFormatter()
		code, err = formatterInst.Format(code)
		if err != nil {
			return errors.NewFormatError("failed to format Go code", err)
		}
	}

	// 5. Output the result
	return writeOutput(code)
}

// parseInput reads JSON from file or stdin
func parseInput() (models.IntermediateRepresentation, error) {
	if CLI.Input != "" {
		// Parse from file
		return parser.ParseFile(CLI.Input)
	}

	// Check if stdin has data
	stdinInfo, err := os.Stdin.Stat()
	if err != nil {
		return models.IntermediateRepresentation{}, errors.NewInputError("failed to access stdin", err)
	}

	// Interactive mode or piped input
	if (stdinInfo.Mode() & os.ModeCharDevice) != 0 {
		// Terminal is interactive (not piped)
		if CLI.Interactive {
			// Interactive mode
			return readInteractiveInput()
		}
		// No data provided on stdin and not in interactive mode
		return models.IntermediateRepresentation{}, errors.NewInputError("no input provided", errors.ErrNoInput)
	}

	// Read from stdin (piped input)
	jsonData, err := io.ReadAll(os.Stdin)
	if err != nil {
		return models.IntermediateRepresentation{}, errors.NewInputError("failed to read from stdin", err)
	}

	if len(jsonData) == 0 {
		return models.IntermediateRepresentation{}, errors.NewInputError("empty input received from stdin", errors.ErrEmptyInput)
	}

	return parser.ParseString(string(jsonData))
}

// writeOutput writes code to file or stdout
func writeOutput(code string) error {
	if CLI.Output != "" {
		// Write to file
		err := os.WriteFile(CLI.Output, []byte(code), 0644)
		if err != nil {
			return errors.NewOutputError(fmt.Sprintf("failed to write to file '%s'", CLI.Output), err)
		}
		fmt.Fprintf(os.Stderr, "Generated Go code written to %s\n", CLI.Output)
		return nil
	}

	// Write to stdout
	_, err := fmt.Println(strings.TrimSpace(code))
	if err != nil {
		return errors.NewOutputError("failed to write to stdout", err)
	}
	return nil
}

// readInteractiveInput provides an interactive mode for users to paste JSON
// and signal completion with Ctrl+D (EOF)
func readInteractiveInput() (models.IntermediateRepresentation, error) {
	fmt.Fprintln(os.Stderr, "GoTyper Interactive Mode")
	fmt.Fprintln(os.Stderr, "Paste your JSON below and press Ctrl+D (or Ctrl+Z on Windows) when done:")

	// Read all input until EOF (Ctrl+D)
	reader := bufio.NewReader(os.Stdin)
	var jsonBuilder strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			// End of input
			break
		}
		if err != nil {
			return models.IntermediateRepresentation{}, errors.NewInputError("error reading input", err)
		}
		jsonBuilder.WriteString(line)
	}

	jsonData := jsonBuilder.String()
	if len(jsonData) == 0 {
		return models.IntermediateRepresentation{}, errors.NewInputError("empty input received", errors.ErrEmptyInput)
	}

	fmt.Fprintln(os.Stderr, "\nProcessing JSON...")
	return parser.ParseString(jsonData)
}
