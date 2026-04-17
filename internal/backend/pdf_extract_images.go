package backend

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"zotero_cli/internal/domain"
)

type ExtractedImage struct {
	AttachmentKey string `json:"attachment_key"`
	Page          int    `json:"page"`
	ObjectID      string `json:"object_id,omitempty"`
	Format        string `json:"format"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	Bytes         int64  `json:"bytes"`
	Path          string `json:"path"`
}

func (r *LocalReader) ExtractAttachmentImages(ctx context.Context, attachment domain.Attachment, outputDir string) ([]ExtractedImage, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	sourcePath := strings.TrimSpace(attachment.ResolvedPath)
	if sourcePath == "" || !attachment.Resolved {
		return nil, fmt.Errorf("attachment %s is not resolved to a local file", attachment.Key)
	}
	if strings.TrimSpace(outputDir) == "" {
		outputDir = "."
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, err
	}

	f, err := os.Open(sourcePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rawImages, err := api.ExtractImagesRaw(f, nil, nil)
	if err != nil {
		return nil, err
	}

	fileBase := sanitizeImageBaseName(firstNonEmptyString(attachment.Filename, filepath.Base(sourcePath), attachment.Key))
	results := make([]ExtractedImage, 0)
	for _, pageImages := range rawImages {
		objNrs := make([]int, 0, len(pageImages))
		for objNr := range pageImages {
			objNrs = append(objNrs, objNr)
		}
		sort.Ints(objNrs)
		for _, objNr := range objNrs {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			img := pageImages[objNr]
			result, err := writeExtractedImage(outputDir, fileBase, attachment.Key, img)
			if err != nil {
				return nil, err
			}
			results = append(results, result)
		}
	}

	return results, nil
}

func writeExtractedImage(outputDir string, fileBase string, attachmentKey string, img model.Image) (ExtractedImage, error) {
	format := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(img.FileType), "."))
	if format == "" {
		format = "bin"
	}

	objectID := strings.TrimSpace(img.Name)
	if objectID == "" {
		objectID = fmt.Sprintf("obj%d", img.ObjNr)
	}
	fileName := fmt.Sprintf("%s_page_%d_%s.%s", fileBase, img.PageNr, sanitizeImageBaseName(objectID), format)
	outPath := filepath.Join(outputDir, fileName)

	content, err := io.ReadAll(img)
	if err != nil {
		return ExtractedImage{}, err
	}

	outFile, err := os.Create(outPath)
	if err != nil {
		return ExtractedImage{}, err
	}
	written, copyErr := outFile.Write(content)
	closeErr := outFile.Close()
	if copyErr != nil {
		return ExtractedImage{}, copyErr
	}
	if closeErr != nil {
		return ExtractedImage{}, closeErr
	}

	width := img.Width
	height := img.Height
	if (width <= 0 || height <= 0) && len(content) > 0 {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(content))
		if err == nil {
			width = cfg.Width
			height = cfg.Height
		}
	}

	return ExtractedImage{
		AttachmentKey: attachmentKey,
		Page:          img.PageNr,
		ObjectID:      objectID,
		Format:        format,
		Width:         width,
		Height:        height,
		Bytes:         int64(written),
		Path:          outPath,
	}, nil
}

func sanitizeImageBaseName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "image"
	}
	replacer := strings.NewReplacer(
		"\\", "_",
		"/", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		" ", "_",
	)
	value = replacer.Replace(value)
	value = strings.Trim(value, "._")
	if value == "" {
		return "image"
	}
	return value
}
