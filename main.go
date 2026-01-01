package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/mcncl/gotyper/internal/analyzer"
	"github.com/mcncl/gotyper/internal/config"
	"github.com/mcncl/gotyper/internal/errors"
	"github.com/mcncl/gotyper/internal/formatter"
	"github.com/mcncl/gotyper/internal/generator"
	"github.com/mcncl/gotyper/internal/models"
	"github.com/mcncl/gotyper/internal/parser"
	"github.com/mcncl/gotyper/internal/schema"
)

// CLI defines the command-line interface
var CLI struct {
	Input       string `help:"Path to input JSON file. If not specified, reads from stdin." short:"i" type:"path"`
	URL         string `help:"URL to fetch JSON from. Supports http and https." short:"u"`
	Schema      string `help:"Path or URL to JSON Schema file. Generates structs from schema instead of sample JSON." short:"s"`
	Output      string `help:"Path to output Go file. If not specified, writes to stdout." short:"o" type:"path"`
	Package     string `help:"Package name for generated code." short:"p" default:"main"`
	RootName    string `help:"Name for the root struct." short:"r" default:"RootType"`
	Config      string `help:"Path to config file. If not specified, searches for .gotyper.yml in current and parent directories." short:"c" type:"path"`
	Format      bool   `help:"Format the output code according to Go standards." short:"f" default:"true"`
	Debug       bool   `help:"Enable debug logging." short:"d"`
	Version     bool   `help:"Show version information." short:"v"`
	Interactive bool   `help:"Run in interactive mode, allowing direct JSON input with Ctrl+D to process." short:"I"`
}

// Context holds the runtime context
type Context struct {
	Debug  bool
	Config *config.Config
}

// Version information
const (
	Version = "dev"
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
		// Explicitly ensure default package name is set to 'main'
		if CLI.Package == "" {
			CLI.Package = "main"
		}
		// When no args provided, run directly without parsing arguments
		ctx, err := createContext()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", errors.UserFriendlyError(err))
			os.Exit(1)
		}
		err = run(ctx)
		if err != nil {
			// Use our custom error handling to provide user-friendly error messages
			fmt.Fprintf(os.Stderr, "%s\n", errors.UserFriendlyError(err))

			// Show help on error
			fmt.Fprintf(os.Stderr, "\nFor help, run: gotyper --help\n")

			os.Exit(1)
		}
		return
	}

	// Parse the command line arguments
	_, err := parser.Parse(os.Args[1:])
	if err != nil {
		// If there's an error parsing arguments, the usage will already be shown by kong.UsageOnError()
		os.Exit(1)
	}

	// Show version and exit if requested
	if CLI.Version {
		fmt.Printf("gotyper version %s\n", Version)
		return
	}

	ctx, err := createContext()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", errors.UserFriendlyError(err))
		os.Exit(1)
	}

	err = run(ctx)
	if err != nil {
		// Use our custom error handling to provide user-friendly error messages
		fmt.Fprintf(os.Stderr, "%s\n", errors.UserFriendlyError(err))

		// Show help on error
		fmt.Fprintf(os.Stderr, "\nFor help, run: gotyper --help\n")

		os.Exit(1)
	}

	// Note: Kong's ctx.Run() is not used in this application since we handle everything in run()
}

// createContext loads configuration and creates a runtime context
func createContext() (*Context, error) {
	// Determine config file path
	configPath := CLI.Config
	if configPath == "" {
		// Search for config file automatically
		configPath = config.FindConfigFile()
	}

	// Load configuration with CLI precedence
	cfg, err := config.LoadConfigWithCLI(configPath, CLI.Package, CLI.RootName, false)
	if err != nil {
		return nil, errors.NewInputError("failed to load configuration", err)
	}

	return &Context{
		Debug:  CLI.Debug,
		Config: cfg,
	}, nil
}

// run executes the main program logic
func run(ctx *Context) error {
	var analysisResult models.AnalysisResult
	var err error

	// Check if using JSON Schema mode or JSON sample mode
	if CLI.Schema != "" {
		// Schema mode: parse and convert JSON Schema
		analysisResult, err = parseSchema(ctx.Config.RootName)
		if err != nil {
			return err
		}
	} else {
		// JSON sample mode: parse and analyze JSON
		ir, err := parseInput()
		if err != nil {
			return err
		}

		analyzerInst := analyzer.NewAnalyzerWithConfig(ctx.Config)
		analysisResult, err = analyzerInst.Analyze(ir, ctx.Config.RootName)
		if err != nil {
			return errors.NewAnalysisError("failed to analyze JSON structure", err)
		}
	}

	// Generate Go structs
	generatorInst := generator.NewGenerator()
	code, err := generatorInst.GenerateStructs(analysisResult, ctx.Config.Package)
	if err != nil {
		return errors.NewGenerateError("failed to generate Go structs", err)
	}

	// Format the code if requested and enabled in config
	if CLI.Format && ctx.Config.Formatting.Enabled {
		formatterInst := formatter.NewFormatter()
		code, err = formatterInst.Format(code)
		if err != nil {
			return errors.NewFormatError("failed to format Go code", err)
		}
	}

	// Output the result
	return writeOutput(code)
}

