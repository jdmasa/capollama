# Capollama

[![CI](https://github.com/jdmasa/capollama/actions/workflows/ci.yml/badge.svg)](https://github.com/jdmasa/capollama/actions/workflows/ci.yml)

Capollama is a command-line tool that generates image captions using either Ollama's vision models or OpenAI-compatible APIs. It can process single images or entire directories, optionally saving the captions as text files alongside the images.

## Features

- Process single images or recursively scan directories
- Support for JPG, JPEG, and PNG formats
- Customizable caption prompts
- Captions and keywords in any language the model speaks
- Optional prefix and suffix for captions
- Automatic caption file generation with dry-run option
- **Optional XMP sidecar output with `dc:description` and `dc:subject` keywords**
- Configurable vision model selection
- **Dual API support: Ollama and OpenAI-compatible endpoints**
- Compatible with LM Studio and Ollama's OpenAI API
- Skips hidden directories (starting with '.')
- Checks the real image format by magic bytes, not by file extension
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
Usage: capollama [--dry-run] [--system SYSTEM] [--prompt PROMPT] [--start START] [--end END] [--model MODEL] [--openai OPENAI] [--language LANGUAGE] [--api-key API-KEY] [--xmp] [--keyword-model KEYWORD-MODEL] [--keyword-system KEYWORD-SYSTEM] [--keyword-prompt KEYWORD-PROMPT] [--max-keywords MAX-KEYWORDS] [--no-keywords] [--single-pass] [--single-pass-prompt SINGLE-PASS-PROMPT] [--no-format-check] [--force-one-sentence] [--force] PATH

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
  --language LANGUAGE, -l LANGUAGE
                         Language the captions and keywords are written in, as a name ("Spanish") or a code ("es", "es-ES") [default: English, env: CAPOLLAMA_LANGUAGE]
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
  --single-pass          Get the description and the keywords from one request instead of two (faster, but needs a model that keeps to the answer format) [env: CAPOLLAMA_SINGLE_PASS]
  --single-pass-prompt SINGLE-PASS-PROMPT
                         The prompt of the single pass [env: CAPOLLAMA_SINGLE_PASS_PROMPT]
  --no-format-check      Send every file the extension claims is an image, instead of checking its magic bytes first [env: CAPOLLAMA_NO_FORMAT_CHECK]
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

Get both fields from a single request, which is roughly twice as fast:
```bash
capollama --xmp --single-pass path/to/images/
```

## Image formats

Only JPEG and PNG can be sent to the vision APIs. Extensions lie about this more
often than you would think: phones and photo managers leave JPEG XL, HEIC and
WebP files behind under a `.jpg` name, and the API then answers `Failed to load
image or audio file`, which used to abort the whole run.

Every file is therefore identified by its magic bytes before a request is spent
on it, and one that cannot be read is reported and skipped while the run
continues:

```
Skipping /holiday.jpg: JPEG XL, which the vision API cannot read, despite the file name
Skipping /notes.png: not a JPEG or PNG
```

JPEG XL, HEIC, AVIF, WebP, GIF, BMP, TIFF and SVG are recognised by name so the
message tells you what the file really is. Convert them first, for example with
`sips -s format jpeg broken.jpg --out fixed.jpg` on macOS or `magick` elsewhere.
`--no-format-check` turns the check off for a backend that accepts more formats
than these two.

The check runs after the skip-existing test, so a file that already has a
caption costs nothing either way.

## Language

`--language` (or `CAPOLLAMA_LANGUAGE`) sets the language of both the caption and
the keywords. It takes a name or a BCP 47 code, so all of these are the same:

```bash
capollama --language Spanish image.jpg
capollama --language es image.jpg
capollama -l es-ES image.jpg
```

The instruction is appended to whichever prompt is in use, so it works with
`.txt` captions, the two pass XMP mode and `--single-pass` alike, and it applies
to your own `--prompt` as well. With `--single-pass` the model is told to keep
the `DESCRIPTION` and `KEYWORDS` labels in English; the parser also accepts the
usual translations of them in case it does not.

In an XMP sidecar the caption is then written twice, once as `x-default` for
readers that ignore languages and once tagged with the language, which exiftool
reports as `XMP-dc:Description-es`:

```xml
   <dc:description>
    <rdf:Alt>
     <rdf:li xml:lang="x-default">Un gato naranja sentado en una terraza de madera soleada.</rdf:li>
     <rdf:li xml:lang="es">Un gato naranja sentado en una terraza de madera soleada.</rdf:li>
    </rdf:Alt>
   </dc:description>
```

`dc:subject` carries no language qualifier, as XMP defines it as an unordered
bag of plain text. A language outside the built-in table is still passed to the
model by name, but the sidecar then stays `x-default` only and a warning says so.

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

### Single pass

`--single-pass` asks for both fields in one request instead of two, which
roughly halves the time per image. The model is asked to answer in two labelled
lines:

```
DESCRIPTION: A fluffy orange cat sitting on a sunny wooden deck outdoors.
KEYWORDS: cat, outdoor, sunny, deck
```

The parser finds those labels anywhere in the reply and also accepts `CAPTION:`,
`TAGS:`, markdown decoration and bullet lists, since models drift. If no keyword
label shows up at all, the whole reply is kept as the description and a warning
is printed, so a malformed answer never costs you the caption as well.

It is a trade: smaller vision models write a weaker caption when the same turn
also has to produce tags. Compare both on your own material before switching a
large archive over. `--single-pass-prompt` (or `CAPOLLAMA_SINGLE_PASS_PROMPT`)
replaces the combined prompt, and `--force-one-sentence` is refused in this mode
because its stop token would cut the answer before the keywords.

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