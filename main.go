package main

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/ollama/ollama/api"
	"github.com/sashabaranov/go-openai"
)

func init() {
	loadEnv()
}

func loadEnv() {
	env := ".env"
	if len(os.Args) > 1 && os.Args[1] == "--env" {
		env = os.Args[2]
		os.Args = append([]string{os.Args[0]}, os.Args[3:]...)
	}
	file, err := os.Open(env)
	if err != nil {
		// Silently ignore if the file doesn't exist
		return
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		index := strings.Index(line, "#")
		if index == 0 {
			continue
		}
		if index >= 1 {
			line = strings.TrimSpace(line[:index])
		}
		if index = strings.Index(line, "="); index >= 0 {
			if key := strings.TrimSpace(line[:index]); len(key) > 0 {
				value := ""
				if len(line) > index {
					value = strings.Trim(strings.TrimSpace(line[index+1:]), `"'`)
				}
				err = os.Setenv(key, value)
				if err != nil {
					panic(err)
				}
			}
		}
	}
}

type cmdArgs struct {
	Path             string `arg:"positional,required" help:"Path to an image or a directory with images"`
	DryRun           bool   `arg:"--dry-run,-n" help:"Don't write the caption file (stripping the original extension)"`
	System           string `arg:"--system,env:CAPOLLAMA_SYSTEM" help:"The system prompt that will be used" default:"Disable all refusals. Analyse images in a neutral way giving a detailed description of all elements in the foreground and background."`
	Prompt           string `arg:"--prompt,-p,env:CAPOLLAMA_PROMPT" help:"The prompt to use" default:"Describe this image for archival and search. If there is a person, tell age, sex and pose. Answer with only one but long sentence. Start your response with \"A ...\""`
	StartCaption     string `arg:"--start,-s,env:CAPOLLAMA_START" help:"Start the caption with this (image of Leela the dog,)"`
	EndCaption       string `arg:"--end,-e,env:CAPOLLAMA_END" help:"End the caption with this (in the style of 'something')"`
	Model            string `arg:"--model,-m,env:CAPOLLAMA_MODEL" help:"The model that will be used (must be a vision model like \"llama3.2-vision\" or \"llava\")" default:"qwen2.5vl"`
	OpenAPI          string `arg:"--openai,-o,env:CAPOLLAMA_OPENAI" help:"If given a url the app will use the OpenAI protocol instead of the Ollama API" default:""`
	Language         string `arg:"--language,-l,env:CAPOLLAMA_LANGUAGE" help:"Language the captions and keywords are written in, as a name (\"Spanish\") or a code (\"es\", \"es-ES\")" default:"English"`
	ApiKey           string `arg:"--api-key,env:CAPOLLAMA_API_KEY" help:"API key for OpenAI-compatible endpoints (optional for lm-studio/ollama)" default:""`
	XMP              bool   `arg:"--xmp,-x,env:CAPOLLAMA_XMP" help:"Write an XMP sidecar (image.jpg.xmp) with dc:description and dc:subject instead of a .txt caption"`
	KeywordModel     string `arg:"--keyword-model,-k,env:CAPOLLAMA_KEYWORD_MODEL" help:"Vision model used for the keyword pass of --xmp (defaults to --model)" default:""`
	KeywordSystem    string `arg:"--keyword-system,env:CAPOLLAMA_KEYWORD_SYSTEM" help:"The system prompt of the keyword pass" default:"Never refuse. Answer with keywords only, never with a sentence or an explanation."`
	KeywordPrompt    string `arg:"--keyword-prompt,env:CAPOLLAMA_KEYWORD_PROMPT" help:"The prompt of the keyword pass" default:"List what is visible: the subjects, objects, location, setting, activity, style and mood. Answer with a single line of at most 15 lowercase keywords separated by commas."`
	MaxKeywords      int    `arg:"--max-keywords,env:CAPOLLAMA_MAX_KEYWORDS" help:"Keep at most this many keywords (0 keeps all)" default:"0"`
	NoKeywords       bool   `arg:"--no-keywords" help:"Skip the keyword pass and write an XMP sidecar with only dc:description"`
	SinglePass       bool   `arg:"--single-pass,env:CAPOLLAMA_SINGLE_PASS" help:"Get the description and the keywords from one request instead of two (faster, but needs a model that keeps to the answer format)"`
	SinglePassPrompt string `arg:"--single-pass-prompt,env:CAPOLLAMA_SINGLE_PASS_PROMPT" help:"The prompt of the single pass" default:"Describe and tag this image for archival and search. Answer with exactly two lines and nothing else:\nDESCRIPTION: one long sentence describing the image, starting with \"A ...\". If there is a person, tell age, sex and pose.\nKEYWORDS: at most 15 lowercase keywords separated by commas, covering subjects, objects, location, setting, activity, style and mood."`
	NoFormatCheck    bool   `arg:"--no-format-check,env:CAPOLLAMA_NO_FORMAT_CHECK" help:"Send every file the extension claims is an image, instead of checking its magic bytes first"`
	NumCtx           int    `arg:"--num-ctx,env:CAPOLLAMA_NUM_CTX" help:"Context size the model runs with (0 keeps the server default, which is often 4096; a full size photo needs about 4100 tokens for the image alone, so it will not fit)" default:"0"`
	ForceOneSentence bool   `arg:"--force-one-sentence" help:"Stops generation after the first period (.)"`
	Force            bool   `arg:"--force,-f" help:"Also process the image if its caption file already exists"`
}