// parseSchema reads and converts a JSON Schema from file or URL
func parseSchema(rootName string) (models.AnalysisResult, error) {
	// Check for conflicting input sources
	if CLI.Input != "" || CLI.URL != "" {
		return models.AnalysisResult{}, errors.NewInputError(
			"cannot specify --schema with --input or --url", nil)
	}

	var s *schema.Schema
	var err error

	// Check if schema is a URL
	schemaLower := strings.ToLower(CLI.Schema)
	if strings.HasPrefix(schemaLower, "http://") || strings.HasPrefix(schemaLower, "https://") {
		// Fetch schema from URL
		s, err = fetchSchemaFromURL(CLI.Schema)
		if err != nil {
			return models.AnalysisResult{}, err
		}
	} else {
		// Parse from file
		s, err = schema.ParseFile(CLI.Schema)
		if err != nil {
			return models.AnalysisResult{}, errors.NewInputError(
				fmt.Sprintf("failed to parse schema file: %s", CLI.Schema), err)
		}
	}

	// Convert schema to analysis result
	converter := schema.NewConverter(s)
	result, err := converter.Convert(rootName)
	if err != nil {
		return models.AnalysisResult{}, errors.NewAnalysisError(
			"failed to convert JSON Schema", err)
	}

	return result, nil
}

// fetchSchemaFromURL fetches a JSON Schema from a URL
func fetchSchemaFromURL(url string) (*schema.Schema, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, errors.NewInputError(fmt.Sprintf("invalid schema URL: %s", url), err)
	}

	req.Header.Set("Accept", "application/json, application/schema+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.NewInputError(fmt.Sprintf("failed to fetch schema from URL: %s", url), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.NewInputError(
			fmt.Sprintf("HTTP %d fetching schema from %s", resp.StatusCode, url), nil)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.NewInputError("failed to read schema response", err)
	}

	s, err := schema.ParseBytes(data)
	if err != nil {
		return nil, errors.NewInputError("failed to parse JSON Schema from URL", err)
	}

	return s, nil
}

// parseInput reads JSON from file, URL, or stdin
func parseInput() (models.IntermediateRepresentation, error) {
	// Check for conflicting input sources
	if CLI.Input != "" && CLI.URL != "" {
		return models.IntermediateRepresentation{}, errors.NewInputError("cannot specify both --input and --url", nil)
	}

	if CLI.Input != "" {
		// Parse from file
		return parser.ParseFile(CLI.Input)
	}

	if CLI.URL != "" {
		// Fetch and parse from URL
		return fetchFromURL(CLI.URL)
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
		err := os.WriteFile(CLI.Output, []byte(code), 0o644)
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

// fetchFromURL fetches JSON from a URL and parses it
func fetchFromURL(urlStr string) (models.IntermediateRepresentation, error) {
	// Validate URL scheme (case-insensitive)
	lowerURL := strings.ToLower(urlStr)
	if !strings.HasPrefix(lowerURL, "http://") && !strings.HasPrefix(lowerURL, "https://") {
		return models.IntermediateRepresentation{}, errors.NewInputError(
			fmt.Sprintf("invalid URL scheme: %s (must be http:// or https://)", urlStr), nil)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create request
	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return models.IntermediateRepresentation{}, errors.NewInputError(
			fmt.Sprintf("failed to create request for URL: %s", urlStr), err)
	}

	// Set headers to indicate we expect JSON
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "gotyper/"+Version)

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return models.IntermediateRepresentation{}, errors.NewInputError(
			fmt.Sprintf("failed to fetch URL: %s", urlStr), err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return models.IntermediateRepresentation{}, errors.NewInputError(
			fmt.Sprintf("HTTP request failed with status %d for URL: %s", resp.StatusCode, urlStr), nil)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.IntermediateRepresentation{}, errors.NewInputError(
			fmt.Sprintf("failed to read response body from URL: %s", urlStr), err)
	}

	if len(body) == 0 {
		return models.IntermediateRepresentation{}, errors.NewInputError(
			fmt.Sprintf("empty response from URL: %s", urlStr), errors.ErrEmptyInput)
	}

	return parser.ParseString(string(body))
}
