package backend

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"zotero_cli/internal/domain"
)

type AttachmentHealth struct {
	OK     bool                    `json:"ok"`
	Status string                  `json:"status"`
	Issues []AttachmentHealthIssue `json:"issues,omitempty"`
}

type AttachmentHealthIssue struct {
	Code          string `json:"code"`
	Severity      string `json:"severity"`
	Message       string `json:"message"`
	SuggestedName string `json:"suggested_name,omitempty"`
}

var (
	attachmentInvalidFilenameChars = regexp.MustCompile(`[\\/:*?"<>|]`)
	attachmentRepeatedSpaces       = regexp.MustCompile(`\s{2,}`)
	attachmentPureNumberName       = regexp.MustCompile(`^\d+$`)
)

func InspectAttachmentHealth(attachment domain.Attachment) AttachmentHealth {
	issues := make([]AttachmentHealthIssue, 0, 4)
	filename := attachmentHealthFilename(attachment)
	filenameLower := strings.ToLower(filename)

	if strings.TrimSpace(attachment.ResolvedPath) == "" || !attachment.Resolved {
		issues = append(issues, AttachmentHealthIssue{
			Code:     "unresolved_path",
			Severity: "error",
			Message:  "attachment path is not resolved locally",
		})
	} else if info, err := os.Stat(attachment.ResolvedPath); err != nil {
		if os.IsNotExist(err) {
			issues = append(issues, AttachmentHealthIssue{
				Code:     "missing_file",
				Severity: "error",
				Message:  "attachment file does not exist",
			})
		} else {
			issues = append(issues, AttachmentHealthIssue{
				Code:     "stat_failed",
				Severity: "error",
				Message:  fmt.Sprintf("attachment file cannot be inspected: %v", err),
			})
		}
	} else if info.IsDir() {
		issues = append(issues, AttachmentHealthIssue{
			Code:     "path_is_directory",
			Severity: "error",
			Message:  "attachment path points to a directory",
		})
	}

	if filename == "" {
		issues = append(issues, AttachmentHealthIssue{
			Code:     "missing_filename",
			Severity: "warning",
			Message:  "attachment has no usable filename",
		})
	} else {
		if len(filename) > 200 {
			issues = append(issues, AttachmentHealthIssue{
				Code:     "filename_too_long",
				Severity: "critical",
				Message:  "attachment filename is longer than 200 characters",
			})
		}
		if attachmentInvalidFilenameChars.MatchString(filename) {
			issues = append(issues, AttachmentHealthIssue{
				Code:          "filename_invalid_chars",
				Severity:      "critical",
				Message:       "attachment filename contains characters that are unsafe on common filesystems",
				SuggestedName: sanitizeAttachmentFilename(filename),
			})
		}
		if strings.TrimSpace(filename) != filename || attachmentRepeatedSpaces.MatchString(filename) {
			issues = append(issues, AttachmentHealthIssue{
				Code:          "filename_spacing",
				Severity:      "warning",
				Message:       "attachment filename has leading/trailing whitespace or repeated spaces",
				SuggestedName: sanitizeAttachmentFilename(filename),
			})
		}
		if isAttachmentPDF(attachment) && filepath.Ext(filenameLower) != ".pdf" {
			issues = append(issues, AttachmentHealthIssue{
				Code:          "missing_pdf_extension",
				Severity:      "warning",
				Message:       "PDF attachment filename does not end with .pdf",
				SuggestedName: strings.TrimSuffix(sanitizeAttachmentFilename(filename), filepath.Ext(filename)) + ".pdf",
			})
		}
		if isGenericAttachmentFilename(filenameLower) {
			issues = append(issues, AttachmentHealthIssue{
				Code:     "filename_generic",
				Severity: "info",
				Message:  "attachment filename looks generic",
			})
		}
	}

	return AttachmentHealth{
		OK:     len(issues) == 0,
		Status: attachmentHealthStatus(issues),
		Issues: issues,
	}
}

func AttachmentHealthMatches(attachment domain.Attachment, minSeverity string) bool {
	minSeverity = strings.TrimSpace(strings.ToLower(minSeverity))
	if minSeverity == "" {
		return false
	}
	health := InspectAttachmentHealth(attachment)
	for _, issue := range health.Issues {
		if attachmentSeverityRank(issue.Severity) >= attachmentSeverityRank(minSeverity) {
			return true
		}
	}
	return false
}

func AttachmentHasMissingFile(attachment domain.Attachment) bool {
	health := InspectAttachmentHealth(attachment)
	for _, issue := range health.Issues {
		if issue.Code == "missing_file" || issue.Code == "unresolved_path" || issue.Code == "path_is_directory" {
			return true
		}
	}
	return false
}

func AttachmentHasBadName(attachment domain.Attachment) bool {
	health := InspectAttachmentHealth(attachment)
	for _, issue := range health.Issues {
		switch issue.Code {
		case "filename_too_long", "filename_invalid_chars", "filename_spacing", "missing_pdf_extension", "filename_generic", "missing_filename":
			return true
		}
	}
	return false
}

func attachmentHealthFilename(attachment domain.Attachment) string {
	if strings.TrimSpace(attachment.Filename) != "" {
		return filepath.Base(filepath.FromSlash(strings.TrimSpace(attachment.Filename)))
	}
	if strings.TrimSpace(attachment.ResolvedPath) != "" {
		return filepath.Base(attachment.ResolvedPath)
	}
	if strings.TrimSpace(attachment.ZoteroPath) != "" {
		path := strings.TrimSpace(attachment.ZoteroPath)
		if after, ok := strings.CutPrefix(path, "storage:"); ok {
			path = after
		} else if after, ok := strings.CutPrefix(path, "attachments:"); ok {
			path = after
		}
		return filepath.Base(filepath.FromSlash(path))
	}
	return strings.TrimSpace(attachment.Title)
}

func attachmentHealthStatus(issues []AttachmentHealthIssue) string {
	status := "ok"
	highest := 0
	for _, issue := range issues {
		if rank := attachmentSeverityRank(issue.Severity); rank > highest {
			highest = rank
			status = strings.ToLower(issue.Severity)
		}
	}
	return status
}

func attachmentSeverityRank(severity string) int {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical":
		return 4
	case "error":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

func isAttachmentPDF(attachment domain.Attachment) bool {
	return strings.EqualFold(attachment.ContentType, "application/pdf") ||
		strings.EqualFold(filepath.Ext(attachmentHealthFilename(attachment)), ".pdf")
}

func isGenericAttachmentFilename(filenameLower string) bool {
	name := strings.TrimSuffix(filepath.Base(filenameLower), filepath.Ext(filenameLower))
	name = strings.TrimSpace(name)
	return strings.HasPrefix(name, "download") ||
		strings.HasPrefix(name, "copy of") ||
		attachmentPureNumberName.MatchString(name)
}

func sanitizeAttachmentFilename(filename string) string {
	cleaned := attachmentInvalidFilenameChars.ReplaceAllString(strings.TrimSpace(filename), "_")
	cleaned = attachmentRepeatedSpaces.ReplaceAllString(cleaned, " ")
	if cleaned == "" {
		return "attachment"
	}
	return cleaned
}