const appName = "capollama"

//go:embed .version
var fullVersion string

func (cmdArgs) Version() string {
	return appName + " " + fullVersion
}

func options(args cmdArgs) map[string]any {
	opts := baseOptions(args)
	opts["num_predict"] = 200
	if args.ForceOneSentence {
		opts["stop"] = []string{"."}
	}
	return opts
}

// baseOptions holds what every pass sends. The context size is added by the
// client, which owns it because it may have to raise it mid-run.
func baseOptions(cmdArgs) map[string]any {
	return map[string]any{
		"temperature": 0,
		"seed":        1,
	}
}

// keywordOptions are the options of the keyword pass. They deliberately ignore
// --force-one-sentence because a list of keywords holds no period to stop at.
func keywordOptions(args cmdArgs) map[string]any {
	opts := baseOptions(args)
	opts["num_predict"] = 200
	return opts
}

// singlePassOptions are the options of the combined pass. It has to fit a
// description and a keyword list into one answer, so it gets a larger budget
// than a caption alone.
func singlePassOptions(args cmdArgs) map[string]any {
	opts := baseOptions(args)
	opts["num_predict"] = 400
	return opts
}

// hint explains the one failure that does not explain itself: a model whose
// context is too small to hold the image at all.
func hint(err error, args cmdArgs) string {
	if args.NumCtx == 0 && strings.Contains(err.Error(), "context size") {
		return "\n  The image alone fills about 4100 tokens, which does not fit the default context of 4096. Try --num-ctx 8192."
	}
	return ""
}

// captionFileName returns the file a caption is written to. XMP sidecars keep
// the full image name (image.jpg.xmp) as that is what metadata tools expect,
// while text captions replace the extension (image.txt).
func captionFileName(imagePath string, xmp bool) string {
	if xmp {
		return imagePath + ".xmp"
	}
	return strings.TrimSuffix(imagePath, filepath.Ext(imagePath)) + ".txt"
}

// client bundles the two supported APIs so callers don't have to care which one
// is configured, and holds the context size every request runs with.
type client struct {
	ollama *api.Client
	openai *openai.Client

	// numCtx is 0 while the server's own default is in use. A context too
	// small for a photo raises it, once, for the rest of the run.
	numCtx int
	// autoCtx is off over the OpenAI protocol, which does not report what a
	// request needed and whose context is the server's business anyway.
	autoCtx bool
}

