# Capollama

Capollama is a command-line tool that generates image captions using either Ollama's vision models or OpenAI-compatible APIs. It can process single images or entire directories, optionally saving the captions as text files alongside the images.

## Features

- Process single images or recursively scan directories
- Support for JPG, JPEG, and PNG formats
- Customizable caption prompts
- Optional prefix and suffix for captions
- Automatic caption file generation with dry-run option
- **Optional XMP sidecar output with `dc:description` and `dc:subject` keywords**
- Configurable vision model selection
- **Dual API support: Ollama and OpenAI-compatible endpoints**
- Compatible with LM Studio and Ollama's OpenAI API
- Skips hidden directories (starting with '.')
- Skip existing captions by default with force option available

## Prerequisites

**For Ollama API:**
- [Ollama](https://ollama.ai/) installed and running as server
- A vision-capable model pulled (like `llava` or `llama3.2-vision`)

**For OpenAI-compatible APIs:**
- A running OpenAI-compatible server such as:
  - [LM Studio](https://lmstudio.ai/) with a vision model loaded
  - Ollama with OpenAI API compatibility enabled
  - OpenAI API or other compatible services

## Installation precompiled binary

Install from [Release Page](https://github.com/oderwat/capollama/releases/latest)

### Installation from source (needs Go >=1.22 installed)

```bash
go install github.com/oderwat/capollama@latest
```

## Usage

**Basic usage with Ollama (default):**
```bash
capollama path/to/image.jpg
```

**Using OpenAI-compatible API (LM Studio):**
```bash
capollama --openai http://localhost:1234/v1 path/to/image.jpg
```

**Using Ollama's OpenAI API:**
```bash
capollama --openai http://localhost:11434/v1 path/to/image.jpg
```

**Process a directory:**
```bash
capollama path/to/images/directory
```

### Command Line Arguments

```
Usage: capollama [--dry-run] [--system SYSTEM] [--prompt PROMPT] [--start START] [--end END] [--model MODEL] [--openai OPENAI] [--api-key API-KEY] [--xmp] [--keyword-model KEYWORD-MODEL] [--keyword-system KEYWORD-SYSTEM] [--keyword-prompt KEYWORD-PROMPT] [--max-keywords MAX-KEYWORDS] [--no-keywords] [--force-one-sentence] [--force] PATH

Positional arguments:
  PATH                   Path to an image or a directory with images

Options:
  --dry-run, -n          Don't write the caption file (stripping the original extension)
  --system SYSTEM        The system prompt that will be used [env: CAPOLLAMA_SYSTEM]
  --prompt PROMPT, -p PROMPT
                         The prompt to use [env: CAPOLLAMA_PROMPT]
  --start START, -s START
                         Start the caption with this (image of Leela the dog,) [env: CAPOLLAMA_START]
  --end END, -e END      End the caption with this (in the style of 'something') [env: CAPOLLAMA_END]
  --model MODEL, -m MODEL
                         The model that will be used (must be a vision model like "llama3.2-vision" or "llava") [default: qwen2.5vl, env: CAPOLLAMA_MODEL]
  --openai OPENAI, -o OPENAI
                         If given a url the app will use the OpenAI protocol instead of the Ollama API [env: CAPOLLAMA_OPENAI]
  --api-key API-KEY      API key for OpenAI-compatible endpoints (optional for lm-studio/ollama) [env: CAPOLLAMA_API_KEY]
  --xmp, -x              Write an XMP sidecar (image.jpg.xmp) with dc:description and dc:subject instead of a .txt caption [env: CAPOLLAMA_XMP]
  --keyword-model KEYWORD-MODEL, -k KEYWORD-MODEL
                         Vision model used for the keyword pass of --xmp (defaults to --model) [env: CAPOLLAMA_KEYWORD_MODEL]
  --keyword-system KEYWORD-SYSTEM
                         The system prompt of the keyword pass [env: CAPOLLAMA_KEYWORD_SYSTEM]
  --keyword-prompt KEYWORD-PROMPT
                         The prompt of the keyword pass [env: CAPOLLAMA_KEYWORD_PROMPT]
  --max-keywords MAX-KEYWORDS
                         Keep at most this many keywords (0 keeps all) [default: 0, env: CAPOLLAMA_MAX_KEYWORDS]
  --no-keywords          Skip the keyword pass and write an XMP sidecar with only dc:description
  --force-one-sentence   Stops generation after the first period (.)
  --force, -f            Also process the image if its caption file already exists
  --help, -h             display this help and exit
  --version              display version and exit

```

### Examples

Generate a caption for a single image (will save as .txt):
```bash
capollama image.jpg
```

Process all images in a directory without writing files (dry run):
```bash
capollama --dry-run path/to/images/
```

Force regeneration of all captions, even if they exist:
```bash
capollama --force path/to/images/
```

Use a custom prompt and model:
```bash
capollama --prompt "Describe this image briefly" --model llava image.jpg
```

Add prefix and suffix to captions:
```bash
capollama --start "A photo showing" --end "in vintage style" image.jpg
```

Write XMP sidecars instead of .txt captions:
```bash
capollama --xmp path/to/images/
```

Use a different vision model for the keyword pass and cap the tag count:
```bash
capollama --xmp --keyword-model llama3.2-vision --max-keywords 10 path/to/images/
```

## XMP output

With `--xmp`, capollama runs the vision model a second time over the same image
with a keyword prompt, and writes an XMP sidecar next to the image instead of a
`.txt` file. The sidecar keeps the full image name, which is the convention
metadata tools such as exiftool, digiKam and Lightroom expect:

```
path/to/image.jpg
path/to/image.jpg.xmp
```

The second pass uses `--model` as well, so no extra model is needed.
`--keyword-model` overrides it when you want a different (vision) model for
tagging, `--max-keywords` caps the list, and `--no-keywords` skips the pass
entirely and writes only the description.


The generated sidecar:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/" x:xmptk="capollama 0.5.0">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about=""
    xmlns:dc="http://purl.org/dc/elements/1.1/">
   <dc:description>
    <rdf:Alt>
     <rdf:li xml:lang="x-default">A fluffy orange cat sitting on a sunny wooden deck outdoors.</rdf:li>
    </rdf:Alt>
   </dc:description>
   <dc:subject>
    <rdf:Bag>
     <rdf:li>cat</rdf:li>
     <rdf:li>outdoor</rdf:li>
     <rdf:li>sunny</rdf:li>
    </rdf:Bag>
   </dc:subject>
  </rdf:Description>
 </rdf:RDF>
</x:xmpmeta>
```

Which reads back as `XMP-dc:Description` and `XMP-dc:Subject`:

```bash
exiftool -XMP-dc:Description -XMP-dc:Subject image.jpg.xmp
```

To burn the sidecar into the image file itself:

```bash
exiftool -tagsfromfile image.jpg.xmp -all:all image.jpg
```

## Output

By default:
- Captions are printed to stdout in the format:
  ```
  path/to/image.jpg: A detailed caption generated by the model
  ```
- Caption files are automatically created alongside images:
  ```
  path/to/image.jpg
  path/to/image.txt
  ```
- With `--xmp` the sidecar `path/to/image.jpg.xmp` is written instead
- Existing caption files are skipped unless `--force` is used (the check looks at
  the file that would be written, so `.txt` and `.xmp` runs are independent)
- Use `--dry-run` to prevent writing caption files

## License

[MIT License](LICENSE.txt)

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

## Acknowledgments

This tool uses:
- [Ollama](https://ollama.ai/) for local LLM inference
- [go-arg](https://github.com/alexflint/go-arg) for argument parsing