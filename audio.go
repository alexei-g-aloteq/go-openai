package openai

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// Whisper Defines the models provided by OpenAI to use when processing audio with OpenAI.
const (
	Whisper1 = "whisper-1"
)

// Response formats; Whisper uses AudioResponseFormatJSON by default.
type AudioResponseFormat string

const (
	AudioResponseFormatJSON AudioResponseFormat = "json"
	AudioResponseFormatSRT  AudioResponseFormat = "srt"
	AudioResponseFormatVTT  AudioResponseFormat = "vtt"
)

// AudioRequest represents a request structure for audio API.
// ResponseFormat is not supported for now. We only return JSON text, which may be sufficient.
type AudioRequest struct {
	Model       string
	FilePath    string  // Local file path - leave empty if using FileBytes + FileName
	FileBytes   *[]byte // File as bytes, also requires FileName to be set (see below)
	FileName    *string // File name for usage together with FileBytes. The API requires this parameter and use them as file format, so at least correct extension is required.
	Prompt      string  // For translation, it should be 'English'
	Temperature float32
	Language    string // For better and faster recognition, but optional.
	Format      AudioResponseFormat
}

// AudioResponse represents a response structure for audio API.
type AudioResponse struct {
	Text string `json:"text"`
}

// CreateTranscription — API call to create a transcription. Returns transcribed text.
func (c *Client) CreateTranscription(
	ctx context.Context,
	request AudioRequest,
) (response AudioResponse, err error) {
	return c.callAudioAPI(ctx, request, "transcriptions")
}

// CreateTranslation — API call to translate audio into English.
func (c *Client) CreateTranslation(
	ctx context.Context,
	request AudioRequest,
) (response AudioResponse, err error) {
	return c.callAudioAPI(ctx, request, "translations")
}

// callAudioAPI — API call to an audio endpoint.
func (c *Client) callAudioAPI(
	ctx context.Context,
	request AudioRequest,
	endpointSuffix string,
) (response AudioResponse, err error) {
	var formBody bytes.Buffer
	builder := c.createFormBuilder(&formBody)

	if err = audioMultipartForm(request, builder); err != nil {
		return AudioResponse{}, err
	}

	urlSuffix := fmt.Sprintf("/audio/%s", endpointSuffix)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.fullURL(urlSuffix), &formBody)
	if err != nil {
		return AudioResponse{}, err
	}
	req.Header.Add("Content-Type", builder.formDataContentType())

	if request.HasJSONResponse() {
		err = c.sendRequest(req, &response)
	} else {
		err = c.sendRequest(req, &response.Text)
	}
	if err != nil {
		return AudioResponse{}, err
	}
	return
}

// HasJSONResponse returns true if the response format is JSON.
func (r AudioRequest) HasJSONResponse() bool {
	return r.Format == "" || r.Format == AudioResponseFormatJSON
}

// audioMultipartForm creates a form with audio file contents and the name of the model to use for
// audio processing.
func audioMultipartForm(request AudioRequest, b formBuilder) error {

	// Create from filesystem path
	if request.FilePath != "" {
		f, err := os.Open(request.FilePath)
		if err != nil {
			return fmt.Errorf("opening audio file: %w", err)
		}
		defer f.Close()

		err = b.createFormFile("file", f)
		if err != nil {
			return fmt.Errorf("creating form file: %w", err)
		}

		// Create from provided bytes
	} else if request.FileBytes != nil {

		if request.FileName == nil || strings.Contains(*request.FileName, ".") == false {
			return errors.New("FileName with correct extension is required while FileBytes is used")
		} else {

			err := b.createFormFileFromBytes("file", *request.FileName, *request.FileBytes)
			if err != nil {
				return fmt.Errorf("creating form bytes: %w", err)
			}
		}

	} else {
		return errors.New("either FilePath or FileBytes should be specified")
	}

	err := b.writeField("model", request.Model)
	if err != nil {
		return fmt.Errorf("writing model name: %w", err)
	}

	// Create a form field for the prompt (if provided)
	if request.Prompt != "" {
		err = b.writeField("prompt", request.Prompt)
		if err != nil {
			return fmt.Errorf("writing prompt: %w", err)
		}
	}

	// Create a form field for the format (if provided)
	if request.Format != "" {
		err = b.writeField("response_format", string(request.Format))
		if err != nil {
			return fmt.Errorf("writing format: %w", err)
		}
	}

	// Create a form field for the temperature (if provided)
	if request.Temperature != 0 {
		err = b.writeField("temperature", fmt.Sprintf("%.2f", request.Temperature))
		if err != nil {
			return fmt.Errorf("writing temperature: %w", err)
		}
	}

	// Create a form field for the language (if provided)
	if request.Language != "" {
		err = b.writeField("language", request.Language)
		if err != nil {
			return fmt.Errorf("writing language: %w", err)
		}
	}

	// Close the multipart writer
	return b.close()
}