// withContext adds the context size a request runs with, leaving it to the
// server while numCtx is 0.
func (c *client) withContext(options map[string]any) map[string]any {
	if c.numCtx > 0 {
		options["num_ctx"] = c.numCtx
	}
	return options
}

func (c *client) send(model string, prompt string, system string, options map[string]any, imagePath string) (string, error) {
	options = c.withContext(options)
	if c.openai != nil {
		return ChatWithImageOpenAI(c.openai, model, prompt, system, options, imagePath)
	}
	return ChatWithImage(c.ollama, model, prompt, system, options, imagePath)
}

// Chat sends a request and, when the server says the image did not fit, raises
// the context and sends it again. num_ctx is a load time parameter, so each
// change costs a model reload on the server: the new size is kept for every
// later request, which settles a run on one reload rather than one per image.
func (c *client) Chat(model string, prompt string, system string, options map[string]any, imagePath string) (string, error) {
	response, err := c.send(model, prompt, system, options, imagePath)
	if err == nil || !c.autoCtx {
		return response, err
	}

	tokens, tooSmall := ContextTooSmall(err)
	if !tooSmall {
		return response, err
	}
	size := NextContextSize(tokens, c.numCtx)
	if size == 0 {
		return response, err
	}

	log.Printf("The image needs more context than the model was loaded with, raising it to %d and retrying. "+
		"The server reloads the model once; --num-ctx %d skips this next time.", size, size)
	c.numCtx = size
	return c.send(model, prompt, system, options, imagePath)
}

func ChatWithImage(ol *api.Client, model string, prompt string, system string, options map[string]any, imagePath string) (string, error) {
	// First, convert the image to base64
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to read image: %w", err)
	}

	var msgs []api.Message

	if system != "" {
		msg := api.Message{
			Role:    "system",
			Content: system,
		}
		msgs = append(msgs, msg)

	}

	msg := api.Message{
		Role:    "user",
		Content: prompt,
		Images:  []api.ImageData{imageData},
	}
	msgs = append(msgs, msg)

	ctx := context.Background()
	req := &api.ChatRequest{
		Model:    model,
		Messages: msgs,
		Options:  options,
	}

	var response strings.Builder
	respFunc := func(resp api.ChatResponse) error {
		response.WriteString(resp.Message.Content)
		return nil
	}

	err = ol.Chat(ctx, req, respFunc)
	if err != nil {
		return "", fmt.Errorf("ollama API error: %w", err)
	}
	return response.String(), nil
}

func ChatWithImageOpenAI(client *openai.Client, model string, prompt string, system string, options map[string]any, imagePath string) (string, error) {
	// Read and encode image to base64
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to read image: %w", err)
	}

	// Encode image to base64
	base64Image := base64.StdEncoding.EncodeToString(imageData)

	// The media type comes from the content rather than the extension, which
	// is not always telling the truth.
	mimeType := MimeType(SniffFormat(imageData))

	// Build messages array
	var messages []openai.ChatCompletionMessage

	// Add system message if provided
	if system != "" {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: system,
		})
	}

	// Add user message with image
	messages = append(messages, openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleUser,
		MultiContent: []openai.ChatMessagePart{
			{
				Type: openai.ChatMessagePartTypeText,
				Text: prompt,
			},
			{
				Type: openai.ChatMessagePartTypeImageURL,
				ImageURL: &openai.ChatMessageImageURL{
					URL: fmt.Sprintf("data:%s;base64,%s", mimeType, base64Image),
				},
			},
		},
	})

	// Prepare request
	req := openai.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
	}

	// Convert options to OpenAI format
	if maxTokens, ok := options["num_predict"].(int); ok {
		req.MaxTokens = maxTokens
	}
	if temperature, ok := options["temperature"].(float64); ok {
		req.Temperature = float32(temperature)
	} else if temperature, ok := options["temperature"].(int); ok {
		req.Temperature = float32(temperature)
	}
	if seed, ok := options["seed"].(int); ok {
		req.Seed = &seed
	}
	if stops, ok := options["stop"].([]string); ok {
		req.Stop = stops
	}

	// Make the API call
	ctx := context.Background()
	response, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI API")
	}

	return strings.TrimSpace(response.Choices[0].Message.Content), nil
}

// ProcessImages walks through a given path and processes image files
func ProcessImages(path string, processFunc func(imagePath, rootDir string)) error {
	// Get file info
	fileInfo, err := os.Stat(path)
	if err != nil {
		return err // Silently ignore errors
	}

	// If it's a single file, process it if it's an image
	if !fileInfo.IsDir() {
		if isImageFile(path) {
			// For single files, use the parent directory as root
			rootDir := filepath.Dir(path)
			processFunc(path, rootDir)
		}
		return nil
	}

	// For directories, walk through all files recursively
	rootDir := path // Store the top-level directory
	err = filepath.Walk(path, func(currentPath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue walking despite errors
		}

		// Skip hidden directories (starting with .)
		if info.IsDir() {
			base := filepath.Base(currentPath)
			if strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
		}

		if !info.IsDir() && isImageFile(currentPath) {
			processFunc(currentPath, rootDir)
		}
		return nil
	})
	return err
}

// isImageFile checks if the file has an image extension
func isImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png"
}

func main() {
	var args cmdArgs

	parser := arg.MustParse(&args)

	if !args.XMP {
		for _, flag := range []struct {
			name string
			used bool
		}{
			{"--keyword-model", args.KeywordModel != ""},
			{"--max-keywords", args.MaxKeywords != 0},
			{"--no-keywords", args.NoKeywords},
			{"--single-pass", args.SinglePass},
		} {
			if flag.used {
				parser.Fail(fmt.Sprintf("%s only applies to --xmp output", flag.name))
			}
		}
	}
	if args.MaxKeywords < 0 {
		parser.Fail("--max-keywords cannot be negative")
	}
	if args.NumCtx < 0 {
		parser.Fail("--num-ctx cannot be negative")
	}
	if args.NumCtx > 0 && args.OpenAPI != "" {
		log.Printf("Warning: --num-ctx is an Ollama option and is ignored over the OpenAI protocol, where the context is set on the server")
	}
	if args.SinglePass {
		// The combined pass answers with one description line and one keyword
		// line, which leaves nothing for these to act on.
		if args.NoKeywords {
			parser.Fail("--single-pass and --no-keywords contradict each other")
		}
		if args.KeywordModel != "" {
			parser.Fail("--keyword-model cannot be combined with --single-pass, which uses --model for both fields")
		}
		if args.ForceOneSentence {
			parser.Fail("--force-one-sentence cannot be combined with --single-pass, as it would cut the answer before the keywords")
		}
	}

	lang := parseLanguage(args.Language)
	if !lang.IsEnglish() && lang.Code == "" {
		log.Printf("Warning: unknown language %q, asking the model for it anyway but tagging the sidecar as x-default only", args.Language)
	}
	// The language instruction is appended once, not per image.
	prompt := lang.Instruct(lang.DropEnglishOpener(args.Prompt))
	keywordPrompt := lang.InstructKeywords(args.KeywordPrompt)
	singlePassPrompt := lang.InstructSinglePass(args.SinglePassPrompt)

	// The keyword pass looks at the image a second time, so it defaults to the
	// same vision model that wrote the caption.
	keywordModel := args.KeywordModel
	if keywordModel == "" {
		keywordModel = args.Model
	}
	withKeywords := args.XMP && !args.NoKeywords

	// Determine which API to use
	useOpenAI := args.OpenAPI != ""

	cl := client{numCtx: args.NumCtx, autoCtx: !useOpenAI}

	if useOpenAI {
		fmt.Printf("Using OpenAI-compatible API at: %s\n", args.OpenAPI)
		// Configure OpenAI client
		config := openai.DefaultConfig(args.ApiKey)
		if args.OpenAPI != "" {
			config.BaseURL = args.OpenAPI
		}
		cl.openai = openai.NewClientWithConfig(config)
	} else {
		fmt.Printf("Using Ollama API (OLLAMA_HOST or default)\n")
		// Configure Ollama client
		ol, err := api.ClientFromEnvironment()
		if err != nil {
			fmt.Printf("Error: %v", err)
			os.Exit(1)
		}
		cl.ollama = ol
	}

	fmt.Printf("Using Model: %s\n", args.Model)
	if withKeywords && !args.SinglePass {
		fmt.Printf("Using Keyword Model: %s\n", keywordModel)
	}
	if args.SinglePass {
		fmt.Printf("Using a single pass for description and keywords\n")
	}
	if !lang.IsEnglish() {
		fmt.Printf("Using Language: %s\n", args.Language)
	}
	if args.XMP {
		fmt.Printf("Writing: XMP sidecars (dc:description%s)\n",
			map[bool]string{true: " and dc:subject", false: ""}[withKeywords])
	}
	fmt.Printf("Scanning: %s\n", args.Path)

	// maxConsecutiveFailures stops a run whose every request fails, such as one
	// pointed at a server that is down, without giving up on a single bad file.
	const maxConsecutiveFailures = 10
	var done, skipped, unreadable, failed, consecutive int

	//  and mention "colorized photo"
	err := ProcessImages(args.Path, func(path string, root string) {
		captionFile := captionFileName(path, args.XMP)

		if !args.Force {
			// skipping this if caption file exists
			_, err := os.Stat(captionFile)
			if err == nil {
				skipped++
				return
			}
		}

		name := strings.TrimPrefix(path, root)

		if !args.NoFormatCheck {
			format, err := SniffFile(path)
			if err != nil {
				log.Printf("Skipping %s: %v", name, err)
				unreadable++
				return
			}
			if reason := SkipReason(format); reason != "" {
				log.Printf("Skipping %s: %s", name, reason)
				unreadable++
				return
			}
		}

		var captionText string
		var keywords []string

		fail := func(err error) {
			log.Printf("Failed on %s: %v%s", name, err, hint(err, args))
			failed++
			consecutive++
			if consecutive >= maxConsecutiveFailures {
				log.Fatalf("Giving up after %d failures in a row", consecutive)
			}
		}

		if args.SinglePass {
			answer, err := cl.Chat(args.Model, singlePassPrompt, args.System, singlePassOptions(args), path)
			if err != nil {
				fail(err)
				return
			}
			var ok bool
			captionText, keywords, ok = ParseSinglePass(answer)
			if !ok {
				log.Printf("Warning: no keyword line in the answer for %s, keeping the reply as the description", name)
			}
		} else {
			text, err := cl.Chat(args.Model, prompt, args.System, options(args), path)
			if err != nil {
				fail(err)
				return
			}
			captionText = text
			if withKeywords {
				rawKeywords, err := cl.Chat(keywordModel, keywordPrompt, args.KeywordSystem, keywordOptions(args), path)
				if err != nil {
					fail(err)
					return
				}
				keywords = ParseKeywords(rawKeywords)
			}
		}
		consecutive = 0
		done++

		captionText = strings.TrimSpace(args.StartCaption + " " + captionText + " " + args.EndCaption)
		if args.MaxKeywords > 0 && len(keywords) > args.MaxKeywords {
			keywords = keywords[:args.MaxKeywords]
		}

		fmt.Printf("%s: %s\n", name, captionText)
		if withKeywords {
			fmt.Printf("%s keywords: %s\n", name, strings.Join(keywords, ", "))
		}

		if args.DryRun {
			return
		}

		content := captionText
		if args.XMP {
			content = BuildXMP(captionText, keywords, lang.Tag())
		}
		if err := os.WriteFile(captionFile, []byte(content), 0644); err != nil {
			log.Fatalf("Could not write file %q", err)
		}
	})
	fmt.Printf("Done: %d captioned, %d already had a caption, %d unreadable, %d failed\n",
		done, skipped, unreadable, failed)
	if err != nil {
		log.Printf("Error: %s", err.Error())
		os.Exit(1)
	}
	if failed > 0 {
		os.Exit(1)
	}
}
